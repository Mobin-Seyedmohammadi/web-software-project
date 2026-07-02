// Package db implements the SQLite-backed persistence layer for WASAText:
// users, conversations, messages, reactions and groups.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	// Registers the "sqlite" driver used by sql.Open below.
	_ "modernc.org/sqlite"
)

// Sentinel errors returned by AppDatabase implementations. Callers should
// compare against these with errors.Is rather than direct equality.
var (
	ErrUserNotFound          = errors.New("user not found")
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrMessageNotFound       = errors.New("message not found")
	ErrGroupNotFound         = errors.New("group not found")
	ErrCommentNotFound       = errors.New("comment not found")
	ErrUsernameExists        = errors.New("username already exists")
	ErrUnauthorized          = errors.New("unauthorized operation")
	ErrUserNotInConversation = errors.New("user not in conversation")
	ErrReactionExists        = errors.New("reaction already exists")
	ErrCannotMessageSelf     = errors.New("cannot start a conversation with yourself")
	ErrUserAlreadyInGroup    = errors.New("user already in group")
)

// Message type values stored in messages.message_type.
const (
	MessageTypeUser   = "user"
	MessageTypeSystem = "system"
)

// Message delivery status values reported on Message.DeliveryStatus.
const (
	deliveryStatusSent     = "sent"
	deliveryStatusReceived = "received"
	deliveryStatusRead     = "read"
)

// defaultDirPerm is used for directories created by this package. 0750
// (owner+group only) rather than 0755 so the database/photo directories
// aren't world-readable.
const defaultDirPerm = 0o750

// fetchUserConversationsQuery lists a user's conversations with their
// display name/photo (resolved per-type) and last-message preview, most
// recently active first. Bound to the same userID three times: once for
// each of the two display-name/photo correlated subqueries, once for the
// participant filter.
const fetchUserConversationsQuery = `
	SELECT
		c.id,
		c.conv_type,
		CASE
			WHEN c.conv_type = 'group' THEN c.group_name
			ELSE (SELECT u.username FROM users u
				  JOIN conversation_participants cp2 ON u.identifier = cp2.user_id
				  WHERE cp2.conversation_id = c.id AND u.identifier != ?)
		END as display_name,
		CASE
			WHEN c.conv_type = 'group' THEN c.group_photo_url
			ELSE (SELECT u.photo_url FROM users u
				  JOIN conversation_participants cp2 ON u.identifier = cp2.user_id
				  WHERE cp2.conversation_id = c.id AND u.identifier != ?)
		END as display_photo,
		COALESCE(m.sent_at, c.created_at) as last_msg_time,
		m.text_content as last_msg_text,
		CASE WHEN m.photo_url IS NOT NULL THEN 1 ELSE 0 END as is_photo
	FROM conversations c
	JOIN conversation_participants cp ON c.id = cp.conversation_id
	LEFT JOIN (
		SELECT conversation_id, text_content, photo_url, sent_at,
			   ROW_NUMBER() OVER (PARTITION BY conversation_id ORDER BY sent_at DESC) as rn
		FROM messages
	) m ON c.id = m.conversation_id AND m.rn = 1
	WHERE cp.user_id = ?
	ORDER BY last_msg_time DESC
`

