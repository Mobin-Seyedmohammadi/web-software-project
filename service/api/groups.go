package api

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/yourname/wasatext/service/db"
)

// CreateGroupRequest represents the request to create a group
type CreateGroupRequest struct {
	Name      string   `json:"name"`
	MemberIDs []string `json:"memberIds"`
}

// AddGroupMemberRequest represents the request to add a member to a group
type AddGroupMemberRequest struct {
	UserID string `json:"userId"`
}

// UpdateGroupNameRequest represents the request to update group name
type UpdateGroupNameRequest struct {
	Name string `json:"name"`
}

// handleCreateGroup handles POST /groups
func (h *APIHandler) handleCreateGroup(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

	if len(req.Name) > 100 {
		h.errorResponse(w, http.StatusBadRequest, "Group name too long")
		return
	}

	if req.MemberIDs == nil {
		req.MemberIDs = []string{}
	}

	group, err := h.database.CreateNewGroup(req.Name, currentUser.Identifier, req.MemberIDs)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create group")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to create group")
		return
	}

	h.logger.WithField("groupID", group.GroupID).Info("Group created")

	h.jsonResponse(w, http.StatusCreated, group)
}

// handleAddGroupMember handles POST /groups/:groupId/members
func (h *APIHandler) handleAddGroupMember(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	var req AddGroupMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.UserID == "" {
		h.errorResponse(w, http.StatusBadRequest, "User ID required")
		return
	}

	// Check if user exists
	_, err := h.database.FindUserByID(req.UserID)
	if err != nil {
		if err == db.ErrUserNotFound {
			h.errorResponse(w, http.StatusNotFound, "User not found")
		} else {
			h.logger.WithError(err).Error("Failed to find user")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to add member")
		}
		return
	}

	err = h.database.AddGroupMember(groupID, req.UserID, currentUser.Identifier)
	if err != nil {
		if err == db.ErrUnauthorized {
			h.errorResponse(w, http.StatusForbidden, "Not authorized to add members to this group")
		} else if err == db.ErrGroupNotFound {
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		} else {
			h.logger.WithError(err).Error("Failed to add group member")
			h.errorResponse(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	group, err := h.database.FetchGroupInfo(groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch group info")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch group info")
		return
	}

	h.logger.WithField("groupID", groupID).WithField("userID", req.UserID).Info("User added to group")

	h.jsonResponse(w, http.StatusCreated, group)
}

// handleLeaveGroup handles DELETE /groups/:groupId/members/me
func (h *APIHandler) handleLeaveGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	// Check if user is in group
	isMember, err := h.database.CheckGroupMembership(currentUser.Identifier, groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to check group membership")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to leave group")
		return
	}

	if !isMember {
		h.errorResponse(w, http.StatusForbidden, "Not a member of this group")
		return
	}

	err = h.database.RemoveGroupMember(groupID, currentUser.Identifier)
	if err != nil {
		if err == db.ErrGroupNotFound {
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		} else {
			h.logger.WithError(err).Error("Failed to leave group")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to leave group")
		}
		return
	}

	h.logger.WithField("groupID", groupID).WithField("userID", currentUser.Identifier).Info("User left group")

	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateGroupName handles PUT /groups/:groupId/name
func (h *APIHandler) handleUpdateGroupName(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	if len(req.Name) > 100 {
		h.errorResponse(w, http.StatusBadRequest, "Group name too long")
		return
	}

	err := h.database.RenameGroup(groupID, currentUser.Identifier, req.Name)
	if err != nil {
		if err == db.ErrUnauthorized {
			h.errorResponse(w, http.StatusForbidden, "Not authorized to rename this group")
		} else if err == db.ErrGroupNotFound {
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		} else {
			h.logger.WithError(err).Error("Failed to rename group")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to rename group")
		}
		return
	}

	group, err := h.database.FetchGroupInfo(groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch group info")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch group info")
		return
	}

	h.logger.WithField("groupID", groupID).WithField("newName", req.Name).Info("Group renamed")

	h.jsonResponse(w, http.StatusOK, group)
}

// handleUploadGroupPhoto handles PUT /groups/:groupId/photo
func (h *APIHandler) handleUploadGroupPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

	err = h.database.SetGroupPhoto(groupID, currentUser.Identifier, photoURL)
	if err != nil {
		if err == db.ErrUnauthorized {
			h.errorResponse(w, http.StatusForbidden, "Not authorized to update this group's photo")
		} else if err == db.ErrGroupNotFound {
			h.errorResponse(w, http.StatusNotFound, "Group not found")
		} else {
			h.logger.WithError(err).Error("Failed to update group photo")
			h.errorResponse(w, http.StatusInternalServerError, "Failed to update group photo")
		}
		return
	}

	group, err := h.database.FetchGroupInfo(groupID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to fetch group info")
		h.errorResponse(w, http.StatusInternalServerError, "Failed to fetch group info")
		return
	}

	h.logger.WithField("groupID", groupID).WithField("photoURL", photoURL).Info("Group photo updated")

	h.jsonResponse(w, http.StatusOK, group)
}
