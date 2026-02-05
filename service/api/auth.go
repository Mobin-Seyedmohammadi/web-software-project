package api

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// LoginRequest represents the login request body
type LoginRequest struct {
	Name string `json:"name"`
}

// LoginResponse represents the login response body
type LoginResponse struct {
	Identifier string `json:"identifier"`
}

// handleLogin handles the POST /session endpoint
func (h *APIHandler) handleLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if len(req.Name) < 3 || len(req.Name) > 16 {
		h.errorResponse(w, http.StatusBadRequest, "Username must be between 3 and 16 characters")
		return
	}

	user, err := h.database.LoginOrRegisterUser(req.Name)
	if err != nil {
		h.logger.WithError(err).Error("Login failed")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to log in")
		return
	}

	h.logger.WithField("userID", user.Identifier).WithField("username", user.Username).Info("User logged in")

	h.jsonResponse(w, http.StatusCreated, LoginResponse{
		Identifier: user.Identifier,
	})
}
