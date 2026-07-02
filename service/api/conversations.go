package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/yourname/wasatext/service/db"
)

// CreateConversationRequest represents the request to create a conversation
type CreateConversationRequest struct {
	UserID string `json:"userId"`
}

// handleGetConversations handles GET /conversations
func (h *APIHandler) handleGetConversations(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	conversations, err := h.database.FetchUserConversations(currentUser.Identifier)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch conversations")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch conversations")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"conversations": conversations,
	})
}

// handleCreateConversation handles POST /conversations
func (h *APIHandler) handleCreateConversation(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.UserID == "" {
		h.errorResponse(w, http.StatusBadRequest, "User ID required")
		return
	}

	if req.UserID == currentUser.Identifier {
		h.errorResponse(w, http.StatusBadRequest, "Cannot start a conversation with yourself")
		return
	}

	// Check if target user exists
	_, err := h.database.FindUserByID(req.UserID)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			h.errorResponse(w, http.StatusNotFound, "User not found")
		} else {
			h.logger.WithError(err).Error("Failed to find user")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to create conversation")
		}
		return
	}

	conversation, err := h.database.InitiatePrivateConversation(currentUser.Identifier, req.UserID)
	if err != nil {
		if errors.Is(err, db.ErrCannotMessageSelf) {
			h.errorResponse(w, http.StatusBadRequest, "Cannot start a conversation with yourself")
		} else {
			h.logger.WithError(err).Error("Failed to create conversation")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to create conversation")
		}
		return
	}

	h.logger.WithField("conversationID", conversation.ConversationID).Info("Conversation created")

	h.jsonResponse(w, http.StatusCreated, conversation)
}

// handleGetConversation handles GET /conversations/:conversationId
func (h *APIHandler) handleGetConversation(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	conversationID := ps.ByName("conversationId")
	if conversationID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Conversation ID required")
		return
	}

	conversation, err := h.database.FetchConversationDetails(conversationID, currentUser.Identifier)
	if err != nil {
		if errors.Is(err, db.ErrConversationNotFound) {
			h.errorResponse(w, http.StatusNotFound, "Conversation not found")
		} else if errors.Is(err, db.ErrUserNotInConversation) {
			h.errorResponse(w, http.StatusForbidden, "Not a member of this conversation")
		} else {
			h.logger.WithError(err).Error("Failed to fetch conversation")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch conversation")
		}
		return
	}

	h.jsonResponse(w, http.StatusOK, conversation)
}
