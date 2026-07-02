package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/yourname/wasatext/service/db"
)

// maxGroupNameLength is the longest group name accepted by
// handleCreateGroup and handleUpdateGroupName.
const maxGroupNameLength = 100

// CreateGroupRequest represents the request to create a group.
type CreateGroupRequest struct {
	Name      string   `json:"name"`
	MemberIDs []string `json:"memberIds"`
}

// AddGroupMemberRequest represents the request to add a member to a group.
type AddGroupMemberRequest struct {
	UserID string `json:"userId"`
}

// UpdateGroupNameRequest represents the request to update group name.
type UpdateGroupNameRequest struct {
	Name string `json:"name"`
}

// handleCreateGroup handles POST /groups.
func (h *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		h.errorResponse(w, http.StatusBadRequest, "Group name required")
		return
	}

	if len(req.Name) > maxGroupNameLength {
		h.errorResponse(w, http.StatusBadRequest, "Group name too long")
		return
	}

	if req.MemberIDs == nil {
		req.MemberIDs = []string{}
	}

	group, err := h.database.CreateNewGroup(r.Context(), req.Name, currentUser.Identifier, req.MemberIDs)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create group")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to create group")
		return
	}

	h.logger.WithField("groupID", group.GroupID).Info("Group created")

	h.jsonResponse(w, http.StatusCreated, group)
}

// decodeAddGroupMemberRequest reads and validates the body of a "add group
// member" request, writing an error response itself if invalid.
func (h *Server) decodeAddGroupMemberRequest(w http.ResponseWriter, r *http.Request) (AddGroupMemberRequest, bool) {
	var req AddGroupMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return req, false
	}
	if req.UserID == "" {
		h.errorResponse(w, http.StatusBadRequest, "User ID required")
		return req, false
	}
	return req, true
}

// handleAddGroupMember handles POST /groups/:groupId/members.
func (h *Server) handleAddGroupMember(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	groupID := ps.ByName("groupId")
	if groupID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Group ID required")
		return
	}

	req, ok := h.decodeAddGroupMemberRequest(w, r)
	if !ok {
		return
	}

	// Check if user exists.
	addedUser, err := h.database.FindUserByID(r.Context(), req.UserID)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			h.errorResponse(w, http.StatusNotFound, "User not found")
		} else {
			h.logger.WithError(err).Error("Failed to find user")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to add member")
		}
		return
	}

	err = h.database.AddGroupMember(r.Context(), groupID, req.UserID, currentUser.Identifier)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrUnauthorized):
			h.errorResponse(w, http.StatusForbidden, "Not authorized to add members to this group")
		case errors.Is(err, db.ErrGroupNotFound):
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		case errors.Is(err, db.ErrUserAlreadyInGroup):
			h.errorResponse(w, http.StatusConflict, "User is already in this group")
		default:
			h.logger.WithError(err).Error("Failed to add group member")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to add group member")
		}
		return
	}

	h.postSystemMessage(r.Context(), groupID, currentUser.Identifier,
		currentUser.Username+" added "+addedUser.Username)

	group, err := h.database.FetchGroupInfo(r.Context(), groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch group info")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch group info")
		return
	}

	h.logger.WithField("groupID", groupID).WithField("userID", req.UserID).Info("User added to group")

	h.jsonResponse(w, http.StatusCreated, group)
}

// handleLeaveGroup handles DELETE /groups/:groupId/members/me.
func (h *Server) handleLeaveGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	groupID := ps.ByName("groupId")
	if groupID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Group ID required")
		return
	}

	// Existence is checked before membership so a bad/unknown groupId reports
	// 404 rather than being indistinguishable from "not a member".
	exists, err := h.database.GroupExists(r.Context(), groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to check group existence")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to leave group")
		return
	}
	if !exists {
		h.errorResponse(w, http.StatusNotFound, "Group not found")
		return
	}

	isMember, err := h.database.CheckGroupMembership(r.Context(), currentUser.Identifier, groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to check group membership")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to leave group")
		return
	}

	if !isMember {
		h.errorResponse(w, http.StatusForbidden, "Not a member of this group")
		return
	}

	err = h.database.RemoveGroupMember(r.Context(), groupID, currentUser.Identifier)
	if err != nil {
		if errors.Is(err, db.ErrGroupNotFound) {
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		} else {
			h.logger.WithError(err).Error("Failed to leave group")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to leave group")
		}
		return
	}

	h.postSystemMessage(r.Context(), groupID, currentUser.Identifier, currentUser.Username+" left the group")

	h.logger.WithField("groupID", groupID).WithField("userID", currentUser.Identifier).Info("User left group")

	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateGroupName handles PUT /groups/:groupId/name.
func (h *Server) handleUpdateGroupName(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	groupID := ps.ByName("groupId")
	if groupID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Group ID required")
		return
	}

	var req UpdateGroupNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		h.errorResponse(w, http.StatusBadRequest, "Group name required")
		return
	}

	if len(req.Name) > maxGroupNameLength {
		h.errorResponse(w, http.StatusBadRequest, "Group name too long")
		return
	}

	err := h.database.RenameGroup(r.Context(), groupID, currentUser.Identifier, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrUnauthorized):
			h.errorResponse(w, http.StatusForbidden, "Not authorized to rename this group")
		case errors.Is(err, db.ErrGroupNotFound):
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		default:
			h.logger.WithError(err).Error("Failed to rename group")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to rename group")
		}
		return
	}

	h.postSystemMessage(r.Context(), groupID, currentUser.Identifier, "Group renamed to "+req.Name)

	group, err := h.database.FetchGroupInfo(r.Context(), groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch group info")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch group info")
		return
	}

	h.logger.WithField("groupID", groupID).WithField("newName", req.Name).Info("Group renamed")

	h.jsonResponse(w, http.StatusOK, group)
}

// handleUploadGroupPhoto handles PUT /groups/:groupId/photo.
func (h *Server) handleUploadGroupPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	currentUser := getUserFromContext(r.Context())
	if currentUser == nil {
		h.errorResponse(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	groupID := ps.ByName("groupId")
	if groupID == "" {
		h.errorResponse(w, http.StatusBadRequest, "Group ID required")
		return
	}

	if !h.requireMultipartForm(w, r, maxPhotoUploadBytes) {
		return
	}

	photoURL, ok := h.receivePhoto(w, r)
	if !ok {
		return
	}

	err := h.database.SetGroupPhoto(r.Context(), groupID, currentUser.Identifier, photoURL)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrUnauthorized):
			h.errorResponse(w, http.StatusForbidden, "Not authorized to update this group's photo")
		case errors.Is(err, db.ErrGroupNotFound):
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		default:
			h.logger.WithError(err).Error("Failed to update group photo")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to update group photo")
		}
		return
	}

	h.postSystemMessage(r.Context(), groupID, currentUser.Identifier, currentUser.Username+" changed the group photo")

	group, err := h.database.FetchGroupInfo(r.Context(), groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch group info")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch group info")
		return
	}

	h.logger.WithField("groupID", groupID).WithField("photoURL", photoURL).Info("Group photo updated")

	h.jsonResponse(w, http.StatusOK, group)
}
