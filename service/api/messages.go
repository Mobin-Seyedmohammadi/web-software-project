package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/yourname/wasatext/service/db"
)

// ForwardMessageRequest represents the request to forward a message.
type ForwardMessageRequest struct {
	TargetConversationID string `json:"targetConversationId"`
}

// AddReactionRequest represents the request to add a reaction.
type AddReactionRequest struct {
	Emoticon string `json:"emoticon"`
}

// requireConversationMember checks that a conversation exists and that
// userID is a participant of it, writing the appropriate error response
// (404 or 403/500) if not. Returns false if it already wrote a response.
func (h *Server) requireConversationMember(
	w http.ResponseWriter, r *http.Request, conversationID, userID, notFoundMsg, actionDesc string,
) bool {
	// Existence is checked before membership so a bad/unknown conversationId
	// reports 404 rather than being indistinguishable from "not a member".
	exists, err := h.database.ConversationExists(r.Context(), conversationID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to check conversation existence")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to "+actionDesc)
		return false
	}
	if !exists {
		h.errorResponse(w, http.StatusNotFound, notFoundMsg)
		return false
	}

	return h.requireMembership(w, r, conversationID, userID, "Not a member of this conversation")
}

// requireMembership checks that userID is a participant of conversationID,
// writing a 403 response if not (or on error, since membership-check
// failures shouldn't leak whether the conversation exists). Returns false
// if it already wrote a response.
func (h *Server) requireMembership(
	w http.ResponseWriter, r *http.Request, conversationID, userID, forbiddenMsg string,
) bool {
	isMember, err := h.database.CheckConversationMembership(r.Context(), userID, conversationID)
	if err != nil || !isMember {
		h.errorResponse(w, http.StatusForbidden, forbiddenMsg)
		return false
	}
	return true
}

// extractPhotoFromForm reads an optional "photo" multipart field and saves
// it, returning (nil, true) if the field is absent. Returns ok=false if it
// already wrote an error response.
func (h *Server) extractPhotoFromForm(w http.ResponseWriter, r *http.Request) (*string, bool) {
	file, header, err := r.FormFile("photo")
	if err != nil {
		return nil, true
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			h.logger.WithError(closeErr).Error("Failed to close uploaded file")
		}
	}()

	uploadedURL, err := h.saveUploadedFile(file, header.Filename)
	if err != nil {
		h.logger.WithError(err).Error("Failed to save photo")
		h.errorResponse(w, http.StatusBadRequest, "Failed to save photo")
		return nil, false
	}
	return &uploadedURL, true
}

// deleteResource runs deleteFn and maps its ErrUnauthorized/notFoundErr
// sentinel errors to 403/404, logging anything else as an internal error.
// On success it writes 204 No Content and returns true.
func (h *Server) deleteResource(
	w http.ResponseWriter, deleteFn func() error, notFoundErr error, unauthorizedMsg, notFoundMsg, actionDesc string,
) bool {
	if err := deleteFn(); err != nil {
		switch {
		case errors.Is(err, db.ErrUnauthorized):
			h.errorResponse(w, http.StatusForbidden, unauthorizedMsg)
		case errors.Is(err, notFoundErr):
			h.errorResponse(w, http.StatusNotFound, notFoundMsg)
		default:
			h.logger.WithError(err).Error("Failed to " + actionDesc)
			h.errorResponse(w, http.StatusInternalServerError, "Failed to "+actionDesc)
		}
		return false
	}
	w.WriteHeader(http.StatusNoContent)
	return true
}

// handleSendMessage handles POST /conversations/:conversationId/messages.
func (h *Server) handleSendMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	ok := h.requireConversationMember(
		w, r, conversationID, currentUser.Identifier, "Conversation not found", "send message",
	)
	if !ok {
		return
	}

	if !h.requireMultipartForm(w, r, maxPhotoUploadBytes) {
		return
	}

	textContent := r.FormValue("content")
	replyTo := r.FormValue("replyTo")

	photoURL, ok := h.extractPhotoFromForm(w, r)
	if !ok {
		return
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

	createdMsg, err := h.database.PostMessage(r.Context(), msg)
	if err != nil {
		h.logger.WithError(err).Error("Failed to send message")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to send message")
		return
	}

	h.logger.WithField("messageID", createdMsg.MessageID).Info("Message sent")

	h.jsonResponse(w, http.StatusCreated, createdMsg)
}

// handleDeleteMessage handles DELETE /messages/:messageId.
func (h *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	ctx := r.Context()
	ok := h.deleteResource(w, func() error {
		return h.database.RemoveMessage(ctx, messageID, currentUser.Identifier)
	}, db.ErrMessageNotFound, "Not authorized to delete this message", "Message not found", "delete message")
	if !ok {
		return
	}

	h.logger.WithField("messageID", messageID).Info("Message deleted")
}

// handleForwardMessage handles POST /messages/:messageId/forward.
func (h *Server) handleForwardMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	if !h.requireConversationMember(w, r, req.TargetConversationID, currentUser.Identifier,
		"Target conversation not found", "forward message") {
		return
	}

	// Check if original message exists and user has access.
	originalMsg, err := h.database.FetchMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, db.ErrMessageNotFound) {
			h.errorResponse(w, http.StatusNotFound, "Message not found")
		} else {
			h.logger.WithError(err).Error("Failed to fetch message")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to forward message")
		}
		return
	}

	forbiddenMsg := "Not authorized to forward this message"
	if !h.requireMembership(w, r, originalMsg.ConversationID, currentUser.Identifier, forbiddenMsg) {
		return
	}

	forwardedMsg, err := h.database.DuplicateMessage(
		r.Context(), messageID, req.TargetConversationID, currentUser.Identifier,
	)
	if err != nil {
		h.logger.WithError(err).Error("Failed to forward message")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to forward message")
		return
	}

	h.logger.WithField("messageID", forwardedMsg.MessageID).Info("Message forwarded")

	h.jsonResponse(w, http.StatusCreated, forwardedMsg)
}

// handleAddReaction handles POST /messages/:messageId/comments.
func (h *Server) handleAddReaction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	// Check if message exists and user has access.
	msg, err := h.database.FetchMessage(r.Context(), messageID)
	if err != nil {
		if errors.Is(err, db.ErrMessageNotFound) {
			h.errorResponse(w, http.StatusNotFound, "Message not found")
		} else {
			h.logger.WithError(err).Error("Failed to fetch message")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to add reaction")
		}
		return
	}

	if !h.requireMembership(w, r, msg.ConversationID, currentUser.Identifier, "Not authorized to react to this message") {
		return
	}

	reaction, err := h.database.AddReaction(r.Context(), messageID, currentUser.Identifier, req.Emoticon)
	if err != nil {
		h.logger.WithError(err).Error("Failed to add reaction")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to add reaction")
		return
	}

	h.logger.WithField("reactionID", reaction.ReactionID).Info("Reaction added")

	h.jsonResponse(w, http.StatusCreated, reaction)
}

// handleRemoveReaction handles DELETE /messages/:messageId/comments/:commentId.
func (h *Server) handleRemoveReaction(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	ctx := r.Context()
	ok := h.deleteResource(w, func() error {
		return h.database.RemoveReaction(ctx, commentID, currentUser.Identifier)
	}, db.ErrCommentNotFound, "Not authorized to remove this reaction", "Reaction not found", "remove reaction")
	if !ok {
		return
	}

	h.logger.WithField("reactionID", commentID).Info("Reaction removed")
}
