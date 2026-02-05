package api

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/yourname/wasatext/service/db"
)

// UsernameUpdateRequest represents the request to update username
type UsernameUpdateRequest struct {
	Username string `json:"username"`
}

// handleSearchUsers handles GET /users
func (h *APIHandler) handleSearchUsers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	query := r.URL.Query().Get("query")

	users, err := h.database.FindUsers(query)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search users")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to search users")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"users": users,
	})
}

// handleUpdateUsername handles PUT /users/me/username
func (h *APIHandler) handleUpdateUsername(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req UsernameUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Username) < 3 || len(req.Username) > 16 {
		h.errorResponse(w, http.StatusBadRequest, "Username must be between 3 and 16 characters")
		return
	}

	updatedUser, err := h.database.ChangeUsername(currentUser.Identifier, req.Username)
	if err != nil {
		if err == db.ErrUsernameExists {
			h.errorResponse(w, http.StatusConflict, "Username already taken")
		} else {
			h.logger.WithError(err).Error("Failed to update username")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to update username")
		}
		return
	}

	h.logger.WithField("userID", currentUser.Identifier).WithField("newUsername", req.Username).Info("Username updated")

	h.jsonResponse(w, http.StatusOK, updatedUser)
}

// handleUploadUserPhoto handles PUT /users/me/photo
func (h *APIHandler) handleUploadUserPhoto(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB max
		h.errorResponse(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Photo file required")
		return
	}
	defer file.Close()

	photoURL, err := h.saveUploadedFile(file, header.Filename)
	if err != nil {
		h.logger.WithError(err).Error("Failed to save photo")
		h.errorResponse(w, http.StatusBadRequest, "Failed to save photo")
		return
	}

	updatedUser, err := h.database.SetUserPhoto(currentUser.Identifier, photoURL)
	if err != nil {
		h.logger.WithError(err).Error("Failed to update user photo")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to update photo")
		return
	}

	h.logger.WithField("userID", currentUser.Identifier).WithField("photoURL", photoURL).Info("User photo updated")

	h.jsonResponse(w, http.StatusOK, updatedUser)
}