// schemaSQL creates every table and index used by this package, guarded by
// IF NOT EXISTS so it's safe to run on every startup.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
	identifier TEXT PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	photo_url TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS conversations (
	id TEXT PRIMARY KEY,
	conv_type TEXT NOT NULL CHECK(conv_type IN ('private', 'group')),
	group_name TEXT,
	group_photo_url TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS conversation_participants (
	conversation_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (conversation_id, user_id),
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(identifier) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS messages (
	id TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	sender_id TEXT NOT NULL,
	text_content TEXT,
	photo_url TEXT,
	reply_to_id TEXT,
	forwarded_from_id TEXT,
	message_type TEXT NOT NULL DEFAULT 'user',
	sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
	FOREIGN KEY (sender_id) REFERENCES users(identifier) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS message_delivery (
	message_id TEXT NOT NULL,
	recipient_id TEXT NOT NULL,
	received_at DATETIME,
	read_at DATETIME,
	PRIMARY KEY (message_id, recipient_id),
	FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
	FOREIGN KEY (recipient_id) REFERENCES users(identifier) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS reactions (
	id TEXT PRIMARY KEY,
	message_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	emoji TEXT NOT NULL,
	reacted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(message_id, user_id),
	FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(identifier) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_participants_user ON conversation_participants(user_id);
CREATE INDEX IF NOT EXISTS idx_reactions_msg ON reactions(message_id);
CREATE INDEX IF NOT EXISTS idx_delivery_recipient ON message_delivery(recipient_id);
`

// UserStore covers user account operations: registration, lookup, and
// profile edits.
type UserStore interface {
	LoginOrRegisterUser(ctx context.Context, username string) (*User, error)
	FindUserByID(ctx context.Context, id string) (*User, error)
	FindUserByUsername(ctx context.Context, username string) (*User, error)
	ChangeUsername(ctx context.Context, userID, newUsername string) (*User, error)
	SetUserPhoto(ctx context.Context, userID, photoURL string) (*User, error)
	FindUsers(ctx context.Context, searchQuery, excludeUserID string) ([]*User, error)
}

// ConversationStore covers conversation lookup, creation, and membership
// checks (for both private and group conversations).
type ConversationStore interface {
	FetchUserConversations(ctx context.Context, userID string) ([]*ConversationPreview, error)
	FetchConversationDetails(ctx context.Context, conversationID, requestingUserID string) (*ConversationFull, error)
	InitiatePrivateConversation(ctx context.Context, user1ID, user2ID string) (*ConversationFull, error)
	CheckConversationMembership(ctx context.Context, userID, conversationID string) (bool, error)
	ConversationExists(ctx context.Context, conversationID string) (bool, error)
	GroupExists(ctx context.Context, groupID string) (bool, error)
}

// MessageStore covers messages and their reactions.
type MessageStore interface {
	PostMessage(ctx context.Context, msg *Message) (*Message, error)
	FetchMessage(ctx context.Context, messageID string) (*Message, error)
	RemoveMessage(ctx context.Context, messageID, userID string) error
	DuplicateMessage(ctx context.Context, originalMsgID, targetConvID, senderID string) (*Message, error)

	AddReaction(ctx context.Context, messageID, userID, emoji string) (*Reaction, error)
	RemoveReaction(ctx context.Context, reactionID, userID string) error
	FetchReaction(ctx context.Context, reactionID string) (*Reaction, error)
}

// GroupStore covers group conversation metadata and membership management.
type GroupStore interface {
	CreateNewGroup(ctx context.Context, name, creatorID string, memberIDs []string) (*Group, error)
	FetchGroupInfo(ctx context.Context, groupID string) (*Group, error)
	AddGroupMember(ctx context.Context, groupID, userID, adderID string) error
	RemoveGroupMember(ctx context.Context, groupID, userID string) error
	RenameGroup(ctx context.Context, groupID, requesterID, newName string) error
	SetGroupPhoto(ctx context.Context, groupID, requesterID, photoURL string) error
	CheckGroupMembership(ctx context.Context, userID, groupID string) (bool, error)
}

// AppDatabase defines the full set of persistence operations available to
// the API layer, composed from the narrower per-domain stores above.
type AppDatabase interface {
	UserStore
	ConversationStore
	MessageStore
	GroupStore

	Close() error
}

// SQLiteDatabase is the SQLite-backed implementation of AppDatabase.
type SQLiteDatabase struct {
	conn *sql.DB
}

// User is a registered WASAText account.
type User struct {
	Identifier string  `json:"identifier"`
	Username   string  `json:"username"`
	PhotoURL   *string `json:"photoUrl,omitempty"`
}

// ConversationPreview is a summary of a conversation as shown in a user's
// conversation list.
type ConversationPreview struct {
	ConversationID       string    `json:"conversationId"`
	ConversationType     string    `json:"conversationType"`
	DisplayName          string    `json:"displayName"`
	DisplayPhotoURL      *string   `json:"displayPhotoUrl,omitempty"`
	LastMessageTimestamp time.Time `json:"lastMessageTimestamp"`
	LastMessageSnippet   *string   `json:"lastMessageSnippet,omitempty"`
	LastMessageIsPhoto   bool      `json:"lastMessageIsPhoto"`
}

// ConversationFull is a conversation together with its participants and
// full message history.
type ConversationFull struct {
	ConversationID   string     `json:"conversationId"`
	ConversationType string     `json:"conversationType"`
	DisplayName      string     `json:"displayName"`
	DisplayPhotoURL  *string    `json:"displayPhotoUrl,omitempty"`
	Participants     []*User    `json:"participants"`
	Messages         []*Message `json:"messages"`
}

// Message is a single chat message, optionally carrying a photo, a reply
// reference, or a forwarded-from reference.
type Message struct {
	MessageID       string          `json:"messageId"`
	ConversationID  string          `json:"-"`
	SenderID        string          `json:"senderId"`
	SenderName      string          `json:"senderName"`
	TextContent     *string         `json:"textContent,omitempty"`
	PhotoURL        *string         `json:"photoUrl,omitempty"`
	SentAt          time.Time       `json:"sentAt"`
	ReplyToID       *string         `json:"replyToId,omitempty"`
	ReplyPreview    *MessagePreview `json:"replyPreview,omitempty"`
	ForwardedFromID *string         `json:"forwardedFromId,omitempty"`
	MessageType     string          `json:"messageType"`
	DeliveryStatus  string          `json:"deliveryStatus"`
	Reactions       []*Reaction     `json:"reactions"`
}

// MessagePreview is the short summary of a message shown when another
// message replies to it.
type MessagePreview struct {
	MessageID      string  `json:"messageId"`
	SenderName     string  `json:"senderName"`
	ContentPreview *string `json:"contentPreview,omitempty"`
	HasPhoto       bool    `json:"hasPhoto"`
}

// Reaction is a single emoji reaction left by a user on a message.
type Reaction struct {
	ReactionID string `json:"reactionId"`
	UserID     string `json:"userId"`
	Username   string `json:"username"`
	Emoji      string `json:"emoji"`
}

// Group is a group conversation's metadata and membership.
type Group struct {
	GroupID      string  `json:"groupId"`
	GroupName    string  `json:"groupName"`
	PhotoURL     *string `json:"photoUrl,omitempty"`
	Participants []*User `json:"participants"`
}

// ensureWritableDir makes sure dir exists and this process can actually
// write to it, creating it if necessary. It performs a real write-then-
// remove probe rather than trusting permission bits, because a
// freshly-mounted Docker volume can look fine (directory exists, mode bits
// look right) and still reject writes — ownership left over from how the
// volume driver initialized it, a read-only mount, a restricted overlay
// filesystem, and so on. On failure the returned error names the exact
// path and wraps the underlying OS error, so a "can't open database"
// failure raised later points straight back at the real cause instead of
// an opaque SQLite errno.
func ensureWritableDir(dirPath string) error {
	if err := os.MkdirAll(dirPath, defaultDirPerm); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dirPath, err)
	}

	// MkdirAll is a no-op if the directory already exists (e.g. a
	// freshly-mounted, root-owned Docker volume), so it can't be trusted to
	// have applied defaultDirPerm; set it explicitly instead.
	if err := os.Chmod(dirPath, defaultDirPerm); err != nil {
		return fmt.Errorf("failed to set permissions on directory %q: %w", dirPath, err)
	}

	probePath := filepath.Join(dirPath, ".write-test")
	//nolint:gosec // probePath is dirPath + a fixed literal suffix, not user input
	probe, err := os.Create(probePath)
	if err != nil {
		return fmt.Errorf("directory %q exists but is not writable by this process: %w", dirPath, err)
	}
	if closeErr := probe.Close(); closeErr != nil {
		log.Printf("failed to close write-test probe %q: %v", probePath, closeErr)
	}
	if err := os.Remove(probePath); err != nil {
		log.Printf("failed to remove write-test probe %q: %v", probePath, err)
	}
	return nil
}

// NewDatabase opens (creating if necessary) the SQLite database at dbPath
// and ensures its schema is up to date.
func NewDatabase(dbPath string) (*SQLiteDatabase, error) {
	ctx := context.Background()

	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database path %q: %w", dbPath, err)
	}

	dirPath := filepath.Dir(absPath)
	if err := ensureWritableDir(dirPath); err != nil {
		return nil, fmt.Errorf("database directory %q is not ready: %w", dirPath, err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %q: %w", absPath, err)
	}

	if err := conn.PingContext(ctx); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("failed to close database connection: %v", closeErr)
		}
		return nil, fmt.Errorf(
			"failed to ping database at %q (directory %q, exists and writable): %w",
			absPath, dirPath, err,
		)
	}

	conn.SetMaxOpenConns(1)

	sqliteDB := &SQLiteDatabase{conn: conn}
	if err := sqliteDB.initialize(ctx); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("failed to close database connection: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return sqliteDB, nil
}

// Close releases the underlying database connection.
func (db *SQLiteDatabase) Close() error {
	if err := db.conn.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	return nil
}

// LoginOrRegisterUser returns the existing user with this username, or
// creates one if none exists yet.
func (db *SQLiteDatabase) LoginOrRegisterUser(ctx context.Context, username string) (*User, error) {
	user, err := db.FindUserByUsername(ctx, username)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	userID := newID()
	now := time.Now().UTC()

	_, err = db.conn.ExecContext(ctx,
		"INSERT INTO users (identifier, username, created_at) VALUES (?, ?, ?)",
		userID, username, now.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return db.FindUserByUsername(ctx, username)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{Identifier: userID, Username: username}, nil
}

// FindUserByID looks up a user by their identifier.
func (db *SQLiteDatabase) FindUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	var photoURL sql.NullString

	err := db.conn.QueryRowContext(ctx,
		"SELECT identifier, username, photo_url FROM users WHERE identifier = ?", id,
	).Scan(&user.Identifier, &user.Username, &photoURL)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	if photoURL.Valid {
		user.PhotoURL = &photoURL.String
	}
	return &user, nil
}

// FindUserByUsername looks up a user by their username.
func (db *SQLiteDatabase) FindUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	var photoURL sql.NullString

	err := db.conn.QueryRowContext(ctx,
		"SELECT identifier, username, photo_url FROM users WHERE username = ?", username,
	).Scan(&user.Identifier, &user.Username, &photoURL)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	if photoURL.Valid {
		user.PhotoURL = &photoURL.String
	}
	return &user, nil
}

// ChangeUsername renames a user, failing with ErrUsernameExists if another
// user already has that username.
func (db *SQLiteDatabase) ChangeUsername(ctx context.Context, userID, newUsername string) (*User, error) {
	existingUser, err := db.FindUserByUsername(ctx, newUsername)
	if err == nil && existingUser.Identifier != userID {
		return nil, ErrUsernameExists
	}
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	_, err = db.conn.ExecContext(ctx, "UPDATE users SET username = ? WHERE identifier = ?", newUsername, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, ErrUsernameExists
		}
		return nil, fmt.Errorf("failed to update username: %w", err)
	}
	return db.FindUserByID(ctx, userID)
}

// SetUserPhoto updates a user's profile photo URL.
func (db *SQLiteDatabase) SetUserPhoto(ctx context.Context, userID, photoURL string) (*User, error) {
	_, err := db.conn.ExecContext(ctx, "UPDATE users SET photo_url = ? WHERE identifier = ?", photoURL, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to set photo: %w", err)
	}
	return db.FindUserByID(ctx, userID)
}

// FindUsers searches for users by username substring, excluding one user
// (typically the requester) from the results.
func (db *SQLiteDatabase) FindUsers(ctx context.Context, searchQuery, excludeUserID string) ([]*User, error) {
	const maxResults = 100

	query := "SELECT identifier, username, photo_url FROM users WHERE identifier != ?"
	args := []any{excludeUserID}

	if searchQuery != "" {
		query += " AND username LIKE ?"
		args = append(args, "%"+searchQuery+"%")
	}
	//nolint:gosec // G202: maxResults is a local constant, not user input
	query += fmt.Sprintf(" ORDER BY username LIMIT %d", maxResults)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	users := []*User{}
	for rows.Next() {
		var user User
		var photoURL sql.NullString
		if err := rows.Scan(&user.Identifier, &user.Username, &photoURL); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		if photoURL.Valid {
			user.PhotoURL = &photoURL.String
		}
		users = append(users, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate users: %w", err)
	}
	return users, nil
}

// FetchUserConversations lists all conversations a user participates in,
// most recently active first.
func (db *SQLiteDatabase) FetchUserConversations(ctx context.Context, userID string) ([]*ConversationPreview, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	// Mark all messages as received for this user.
	_, _ = db.conn.ExecContext(ctx, `
		UPDATE message_delivery SET received_at = ?
		WHERE recipient_id = ? AND received_at IS NULL
	`, now, userID)

	rows, err := db.conn.QueryContext(ctx, fetchUserConversationsQuery, userID, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch conversations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	conversations := []*ConversationPreview{}
	for rows.Next() {
		conv, err := scanConversationPreview(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, conv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate conversations: %w", err)
	}
	return conversations, nil
}

// scanConversationPreview scans one row of the fetchUserConversationsQuery
// result set.
func scanConversationPreview(rows *sql.Rows) (*ConversationPreview, error) {
	var conv ConversationPreview
	var displayPhoto, lastMsgText, lastMsgTime sql.NullString
	var isPhoto int

	if err := rows.Scan(&conv.ConversationID, &conv.ConversationType, &conv.DisplayName,
		&displayPhoto, &lastMsgTime, &lastMsgText, &isPhoto); err != nil {
		return nil, fmt.Errorf("failed to scan conversation: %w", err)
	}

	if displayPhoto.Valid {
		conv.DisplayPhotoURL = &displayPhoto.String
	}
	if lastMsgText.Valid {
		conv.LastMessageSnippet = &lastMsgText.String
	}
	if lastMsgTime.Valid {
		conv.LastMessageTimestamp = parseTimeString(lastMsgTime.String)
	} else {
		conv.LastMessageTimestamp = time.Now().UTC()
	}
	conv.LastMessageIsPhoto = isPhoto == 1
	return &conv, nil
}

// FetchConversationDetails returns a conversation's full details, provided
// requestingUserID is a participant.
func (db *SQLiteDatabase) FetchConversationDetails(
	ctx context.Context, conversationID, requestingUserID string,
) (*ConversationFull, error) {
	// Existence is checked before membership so a bad/unknown conversationId
	// reports 404 rather than being indistinguishable from "not a member".
	var conv ConversationFull
	var groupName, groupPhoto sql.NullString

	err := db.conn.QueryRowContext(ctx,
		"SELECT id, conv_type, group_name, group_photo_url FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&conv.ConversationID, &conv.ConversationType, &groupName, &groupPhoto)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch conversation: %w", err)
	}

	isMember, err := db.CheckConversationMembership(ctx, requestingUserID, conversationID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrUserNotInConversation
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	// Mark messages in this conversation as received and read for the requesting user.
	_, _ = db.conn.ExecContext(ctx, `
		UPDATE message_delivery
		SET received_at = COALESCE(received_at, ?),
		    read_at = COALESCE(read_at, ?)
		WHERE recipient_id = ?
		AND message_id IN (SELECT id FROM messages WHERE conversation_id = ?)
	`, now, now, requestingUserID, conversationID)

	db.applyConversationDisplay(ctx, &conv, conversationID, requestingUserID, groupName, groupPhoto)

	conv.Participants, err = db.fetchParticipants(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	conv.Messages, err = db.fetchMessages(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	return &conv, nil
}

// InitiatePrivateConversation returns the existing private conversation
// between the two users, creating one if none exists yet.
func (db *SQLiteDatabase) InitiatePrivateConversation(
	ctx context.Context, user1ID, user2ID string,
) (*ConversationFull, error) {
	if user1ID == user2ID {
		// The existing-conversation lookup below is a self-join on
		// conversation_participants keyed only by user ID; with the same ID
		// on both sides it would match the first private conversation this
		// user happens to be in with someone else, not a conversation with
		// themselves. Reject outright instead.
		return nil, ErrCannotMessageSelf
	}

	var convID string
	err := db.conn.QueryRowContext(ctx, `
		SELECT c.id FROM conversations c
		JOIN conversation_participants cp1 ON c.id = cp1.conversation_id
		JOIN conversation_participants cp2 ON c.id = cp2.conversation_id
		WHERE c.conv_type = 'private' AND cp1.user_id = ? AND cp2.user_id = ?
	`, user1ID, user2ID).Scan(&convID)

	if err == nil {
		return db.FetchConversationDetails(ctx, convID, user1ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing conversation: %w", err)
	}

	convID = newID()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			log.Printf("failed to rollback transaction: %v", rbErr)
		}
	}()

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO conversations (id, conv_type, created_at) VALUES (?, 'private', ?)",
		convID, now,
	); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?), (?, ?, ?)",
		convID, user1ID, now, convID, user2ID, now,
	); err != nil {
		return nil, fmt.Errorf("failed to add participants: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return db.FetchConversationDetails(ctx, convID, user1ID)
}

// ConversationExists reports whether a conversation (private or group) with
// this ID exists at all, independent of the requester's membership. Callers
// use this to tell a nonexistent conversation (404) apart from one the
// requester simply isn't a member of (403).
func (db *SQLiteDatabase) ConversationExists(ctx context.Context, conversationID string) (bool, error) {
	var count int
	err := db.conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check conversation existence: %w", err)
	}
	return count > 0, nil
}

// GroupExists is like ConversationExists but only matches group conversations.
func (db *SQLiteDatabase) GroupExists(ctx context.Context, groupID string) (bool, error) {
	var count int
	err := db.conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM conversations WHERE id = ? AND conv_type = 'group'",
		groupID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check group existence: %w", err)
	}
	return count > 0, nil
}

// CheckConversationMembership reports whether userID is a participant of
// conversationID.
func (db *SQLiteDatabase) CheckConversationMembership(
	ctx context.Context, userID, conversationID string,
) (bool, error) {
	var count int
	err := db.conn.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM conversation_participants WHERE conversation_id = ? AND user_id = ?",
		conversationID, userID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}
	return count > 0, nil
}

// PostMessage stores a new message and its per-recipient delivery records.
func (db *SQLiteDatabase) PostMessage(ctx context.Context, msg *Message) (*Message, error) {
	msg.MessageID = newID()
	msg.SentAt = time.Now().UTC()
	if msg.MessageType == "" {
		msg.MessageType = MessageTypeUser
	}

	var textContent, photoURL, replyTo, forwardedFrom any
	if msg.TextContent != nil {
		textContent = *msg.TextContent
	}
	if msg.PhotoURL != nil {
		photoURL = *msg.PhotoURL
	}
	if msg.ReplyToID != nil {
		replyTo = *msg.ReplyToID
	}
	if msg.ForwardedFromID != nil {
		forwardedFrom = *msg.ForwardedFromID
	}

	sentAtStr := msg.SentAt.Format("2006-01-02 15:04:05")

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO messages
			(id, conversation_id, sender_id, text_content, photo_url, reply_to_id, forwarded_from_id, message_type, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.MessageID, msg.ConversationID, msg.SenderID, textContent, photoURL, replyTo,
		forwardedFrom, msg.MessageType, sentAtStr)

	if err != nil {
		return nil, fmt.Errorf("failed to post message: %w", err)
	}

	db.insertDeliveryRecords(ctx, msg)

	user, err := db.FindUserByID(ctx, msg.SenderID)
	if err != nil {
		return nil, err
	}
	msg.SenderName = user.Username
	msg.DeliveryStatus = deliveryStatusSent
	msg.Reactions = []*Reaction{}

	if msg.ReplyToID != nil {
		msg.ReplyPreview, _ = db.fetchMessageSnippet(ctx, *msg.ReplyToID)
	}

	return msg, nil
}

// FetchMessage looks up a single message by ID.
func (db *SQLiteDatabase) FetchMessage(ctx context.Context, messageID string) (*Message, error) {
	var msg Message
	var textContent, photoURL, replyTo, forwardedFrom, sentAt, convID sql.NullString

	err := db.conn.QueryRowContext(ctx, `
		SELECT m.id, m.conversation_id, m.sender_id, u.username,
			   m.text_content, m.photo_url, m.sent_at,
			   m.reply_to_id, m.forwarded_from_id
		FROM messages m
		JOIN users u ON m.sender_id = u.identifier
		WHERE m.id = ?
	`, messageID).Scan(&msg.MessageID, &convID, &msg.SenderID, &msg.SenderName,
		&textContent, &photoURL, &sentAt, &replyTo, &forwardedFrom)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMessageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}

	if convID.Valid {
		msg.ConversationID = convID.String
	}
	if textContent.Valid {
		msg.TextContent = &textContent.String
	}
	if photoURL.Valid {
		msg.PhotoURL = &photoURL.String
	}
	if sentAt.Valid {
		msg.SentAt = parseTimeString(sentAt.String)
	}
	if replyTo.Valid {
		msg.ReplyToID = &replyTo.String
	}
	if forwardedFrom.Valid {
		msg.ForwardedFromID = &forwardedFrom.String
	}

	msg.DeliveryStatus = deliveryStatusSent
	msg.Reactions, _ = db.fetchReactions(ctx, msg.MessageID)
	return &msg, nil
}

// RemoveMessage deletes a message, provided userID is its sender.
func (db *SQLiteDatabase) RemoveMessage(ctx context.Context, messageID, userID string) error {
	var senderID string
	err := db.conn.QueryRowContext(ctx, "SELECT sender_id FROM messages WHERE id = ?", messageID).Scan(&senderID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMessageNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to check message: %w", err)
	}
	if senderID != userID {
		return ErrUnauthorized
	}

	_, err = db.conn.ExecContext(ctx, "DELETE FROM messages WHERE id = ?", messageID)
	if err != nil {
		return fmt.Errorf("failed to remove message: %w", err)
	}
	return nil
}

// DuplicateMessage forwards an existing message into another conversation.
func (db *SQLiteDatabase) DuplicateMessage(
	ctx context.Context, originalMsgID, targetConvID, senderID string,
) (*Message, error) {
	originalMsg, err := db.FetchMessage(ctx, originalMsgID)
	if err != nil {
		return nil, err
	}

	newMsg := &Message{
		ConversationID:  targetConvID,
		SenderID:        senderID,
		TextContent:     originalMsg.TextContent,
		PhotoURL:        originalMsg.PhotoURL,
		ForwardedFromID: &originalMsgID,
	}

	return db.PostMessage(ctx, newMsg)
}

// AddReaction records (or replaces) userID's emoji reaction on a message.
func (db *SQLiteDatabase) AddReaction(ctx context.Context, messageID, userID, emoji string) (*Reaction, error) {
	reactionID := newID()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	_, err := db.conn.ExecContext(ctx,
		"INSERT OR REPLACE INTO reactions (id, message_id, user_id, emoji, reacted_at) VALUES (?, ?, ?, ?, ?)",
		reactionID, messageID, userID, emoji, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add reaction: %w", err)
	}

	// Fetch the actual reaction (may have a different ID if replaced).
	var reaction Reaction
	err = db.conn.QueryRowContext(ctx, `
		SELECT r.id, r.user_id, u.username, r.emoji
		FROM reactions r
		JOIN users u ON r.user_id = u.identifier
		WHERE r.message_id = ? AND r.user_id = ?
	`, messageID, userID).Scan(&reaction.ReactionID, &reaction.UserID, &reaction.Username, &reaction.Emoji)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reaction: %w", err)
	}
	return &reaction, nil
}

// RemoveReaction deletes a reaction, provided userID is the one who left it.
func (db *SQLiteDatabase) RemoveReaction(ctx context.Context, reactionID, userID string) error {
	var ownerID string
	err := db.conn.QueryRowContext(ctx, "SELECT user_id FROM reactions WHERE id = ?", reactionID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCommentNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to check reaction: %w", err)
	}
	if ownerID != userID {
		return ErrUnauthorized
	}

	_, err = db.conn.ExecContext(ctx, "DELETE FROM reactions WHERE id = ?", reactionID)
	if err != nil {
		return fmt.Errorf("failed to remove reaction: %w", err)
	}
	return nil
}

// FetchReaction looks up a single reaction by ID.
func (db *SQLiteDatabase) FetchReaction(ctx context.Context, reactionID string) (*Reaction, error) {
	var reaction Reaction
	err := db.conn.QueryRowContext(ctx, `
		SELECT r.id, r.user_id, u.username, r.emoji
		FROM reactions r
		JOIN users u ON r.user_id = u.identifier
		WHERE r.id = ?
	`, reactionID).Scan(&reaction.ReactionID, &reaction.UserID, &reaction.Username, &reaction.Emoji)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCommentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reaction: %w", err)
	}
	return &reaction, nil
}

// CreateNewGroup creates a new group conversation owned by creatorID with
// the given initial members.
func (db *SQLiteDatabase) CreateNewGroup(
	ctx context.Context, name, creatorID string, memberIDs []string,
) (*Group, error) {
	groupID := newID()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			log.Printf("failed to rollback transaction: %v", rbErr)
		}
	}()

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO conversations (id, conv_type, group_name, created_at) VALUES (?, 'group', ?, ?)",
		groupID, name, now,
	); err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	if _, err = tx.ExecContext(ctx,
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
		groupID, creatorID, now,
	); err != nil {
		return nil, fmt.Errorf("failed to add creator: %w", err)
	}

	for _, memberID := range memberIDs {
		if memberID != creatorID {
			_, _ = tx.ExecContext(ctx,
				"INSERT OR IGNORE INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
				groupID, memberID, now,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return db.FetchGroupInfo(ctx, groupID)
}

// FetchGroupInfo looks up a group's metadata and participants.
func (db *SQLiteDatabase) FetchGroupInfo(ctx context.Context, groupID string) (*Group, error) {
	var group Group
	var photoURL sql.NullString

	err := db.conn.QueryRowContext(ctx,
		"SELECT id, group_name, group_photo_url FROM conversations WHERE id = ? AND conv_type = 'group'",
		groupID,
	).Scan(&group.GroupID, &group.GroupName, &photoURL)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGroupNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch group: %w", err)
	}

	if photoURL.Valid {
		group.PhotoURL = &photoURL.String
	}

	group.Participants, err = db.fetchParticipants(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// AddGroupMember adds userID to a group, provided adderID is already a
// member.
func (db *SQLiteDatabase) AddGroupMember(ctx context.Context, groupID, userID, adderID string) error {
	exists, err := db.GroupExists(ctx, groupID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrGroupNotFound
	}

	isMember, err := db.CheckGroupMembership(ctx, adderID, groupID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrUnauthorized
	}

	isAlreadyMember, err := db.CheckGroupMembership(ctx, userID, groupID)
	if err != nil {
		return err
	}
	if isAlreadyMember {
		return ErrUserAlreadyInGroup
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = db.conn.ExecContext(ctx,
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
		groupID, userID, now,
	)
	if err != nil {
		return fmt.Errorf("failed to add group member: %w", err)
	}
	return nil
}

// RemoveGroupMember removes userID from a group.
func (db *SQLiteDatabase) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			log.Printf("failed to rollback transaction: %v", rbErr)
		}
	}()

	result, err := tx.ExecContext(ctx,
		"DELETE FROM conversation_participants WHERE conversation_id = ? AND user_id = ?",
		groupID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove group member: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rowsAffected == 0 {
		return ErrGroupNotFound
	}

	// Drop this member's pending delivery records for the conversation. Without
	// this, a message can never reach "read" once every remaining member has
	// seen it, because the departed member's unread row still counts toward
	// the total.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM message_delivery WHERE recipient_id = ? AND message_id IN (
			SELECT id FROM messages WHERE conversation_id = ?
		)`,
		userID, groupID,
	); err != nil {
		return fmt.Errorf("failed to clean up delivery records: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// RenameGroup changes a group's display name, provided requesterID is a
// member.
func (db *SQLiteDatabase) RenameGroup(ctx context.Context, groupID, requesterID, newName string) error {
	exists, err := db.GroupExists(ctx, groupID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrGroupNotFound
	}

	isMember, err := db.CheckGroupMembership(ctx, requesterID, groupID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrUnauthorized
	}

	_, err = db.conn.ExecContext(ctx,
		"UPDATE conversations SET group_name = ? WHERE id = ? AND conv_type = 'group'",
		newName, groupID,
	)
	if err != nil {
		return fmt.Errorf("failed to rename group: %w", err)
	}
	return nil
}

// SetGroupPhoto changes a group's photo URL, provided requesterID is a
// member.
func (db *SQLiteDatabase) SetGroupPhoto(ctx context.Context, groupID, requesterID, photoURL string) error {
	exists, err := db.GroupExists(ctx, groupID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrGroupNotFound
	}

	isMember, err := db.CheckGroupMembership(ctx, requesterID, groupID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrUnauthorized
	}

	_, err = db.conn.ExecContext(ctx,
		"UPDATE conversations SET group_photo_url = ? WHERE id = ? AND conv_type = 'group'",
		photoURL, groupID,
	)
	if err != nil {
		return fmt.Errorf("failed to set group photo: %w", err)
	}
	return nil
}

// CheckGroupMembership reports whether userID is a member of groupID.
func (db *SQLiteDatabase) CheckGroupMembership(ctx context.Context, userID, groupID string) (bool, error) {
	return db.CheckConversationMembership(ctx, userID, groupID)
}

func (db *SQLiteDatabase) initialize(ctx context.Context) error {
	if err := db.applyPragmas(ctx); err != nil {
		return err
	}

	if _, err := db.conn.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema SQL: %w", err)
	}

	return db.migrateMessageType(ctx)
}

func (db *SQLiteDatabase) applyPragmas(ctx context.Context) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	}
	for _, p := range pragmas {
		if _, err := db.conn.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("failed to set pragma: %w", err)
		}
	}
	return nil
}

// migrateMessageType adds the message_type column to databases created
// before it existed. SQLite has no "ADD COLUMN IF NOT EXISTS", so the
// duplicate-column error is expected and ignored on databases that already
// have it.
func (db *SQLiteDatabase) migrateMessageType(ctx context.Context) error {
	if _, err := db.conn.ExecContext(ctx,
		"ALTER TABLE messages ADD COLUMN message_type TEXT NOT NULL DEFAULT 'user'"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("failed to migrate messages table: %w", err)
		}
	}
	return nil
}

// applyConversationDisplay fills in conv's DisplayName/DisplayPhotoURL: for
// group conversations from the already-fetched group_name/group_photo_url,
// for private conversations by looking up the other participant.
func (db *SQLiteDatabase) applyConversationDisplay(
	ctx context.Context, conv *ConversationFull, conversationID, requestingUserID string,
	groupName, groupPhoto sql.NullString,
) {
	if conv.ConversationType != "private" {
		if groupName.Valid {
			conv.DisplayName = groupName.String
		}
		if groupPhoto.Valid {
			conv.DisplayPhotoURL = &groupPhoto.String
		}
		return
	}

	var otherUsername string
	var otherPhoto sql.NullString
	err := db.conn.QueryRowContext(ctx, `
		SELECT u.username, u.photo_url FROM users u
		JOIN conversation_participants cp ON u.identifier = cp.user_id
		WHERE cp.conversation_id = ? AND u.identifier != ?
	`, conversationID, requestingUserID).Scan(&otherUsername, &otherPhoto)
	if err != nil {
		return
	}
	conv.DisplayName = otherUsername
	if otherPhoto.Valid {
		conv.DisplayPhotoURL = &otherPhoto.String
	}
}

func (db *SQLiteDatabase) fetchParticipants(ctx context.Context, conversationID string) ([]*User, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT u.identifier, u.username, u.photo_url
		FROM users u
		JOIN conversation_participants cp ON u.identifier = cp.user_id
		WHERE cp.conversation_id = ?
		ORDER BY u.username
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch participants: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	participants := []*User{}
	for rows.Next() {
		var user User
		var photoURL sql.NullString
		if err := rows.Scan(&user.Identifier, &user.Username, &photoURL); err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}
		if photoURL.Valid {
			user.PhotoURL = &photoURL.String
		}
		participants = append(participants, &user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate participants: %w", err)
	}
	return participants, nil
}

// messageDeliveryStatus derives a message's overall delivery status from
// the recipient count and how many have received/read it.
func messageDeliveryStatus(total, received, read int) string {
	switch {
	case total > 0 && read >= total:
		return deliveryStatusRead
	case total > 0 && received >= total:
		return deliveryStatusReceived
	default:
		return deliveryStatusSent
	}
}

// populateMessageOptionalFields copies the nullable scanned columns of a
// messages row onto msg.
func populateMessageOptionalFields(msg *Message, textContent, photoURL, replyTo, forwardedFrom, sentAt sql.NullString) {
	if textContent.Valid {
		msg.TextContent = &textContent.String
	}
	if photoURL.Valid {
		msg.PhotoURL = &photoURL.String
	}
	if sentAt.Valid {
		msg.SentAt = parseTimeString(sentAt.String)
	} else {
		msg.SentAt = time.Now().UTC()
	}
	if replyTo.Valid {
		msg.ReplyToID = &replyTo.String
	}
	if forwardedFrom.Valid {
		msg.ForwardedFromID = &forwardedFrom.String
	}
}

func (db *SQLiteDatabase) fetchMessages(ctx context.Context, conversationID string) ([]*Message, error) {
	query := `
		SELECT
			m.id, m.sender_id, u.username, m.text_content, m.photo_url,
			m.sent_at, m.reply_to_id, m.forwarded_from_id, m.message_type,
			COALESCE((SELECT COUNT(*) FROM message_delivery md WHERE md.message_id = m.id), 0) as total_r,
			COALESCE((SELECT COUNT(*) FROM message_delivery md
				WHERE md.message_id = m.id AND md.received_at IS NOT NULL), 0) as recv_r,
			COALESCE((SELECT COUNT(*) FROM message_delivery md
				WHERE md.message_id = m.id AND md.read_at IS NOT NULL), 0) as read_r
		FROM messages m
		JOIN users u ON m.sender_id = u.identifier
		WHERE m.conversation_id = ?
		ORDER BY m.sent_at DESC
	`

	rows, err := db.conn.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	messages := []*Message{}
	for rows.Next() {
		var msg Message
		var textContent, photoURL, replyTo, forwardedFrom, sentAt sql.NullString
		var totalR, recvR, readR int

		if err := rows.Scan(&msg.MessageID, &msg.SenderID, &msg.SenderName,
			&textContent, &photoURL, &sentAt, &replyTo, &forwardedFrom, &msg.MessageType,
			&totalR, &recvR, &readR); err != nil {
			// Not deferred: the connection pool has exactly one connection
			// (see fetchMessages' own closing comment below), so rows must be
			// closed before this function's caller can issue another query.
			if closeErr := rows.Close(); closeErr != nil { //nolint:sqlclosecheck // see comment above
				log.Printf("failed to close rows: %v", closeErr)
			}
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		populateMessageOptionalFields(&msg, textContent, photoURL, replyTo, forwardedFrom, sentAt)
		msg.DeliveryStatus = messageDeliveryStatus(totalR, recvR, readR)

		messages = append(messages, &msg)
	}
	rowsErr := rows.Err()
	if closeErr := rows.Close(); closeErr != nil {
		log.Printf("failed to close rows: %v", closeErr)
	}
	if rowsErr != nil {
		return nil, fmt.Errorf("failed to iterate messages: %w", rowsErr)
	}

	// Fetch reply previews and reactions after closing the outer rows: the
	// connection pool has only one connection, so a nested query while rows
	// above is still open would deadlock waiting for itself to free up.
	for _, msg := range messages {
		if msg.ReplyToID != nil {
			msg.ReplyPreview, _ = db.fetchMessageSnippet(ctx, *msg.ReplyToID)
		}
		msg.Reactions, _ = db.fetchReactions(ctx, msg.MessageID)
	}

	return messages, nil
}

func (db *SQLiteDatabase) fetchMessageSnippet(ctx context.Context, messageID string) (*MessagePreview, error) {
	var preview MessagePreview
	var textContent, photoURL sql.NullString

	err := db.conn.QueryRowContext(ctx, `
		SELECT m.id, u.username, m.text_content, m.photo_url
		FROM messages m
		JOIN users u ON m.sender_id = u.identifier
		WHERE m.id = ?
	`, messageID).Scan(&preview.MessageID, &preview.SenderName, &textContent, &photoURL)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch message snippet: %w", err)
	}

	if textContent.Valid {
		preview.ContentPreview = &textContent.String
	}
	preview.HasPhoto = photoURL.Valid
	return &preview, nil
}

func (db *SQLiteDatabase) fetchReactions(ctx context.Context, messageID string) ([]*Reaction, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT r.id, r.user_id, u.username, r.emoji
		FROM reactions r
		JOIN users u ON r.user_id = u.identifier
		WHERE r.message_id = ?
		ORDER BY r.reacted_at
	`, messageID)
	if err != nil {
		// Callers of fetchReactions discard the error and keep whatever slice
		// is returned (Message.Reactions has no omitempty), so an empty slice
		// here matters: nil would serialize as JSON null instead of [].
		return []*Reaction{}, fmt.Errorf("failed to fetch reactions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("failed to close rows: %v", closeErr)
		}
	}()

	reactions := []*Reaction{}
	for rows.Next() {
		var reaction Reaction
		if err := rows.Scan(&reaction.ReactionID, &reaction.UserID, &reaction.Username, &reaction.Emoji); err != nil {
			return reactions, fmt.Errorf("failed to scan reaction: %w", err)
		}
		reactions = append(reactions, &reaction)
	}
	if err := rows.Err(); err != nil {
		return reactions, fmt.Errorf("failed to iterate reactions: %w", err)
	}
	return reactions, nil
}

// insertDeliveryRecords creates a pending message_delivery row for every
// other participant of msg's conversation. Best-effort: failures here don't
// fail message posting, since delivery/read tracking is secondary to the
// message itself having been stored.
func (db *SQLiteDatabase) insertDeliveryRecords(ctx context.Context, msg *Message) {
	// System messages (join/leave/rename/photo announcements) don't need
	// read receipts, so they're skipped.
	if msg.MessageType == MessageTypeSystem {
		return
	}

	// Recipient IDs are collected first and the rows closed before issuing
	// the inserts: the connection pool has only one connection, so an Exec
	// while recipientRows is still open would deadlock waiting for itself to
	// free up.
	recipientRows, err := db.conn.QueryContext(ctx,
		"SELECT user_id FROM conversation_participants WHERE conversation_id = ? AND user_id != ?",
		msg.ConversationID, msg.SenderID,
	)
	if err != nil {
		return
	}

	var recipientIDs []string
	for recipientRows.Next() {
		var recipientID string
		if scanErr := recipientRows.Scan(&recipientID); scanErr == nil {
			recipientIDs = append(recipientIDs, recipientID)
		}
	}
	if err := recipientRows.Err(); err != nil {
		log.Printf("failed to iterate recipients: %v", err)
	}
	// Not deferred: see the "single connection" comment above the query
	// that produced recipientRows — it must close before the inserts below.
	if closeErr := recipientRows.Close(); closeErr != nil { //nolint:sqlclosecheck // see comment above
		log.Printf("failed to close rows: %v", closeErr)
	}

	for _, recipientID := range recipientIDs {
		_, _ = db.conn.ExecContext(ctx,
			"INSERT OR IGNORE INTO message_delivery (message_id, recipient_id) VALUES (?, ?)",
			msg.MessageID, recipientID,
		)
	}
}

func newID() string {
	id, _ := uuid.NewV4()
	return id.String()
}

func parseTimeString(s string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}
