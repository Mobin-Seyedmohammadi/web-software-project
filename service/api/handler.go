package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/yourname/wasatext/service/db"
)

// APIHandler is the main API server structure
type APIHandler struct {
	database db.AppDatabase
	logger   *logrus.Logger
	router   *httprouter.Router
}

// Config contains configuration for the API handler
type Config struct {
	Database db.AppDatabase
	Logger   *logrus.Logger
}

// NewHandler creates a new API handler
func NewHandler(cfg Config) (*APIHandler, error) {
	handler := &APIHandler{
		database: cfg.Database,
		logger:   cfg.Logger,
		router:   httprouter.New(),
	}

	handler.setupRoutes()

	return handler, nil
}

// Router returns the HTTP router
func (h *APIHandler) Router() *httprouter.Router {
	return h.router
}

// Handler returns an HTTP handler with CORS middleware
func (h *APIHandler) Handler() http.Handler {
	return h.applyCORS(h.router)
}

// applyCORS adds CORS headers to responses
func (h *APIHandler) applyCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "1")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequestHandler is a function type for handling authenticated requests
type RequestHandler func(w http.ResponseWriter, r *http.Request, ps httprouter.Params)

// authenticated wraps handlers that require authentication
func (h *APIHandler) authenticated(handler RequestHandler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if r.Method == http.MethodOptions {
			handler(w, r, ps)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.errorResponse(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			h.errorResponse(w, http.StatusUnauthorized, "Invalid authorization format")
			return
		}

		userID := authHeader[7:]
		if userID == "" {
			h.errorResponse(w, http.StatusUnauthorized, "Empty user identifier")
			return
		}

		user, err := h.database.FindUserByID(userID)
		if err != nil {
			h.errorResponse(w, http.StatusUnauthorized, "Invalid user identifier")
			return
		}

		ctx := setUserInContext(r.Context(), user)
		handler(w, r.WithContext(ctx), ps)
	}
}

// setupRoutes configures all API endpoints
func (h *APIHandler) setupRoutes() {
	// Authentication
	h.router.POST("/session", h.handleLogin)

	// User profile
	h.router.GET("/users", h.authenticated(h.handleSearchUsers))
	h.router.PUT("/users/me/username", h.authenticated(h.handleUpdateUsername))
	h.router.PUT("/users/me/photo", h.authenticated(h.handleUploadUserPhoto))

	// Conversations
	h.router.GET("/conversations", h.authenticated(h.handleGetConversations))
	h.router.POST("/conversations", h.authenticated(h.handleCreateConversation))
	h.router.GET("/conversations/:conversationId", h.authenticated(h.handleGetConversation))

	// Messages
	h.router.POST("/conversations/:conversationId/messages", h.authenticated(h.handleSendMessage))
	h.router.DELETE("/messages/:messageId", h.authenticated(h.handleDeleteMessage))
	h.router.POST("/messages/:messageId/forward", h.authenticated(h.handleForwardMessage))
	h.router.POST("/messages/:messageId/comments", h.authenticated(h.handleAddReaction))
	h.router.DELETE("/messages/:messageId/comments/:commentId", h.authenticated(h.handleRemoveReaction))

	// Groups
	h.router.POST("/groups", h.authenticated(h.handleCreateGroup))
	h.router.POST("/groups/:groupId/members", h.authenticated(h.handleAddGroupMember))
	h.router.DELETE("/groups/:groupId/members/me", h.authenticated(h.handleLeaveGroup))
	h.router.PUT("/groups/:groupId/name", h.authenticated(h.handleUpdateGroupName))
	h.router.PUT("/groups/:groupId/photo", h.authenticated(h.handleUploadGroupPhoto))

	// Static files (photos)
	h.router.ServeFiles("/photos/*filepath", http.Dir("./photos"))
}
