package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/yourname/wasatext/service/db"
)

// defaultPhotoExt is used both as the extension for uploads with no (or an
// unrecognized) extension and as a key in validPhotoExts below.
const defaultPhotoExt = ".jpg"

// maxPhotoUploadBytes is the maximum accepted size, in bytes, for a
// multipart photo upload (user avatar, group avatar, or message photo).
const maxPhotoUploadBytes = 10 << 20 // 10 MB

// photoDirPerm restricts the photos directory to owner+group access.
const photoDirPerm = 0o750

var validPhotoExts = map[string]bool{
	defaultPhotoExt: true,
	".jpeg":         true,
	".png":          true,
	".gif":          true,
	".webp":         true,
}

func (h *Server) errorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]string{"error": message}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode error response")
	}
}

func (h *Server) jsonResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.logger.WithError(err).Error("Failed to encode JSON response")
		}
	}
}

// postSystemMessage records a group-event announcement (join/leave/rename/
// photo change) as a chat message so it shows up in the conversation
// timeline. Best-effort: a failure here logs but doesn't fail the caller's
// primary operation, which has already succeeded.
func (h *Server) postSystemMessage(ctx context.Context, conversationID, actorID, text string) {
	_, err := h.database.PostMessage(ctx, &db.Message{
		ConversationID: conversationID,
		SenderID:       actorID,
		TextContent:    &text,
		MessageType:    db.MessageTypeSystem,
	})
	if err != nil {
		h.logger.WithError(err).Error("Failed to post system message")
	}
}

// requireMultipartForm rejects requests that aren't multipart/form-data with
// 415 (wrong Content-Type entirely) before attempting to parse the body, so
// that case is distinguishable from a 400 (right Content-Type, malformed
// body). Returns false if it already wrote an error response.
func (h *Server) requireMultipartForm(w http.ResponseWriter, r *http.Request, maxMemory int64) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		h.errorResponse(w, http.StatusUnsupportedMediaType, "Content-Type must be multipart/form-data")
		return false
	}
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Failed to parse form data")
		return false
	}
	return true
}

// receivePhoto reads a required "photo" multipart field and saves it,
// writing an error response and returning ok=false if the field is missing
// or the upload can't be saved. Caller must have already validated the
// request is a multipart form (see requireMultipartForm).
func (h *Server) receivePhoto(w http.ResponseWriter, r *http.Request) (string, bool) {
	file, header, err := r.FormFile("photo")
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Photo file required")
		return "", false
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			h.logger.WithError(closeErr).Error("Failed to close uploaded file")
		}
	}()

	photoURL, err := h.saveUploadedFile(file, header.Filename)
	if err != nil {
		h.logger.WithError(err).Error("Failed to save photo")
		h.errorResponse(w, http.StatusBadRequest, "Failed to save photo")
		return "", false
	}
	return photoURL, true
}

func photosDir() string {
	if dir := os.Getenv("PHOTOS_DIR"); dir != "" {
		return dir
	}
	return "./photos"
}

// EnsurePhotosDir creates the photos directory (from PHOTOS_DIR, or
// ./photos by default) if needed and confirms this process can actually
// write to it, via a real write-then-remove probe rather than trusting
// permission bits — a freshly-mounted Docker volume can look fine and
// still reject writes. Called once at startup so a misconfigured/
// unwritable volume fails fast with a clear, specific error naming the
// exact path, instead of surfacing as a cryptic failure on a user's first
// photo upload.
func EnsurePhotosDir() error {
	dir := photosDir()
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to resolve photos directory %q: %w", dir, err)
	}

	if err := os.MkdirAll(absDir, photoDirPerm); err != nil {
		return fmt.Errorf("failed to create photos directory %q: %w", absDir, err)
	}

	probePath := filepath.Join(absDir, ".write-test")
	//nolint:gosec // probePath is absDir + a fixed literal suffix, not user input
	probe, err := os.Create(probePath)
	if err != nil {
		return fmt.Errorf("photos directory %q exists but is not writable by this process: %w", absDir, err)
	}
	if closeErr := probe.Close(); closeErr != nil {
		return fmt.Errorf("failed to close write-test probe in %q: %w", absDir, closeErr)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("failed to remove write-test probe from %q: %w", absDir, err)
	}
	return nil
}

// saveUploadedFile writes an uploaded photo under the photos directory with
// a generated filename and returns its public URL path. filename is only
// used to infer an extension from an allowlist; it never contributes to the
// path directly, so it can't be used for directory traversal or to
// overwrite arbitrary files.
func (h *Server) saveUploadedFile(file io.Reader, filename string) (string, error) {
	dir := photosDir()
	if err := os.MkdirAll(dir, photoDirPerm); err != nil {
		return "", fmt.Errorf("failed to create photo directory: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !validPhotoExts[ext] {
		ext = defaultPhotoExt
	}

	id, _ := uuid.NewV4()
	newFilename := id.String() + ext
	filePath := filepath.Join(dir, newFilename)

	//nolint:gosec // filePath is dir + generated UUID + allowlisted extension, not user input
	outFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			h.logger.WithError(closeErr).Error("Failed to close uploaded file")
		}
	}()

	if _, err := io.Copy(outFile, file); err != nil {
		if removeErr := os.Remove(filePath); removeErr != nil {
			h.logger.WithError(removeErr).Error("Failed to remove partially written file")
		}
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "/photos/" + newFilename, nil
}
