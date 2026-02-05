package api

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/yourname/wasatext/service/db"
)

// ForwardMessageRequest represents the request to forward a message
type ForwardMessageRequest struct {
	TargetConversationID string `json:"targetConversationId"`
}

// handleSendMessage handles POST /conversations/:conversationId/messages
func (h *APIHandler) handleSendMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	// Check if user is in conversation
	isMember, err := h.database.CheckConversationMembership(currentUser.Identifier, conversationID)
	if err != nil || !isMember {
		h.errorResponse(w, http.StatusForbidden, "Not a member of this conversation")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB max
		h.errorResponse(w, http.StatusBadRequest, "Failed to parse form data")
		return
	}

	textContent := r.FormValue("content")
	replyTo := r.FormValue("replyTo")

	var photoURL *string
	file, header, err := r.FormFile("photo")
	if err == nil {
		defer file.Close()
		uploadedURL, err := h.saveUploadedFile(file, header.Filename)
		if err != nil {
			h.logger.WithError(err).Error("Failed to save photo")
			h.errorResponse(w, http.StatusBadRequest, "Failed to save photo")
			return
		}
		photoURL = &uploadedURL
	}

	if textContent == "" && photoURL == nil {
		h.errorResponse(w, http.StatusBadRequest, "Message content or photo required")
		return
	}

	msg := &db.Message{
		ConversationID: conversationID,
		SenderID:       currentUser.Identifier,
	}

	if textContent != "" {
		msg.TextContent = &textContent
	}
	if photoURL != nil {
		msg.PhotoURL = photoURL
	}
	if replyTo != "" {
		msg.ReplyToID = &replyTo
	}

	createdMsg, err := h.database.PostMessage(msg)
	if err != nil {
		h.logger.WithError(err).Error("Failed to send message")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to send message")
		return
	}

	h.logger.WithField("messageID", createdMsg.MessageID).Info("Message sent")

	h.jsonResponse(w, http.StatusCreated, createdMsg)
}

// handleDeleteMessage handles DELETE /messages/:messageId
func (h *APIHandler) handleDeleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	messageID := ps.ByName("messageId")
	if messageID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Message ID required")
		return
	}

	err := h.database.RemoveMessage(messageID, currentUser.Identifier)
	if err != nil {
		if err == db.ErrUnauthorized {
			h.errorResponse(w, http.StatusForbidden, "Not authorized to delete this message")
		} else if err == db.ErrMessageNotFound {
			h.errorResponse(w, http.StatusNotFound, "Message not found")
		} else {
			h.logger.WithError(err).Error("Failed to delete message")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to delete message")
		}
		return
	}

	h.logger.WithField("messageID", messageID).Info("Message deleted")

	w.WriteHeader(http.StatusNoContent)
}

// handleForwardMessage handles POST /messages/:messageId/forward
func (h *APIHandler) handleForwardMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	messageID := ps.ByName("messageId")
	if messageID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Message ID required")
		return
	}

	var req ForwardMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TargetConversationID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Target conversation ID required")
		return
	}

	// Check if user is in target conversation
	isMember, err := h.database.CheckConversationMembership(currentUser.Identifier, req.TargetConversationID)
	if err != nil || !isMember {
		h.errorResponse(w, http.StatusForbidden, "Not a member of target conversation")
		return
	}

	// Check if original message exists and user has access
	originalMsg, err := h.database.FetchMessage(messageID)
	if err != nil {
		if err == db.ErrMessageNotFound {
			h.errorResponse(w, http.StatusNotFound, "Message not found")
		} else {
			h.logger.WithError(err).Error("Failed to fetch message")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to forward message")
		}
		return
	}

	// Check if user is in the original conversation
	isMember, err = h.database.CheckConversationMembership(currentUser.Identifier, originalMsg.ConversationID)
	if err != nil || !isMember {
		h.errorResponse(w, http.StatusForbidden, "Not authorized to forward this message")
		return
	}

	forwardedMsg, err := h.database.DuplicateMessage(messageID, req.TargetConversationID, currentUser.Identifier)
	if err != nil {
		h.logger.WithError(err).Error("Failed to forward message")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to forward message")
		return
	}

	h.logger.WithField("messageID", forwardedMsg.MessageID).Info("Message forwarded")

	h.jsonResponse(w, http.StatusCreated, forwardedMsg)
}

// AddReactionRequest represents the request to add a reaction
type AddReactionRequest struct {
	Emoticon string `json:"emoticon"`
}

// handleAddReaction handles POST /messages/:messageId/comments
func (h *APIHandler) handleAddReaction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	messageID := ps.ByName("messageId")
	if messageID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Message ID required")
		return
	}

	var req AddReactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Emoticon == "" {
		h.errorResponse(w, http.StatusBadRequest, "Emoticon required")
		return
	}

	// Check if message exists and user has access
	msg, err := h.database.FetchMessage(messageID)
	if err != nil {
		if err == db.ErrMessageNotFound {
			h.errorResponse(w, http.StatusNotFound, "Message not found")
		} else {
			h.logger.WithError(err).Error("Failed to fetch message")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to add reaction")
		}
		return
	}

	isMember, err := h.database.CheckConversationMembership(currentUser.Identifier, msg.ConversationID)
	if err != nil || !isMember {
		h.errorResponse(w, http.StatusForbidden, "Not authorized to react to this message")
		return
	}

	reaction, err := h.database.AddReaction(messageID, currentUser.Identifier, req.Emoticon)
	if err != nil {
		h.logger.WithError(err).Error("Failed to add reaction")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to add reaction")
		return
	}

	h.logger.WithField("reactionID", reaction.ReactionID).Info("Reaction added")

	h.jsonResponse(w, http.StatusCreated, reaction)
}

// handleRemoveReaction handles DELETE /messages/:messageId/comments/:commentId
func (h *APIHandler) handleRemoveReaction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	commentID := ps.ByName("commentId")
	if commentID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Comment ID required")
		return
	}

	err := h.database.RemoveReaction(commentID, currentUser.Identifier)
	if err != nil {
		if err == db.ErrUnauthorized {
			h.errorResponse(w, http.StatusForbidden, "Not authorized to remove this reaction")
		} else if err == db.ErrCommentNotFound {
			h.errorResponse(w, http.StatusNotFound, "Reaction not found")
		} else {
			h.logger.WithError(err).Error("Failed to remove reaction")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to remove reaction")
		}
		return
	}

	h.logger.WithField("reactionID", commentID).Info("Reaction removed")

	w.WriteHeader(http.StatusNoContent)
}
