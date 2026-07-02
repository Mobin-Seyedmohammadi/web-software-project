package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/yourname/wasatext/service/db"
)

// UsernameUpdateRequest represents the request to update username.
type UsernameUpdateRequest struct {
	Username string `json:"username"`
}

const (
	minUsernameLength = 3
	maxUsernameLength = 16
)

// handleSearchUsers handles GET /users.
func (h *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	query := r.URL.Query().Get("query")

	// Exclude the requester so you can never pick yourself to start a chat,
	// add as a group member, etc.
	users, err := h.database.FindUsers(r.Context(), query, currentUser.Identifier)
	if err != nil {
		h.logger.WithError(err).Error("Failed to search users")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to search users")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"users": users,
	})
}

// handleUpdateUsername handles PUT /users/me/username.
func (h *Server) handleUpdateUsername(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	if len(req.Username) < minUsernameLength || len(req.Username) > maxUsernameLength {
		h.errorResponse(w, http.StatusBadRequest, "Username must be between 3 and 16 characters")
		return
	}

	updatedUser, err := h.database.ChangeUsername(r.Context(), currentUser.Identifier, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrUsernameExists):
			h.errorResponse(w, http.StatusConflict, "Username already taken")
		default:
			h.logger.WithError(err).Error("Failed to update username")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to update username")
		}
		return
	}

	h.logger.WithField("userID", currentUser.Identifier).WithField("newUsername", req.Username).Info("Username updated")

	h.jsonResponse(w, http.StatusOK, updatedUser)
}

// handleUploadUserPhoto handles PUT /users/me/photo.
func (h *Server) handleUploadUserPhoto(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	if !h.requireMultipartForm(w, r, maxPhotoUploadBytes) {
		return
	}

	photoURL, ok := h.receivePhoto(w, r)
	if !ok {
		return
	}

	updatedUser, err := h.database.SetUserPhoto(r.Context(), currentUser.Identifier, photoURL)
	if err != nil {
		h.logger.WithError(err).Error("Failed to update user photo")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to update photo")
		return
	}

	h.logger.WithField("userID", currentUser.Identifier).WithField("photoURL", photoURL).Info("User photo updated")

	h.jsonResponse(w, http.StatusOK, updatedUser)
}
