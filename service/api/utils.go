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
)

// errorResponse sends a JSON error response
func (h *APIHandler) errorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error": message,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.WithError(err).Error("Failed to encode error response")
	}
}

// jsonResponse sends a JSON success response
func (h *APIHandler) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.logger.WithError(err).Error("Failed to encode JSON response")
		}
	}
}

// saveUploadedFile saves an uploaded file and returns its URL
func (h *APIHandler) saveUploadedFile(file io.Reader, filename string) (string, error) {
	photoDir := "./photos"
	if err := os.MkdirAll(photoDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create photo directory: %w", err)
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}

	// Validate file extension
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
	}

	if !validExts[strings.ToLower(ext)] {
		return "", fmt.Errorf("invalid file type")
	}

	// Generate unique filename
	id, _ := uuid.NewV4()
	newFilename := id.String() + ext
	filePath := filepath.Join(photoDir, newFilename)

	// Create and write file
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
