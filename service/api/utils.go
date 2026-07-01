package api

import (
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

func (h *APIHandler) errorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]string{"error": message}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode error response")
	}
}

func (h *APIHandler) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
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
func (h *APIHandler) postSystemMessage(conversationID, actorID, text string) {
	_, err := h.database.PostMessage(&db.Message{
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
func (h *APIHandler) requireMultipartForm(w http.ResponseWriter, r *http.Request, maxMemory int64) bool {
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

func photosDir() string {
	if dir := os.Getenv("PHOTOS_DIR"); dir != "" {
		return dir
	}
	return "./photos"
}

func (h *APIHandler) saveUploadedFile(file io.Reader, filename string) (string, error) {
	dir := photosDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create photo directory: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".jpg"
	}

	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
	if !validExts[ext] {
		ext = ".jpg"
	}

	id, _ := uuid.NewV4()
	newFilename := id.String() + ext
	filePath := filepath.Join(dir, newFilename)

	outFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, file); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "/photos/" + newFilename, nil
}
