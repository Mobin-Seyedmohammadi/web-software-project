package api

import (
	"context"

	"github.com/yourname/wasatext/service/db"
)

// contextKey is a private type for context keys defined in this package,
// so they can't collide with keys defined elsewhere.
type contextKey string

const userContextKey contextKey = "authenticatedUser"

// setUserInContext stores the user in the request context.
func setUserInContext(ctx context.Context, user *db.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// getUserFromContext retrieves the user from the request context.
func getUserFromContext(ctx context.Context) *db.User {
	user, ok := ctx.Value(userContextKey).(*db.User)
	if !ok {
		return nil
	}
	return user
}
