package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Common errors
var (
	ErrUserNotFound          = errors.New("user not found")
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrMessageNotFound       = errors.New("message not found")
	ErrGroupNotFound         = errors.New("group not found")
	ErrCommentNotFound       = errors.New("comment not found")
	ErrUsernameExists        = errors.New("username already exists")
	ErrUnauthorized          = errors.New("unauthorized operation")
	ErrUserNotInConversation = errors.New("user not in conversation")
)

// AppDatabase defines all database operations
type AppDatabase interface {
	// User operations
	LoginOrRegisterUser(username string) (*User, error)
	FindUserByID(id string) (*User, error)
	FindUserByUsername(username string) (*User, error)
	ChangeUsername(userID, newUsername string) (*User, error)
	SetUserPhoto(userID, photoURL string) (*User, error)
	FindUsers(searchQuery string) ([]*User, error)

	// Conversation operations
	FetchUserConversations(userID string) ([]*ConversationPreview, error)
	FetchConversationDetails(conversationID, requestingUserID string) (*ConversationFull, error)
	InitiatePrivateConversation(user1ID, user2ID string) (*ConversationFull, error)
	CheckConversationMembership(userID, conversationID string) (bool, error)

	// Message operations
	PostMessage(msg *Message) (*Message, error)
	FetchMessage(messageID string) (*Message, error)
	RemoveMessage(messageID, userID string) error
	DuplicateMessage(originalMsgID, targetConvID, senderID string) (*Message, error)

	// Reaction operations
	AddReaction(messageID, userID, emoji string) (*Reaction, error)
	RemoveReaction(reactionID, userID string) error
	FetchReaction(reactionID string) (*Reaction, error)

	// Group operations
	CreateNewGroup(name, creatorID string, memberIDs []string) (*Group, error)
	FetchGroupInfo(groupID string) (*Group, error)
	AddGroupMember(groupID, userID, adderID string) error
	RemoveGroupMember(groupID, userID string) error
	RenameGroup(groupID, requesterID, newName string) error
	SetGroupPhoto(groupID, requesterID, photoURL string) error
	CheckGroupMembership(userID, groupID string) (bool, error)

	// Cleanup
	Close() error
}

// SQLiteDatabase implements AppDatabase using SQLite
type SQLiteDatabase struct {
	conn *sql.DB
}

// User represents a user account
type User struct {
	Identifier string  `json:"identifier"`
	Username   string  `json:"username"`
	PhotoURL   *string `json:"photoUrl,omitempty"`
}

// ConversationPreview shows a conversation in the list
type ConversationPreview struct {
	ConversationID        string    `json:"conversationId"`
	ConversationType      string    `json:"conversationType"`
	DisplayName           string    `json:"displayName"`
	DisplayPhotoURL       *string   `json:"displayPhotoUrl,omitempty"`
	LastMessageTimestamp  time.Time `json:"lastMessageTimestamp"`
	LastMessageSnippet    *string   `json:"lastMessageSnippet,omitempty"`
	LastMessageIsPhoto    bool      `json:"lastMessageIsPhoto"`
}

// ConversationFull includes all conversation details and messages
type ConversationFull struct {
	ConversationID   string     `json:"conversationId"`
	ConversationType string     `json:"conversationType"`
	DisplayName      string     `json:"displayName"`
	DisplayPhotoURL  *string    `json:"displayPhotoUrl,omitempty"`
	Participants     []*User    `json:"participants"`
	Messages         []*Message `json:"messages"`
}

// Message represents a single message
type Message struct {
	MessageID         string              `json:"messageId"`
	ConversationID    string              `json:"-"`
	SenderID          string              `json:"senderId"`
	SenderName        string              `json:"senderName"`
	TextContent       *string             `json:"textContent,omitempty"`
	PhotoURL          *string             `json:"photoUrl,omitempty"`
	SentAt            time.Time           `json:"sentAt"`
	ReplyToID         *string             `json:"replyToId,omitempty"`
	ReplyPreview      *MessagePreview     `json:"replyPreview,omitempty"`
	ForwardedFromID   *string             `json:"forwardedFromId,omitempty"`
	DeliveryStatus    string              `json:"deliveryStatus"`
	Reactions         []*Reaction         `json:"reactions"`
}

// MessagePreview is a snippet of a message
type MessagePreview struct {
	MessageID      string  `json:"messageId"`
	SenderName     string  `json:"senderName"`
	ContentPreview *string `json:"contentPreview,omitempty"`
	HasPhoto       bool    `json:"hasPhoto"`
}

// Reaction represents an emoji reaction to a message
type Reaction struct {
	ReactionID string `json:"reactionId"`
	UserID     string `json:"userId"`
	Username   string `json:"username"`
	Emoji      string `json:"emoji"`
}

// Group represents a group chat
type Group struct {
	GroupID      string  `json:"groupId"`
	GroupName    string  `json:"groupName"`
	PhotoURL     *string `json:"photoUrl,omitempty"`
	Participants []*User `json:"participants"`
}

// NewDatabase creates a new database connection and initializes tables
func NewDatabase(dbPath string) (AppDatabase, error) {
	dirPath := filepath.Dir(dbPath)
	if dirPath != "" && dirPath != "." {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &SQLiteDatabase{conn: conn}
	if err := db.initialize(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *SQLiteDatabase) Close() error {
	return db.conn.Close()
}

// initialize sets up the database schema
func (db *SQLiteDatabase) initialize() error {
	schemaSQL := `
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
		sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
		FOREIGN KEY (sender_id) REFERENCES users(identifier) ON DELETE CASCADE,
		FOREIGN KEY (reply_to_id) REFERENCES messages(id) ON DELETE SET NULL,
		FOREIGN KEY (forwarded_from_id) REFERENCES messages(id) ON DELETE SET NULL
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
		FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(identifier) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, sent_at DESC);
	CREATE INDEX IF NOT EXISTS idx_participants_user ON conversation_participants(user_id);
	CREATE INDEX IF NOT EXISTS idx_reactions_msg ON reactions(message_id);
	CREATE INDEX IF NOT EXISTS idx_username_search ON users(username);
	`

	if _, err := db.conn.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema SQL: %w", err)
	}

	return nil
}

// newID generates a new UUID identifier
func newID() string {
	id, _ := uuid.NewV4()
	return id.String()
}

// parseTimeString converts SQLite datetime strings to time.Time
func parseTimeString(s string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}
	return time.Now().UTC()
}

// LoginOrRegisterUser handles simplified login
func (db *SQLiteDatabase) LoginOrRegisterUser(username string) (*User, error) {
	user, err := db.FindUserByUsername(username)
	if err == nil {
		return user, nil
	}

	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	userID := newID()
	now := time.Now().UTC()

	_, err = db.conn.Exec(
		"INSERT INTO users (identifier, username, created_at) VALUES (?, ?, ?)",
		userID, username, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return db.FindUserByUsername(username)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{
		Identifier: userID,
		Username:   username,
	}, nil
}

// FindUserByID retrieves a user by identifier
func (db *SQLiteDatabase) FindUserByID(id string) (*User, error) {
	var user User
	var photoURL sql.NullString

	err := db.conn.QueryRow(
		"SELECT identifier, username, photo_url FROM users WHERE identifier = ?",
		id,
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

// FindUserByUsername retrieves a user by username
func (db *SQLiteDatabase) FindUserByUsername(username string) (*User, error) {
	var user User
	var photoURL sql.NullString

	err := db.conn.QueryRow(
		"SELECT identifier, username, photo_url FROM users WHERE username = ?",
		username,
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

// ChangeUsername updates a user's username
func (db *SQLiteDatabase) ChangeUsername(userID, newUsername string) (*User, error) {
	existingUser, err := db.FindUserByUsername(newUsername)
	if err == nil && existingUser.Identifier != userID {
		return nil, ErrUsernameExists
	}

	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	_, err = db.conn.Exec(
		"UPDATE users SET username = ? WHERE identifier = ?",
		newUsername, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update username: %w", err)
	}

	return db.FindUserByID(userID)
}

// SetUserPhoto sets the user's profile photo
func (db *SQLiteDatabase) SetUserPhoto(userID, photoURL string) (*User, error) {
	_, err := db.conn.Exec(
		"UPDATE users SET photo_url = ? WHERE identifier = ?",
		photoURL, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set photo: %w", err)
	}

	return db.FindUserByID(userID)
}

// FindUsers searches for users by username
func (db *SQLiteDatabase) FindUsers(searchQuery string) ([]*User, error) {
	query := "SELECT identifier, username, photo_url FROM users"
	args := []interface{}{}

	if searchQuery != "" {
		query += " WHERE username LIKE ?"
		args = append(args, "%"+searchQuery+"%")
	}

	query += " ORDER BY username LIMIT 100"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

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

	return users, rows.Err()
}

// FetchUserConversations retrieves all conversations for a user
func (db *SQLiteDatabase) FetchUserConversations(userID string) ([]*ConversationPreview, error) {
	query := `
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
				   ROW_NUMBER() OVER (PARTITION BY conversation_id ORDER BY sent_at DESC) as row_num
			FROM messages
		) m ON c.id = m.conversation_id AND m.row_num = 1
		WHERE cp.user_id = ?
		ORDER BY last_msg_time DESC
	`

	rows, err := db.conn.Query(query, userID, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch conversations: %w", err)
	}
	defer rows.Close()

	conversations := []*ConversationPreview{}
	for rows.Next() {
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

		conversations = append(conversations, &conv)
	}

	return conversations, rows.Err()
}

// FetchConversationDetails retrieves full conversation details
func (db *SQLiteDatabase) FetchConversationDetails(conversationID, requestingUserID string) (*ConversationFull, error) {
	isMember, err := db.CheckConversationMembership(requestingUserID, conversationID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrUserNotInConversation
	}

	var conv ConversationFull
	var groupName, groupPhoto sql.NullString

	err = db.conn.QueryRow(
		"SELECT id, conv_type, group_name, group_photo_url FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&conv.ConversationID, &conv.ConversationType, &groupName, &groupPhoto)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch conversation: %w", err)
	}

	if conv.ConversationType == "private" {
		var otherUsername string
		var otherPhoto sql.NullString
		err = db.conn.QueryRow(`
			SELECT u.username, u.photo_url FROM users u
			JOIN conversation_participants cp ON u.identifier = cp.user_id
			WHERE cp.conversation_id = ? AND u.identifier != ?
		`, conversationID, requestingUserID).Scan(&otherUsername, &otherPhoto)
		if err == nil {
			conv.DisplayName = otherUsername
			if otherPhoto.Valid {
				conv.DisplayPhotoURL = &otherPhoto.String
			}
		}
	} else {
		if groupName.Valid {
			conv.DisplayName = groupName.String
		}
		if groupPhoto.Valid {
			conv.DisplayPhotoURL = &groupPhoto.String
		}
	}

	conv.Participants, err = db.fetchParticipants(conversationID)
	if err != nil {
		return nil, err
	}

	conv.Messages, err = db.fetchMessages(conversationID)
	if err != nil {
		return nil, err
	}

	return &conv, nil
}

// fetchParticipants retrieves participants of a conversation
func (db *SQLiteDatabase) fetchParticipants(conversationID string) ([]*User, error) {
	rows, err := db.conn.Query(`
		SELECT u.identifier, u.username, u.photo_url
		FROM users u
		JOIN conversation_participants cp ON u.identifier = cp.user_id
		WHERE cp.conversation_id = ?
		ORDER BY u.username
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch participants: %w", err)
	}
	defer rows.Close()

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

	return participants, rows.Err()
}

// fetchMessages retrieves messages for a conversation
func (db *SQLiteDatabase) fetchMessages(conversationID string) ([]*Message, error) {
	query := `
		SELECT 
			m.id, m.sender_id, u.username, m.text_content, m.photo_url,
			m.sent_at, m.reply_to_id, m.forwarded_from_id
		FROM messages m
		JOIN users u ON m.sender_id = u.identifier
		WHERE m.conversation_id = ?
		ORDER BY m.sent_at DESC
	`

	rows, err := db.conn.Query(query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}
	defer rows.Close()

	messages := []*Message{}
	for rows.Next() {
		var msg Message
		var textContent, photoURL, replyTo, forwardedFrom, sentAt sql.NullString

		if err := rows.Scan(&msg.MessageID, &msg.SenderID, &msg.SenderName,
			&textContent, &photoURL, &sentAt, &replyTo, &forwardedFrom); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

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
			msg.ReplyPreview, _ = db.fetchMessageSnippet(replyTo.String)
		}
		if forwardedFrom.Valid {
			msg.ForwardedFromID = &forwardedFrom.String
		}

		msg.DeliveryStatus = "sent"
		msg.Reactions, _ = db.fetchReactions(msg.MessageID)

		messages = append(messages, &msg)
	}

	return messages, rows.Err()
}

// fetchMessageSnippet retrieves a message preview
func (db *SQLiteDatabase) fetchMessageSnippet(messageID string) (*MessagePreview, error) {
	var preview MessagePreview
	var textContent, photoURL sql.NullString

	err := db.conn.QueryRow(`
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

// fetchReactions retrieves reactions for a message
func (db *SQLiteDatabase) fetchReactions(messageID string) ([]*Reaction, error) {
	rows, err := db.conn.Query(`
		SELECT r.id, r.user_id, u.username, r.emoji
		FROM reactions r
		JOIN users u ON r.user_id = u.identifier
		WHERE r.message_id = ?
		ORDER BY r.reacted_at
	`, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reactions: %w", err)
	}
	defer rows.Close()

	reactions := []*Reaction{}
	for rows.Next() {
		var reaction Reaction
		if err := rows.Scan(&reaction.ReactionID, &reaction.UserID, &reaction.Username, &reaction.Emoji); err != nil {
			return nil, fmt.Errorf("failed to scan reaction: %w", err)
		}
		reactions = append(reactions, &reaction)
	}

	return reactions, rows.Err()
}

// InitiatePrivateConversation creates or retrieves a private conversation
func (db *SQLiteDatabase) InitiatePrivateConversation(user1ID, user2ID string) (*ConversationFull, error) {
	var convID string
	err := db.conn.QueryRow(`
		SELECT c.id FROM conversations c
		JOIN conversation_participants cp1 ON c.id = cp1.conversation_id
		JOIN conversation_participants cp2 ON c.id = cp2.conversation_id
		WHERE c.conv_type = 'private' AND cp1.user_id = ? AND cp2.user_id = ?
	`, user1ID, user2ID).Scan(&convID)

	if err == nil {
		return db.FetchConversationDetails(convID, user1ID)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check existing conversation: %w", err)
	}

	convID = newID()
	now := time.Now().UTC()

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO conversations (id, conv_type, created_at) VALUES (?, 'private', ?)",
		convID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	_, err = tx.Exec(
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?), (?, ?, ?)",
		convID, user1ID, now, convID, user2ID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add participants: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return db.FetchConversationDetails(convID, user1ID)
}

// CheckConversationMembership checks if user is in conversation
func (db *SQLiteDatabase) CheckConversationMembership(userID, conversationID string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM conversation_participants WHERE conversation_id = ? AND user_id = ?",
		conversationID, userID,
	).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}

	return count > 0, nil
}

// PostMessage creates a new message
func (db *SQLiteDatabase) PostMessage(msg *Message) (*Message, error) {
	msg.MessageID = newID()
	msg.SentAt = time.Now().UTC()

	var textContent, photoURL, replyTo, forwardedFrom interface{}
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

	_, err := db.conn.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, text_content, photo_url, reply_to_id, forwarded_from_id, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.MessageID, msg.ConversationID, msg.SenderID, textContent, photoURL, replyTo, forwardedFrom, msg.SentAt)

	if err != nil {
		return nil, fmt.Errorf("failed to post message: %w", err)
	}

	user, err := db.FindUserByID(msg.SenderID)
	if err != nil {
		return nil, err
	}
	msg.SenderName = user.Username
	msg.DeliveryStatus = "sent"
	msg.Reactions = []*Reaction{}

	if msg.ReplyToID != nil {
		msg.ReplyPreview, _ = db.fetchMessageSnippet(*msg.ReplyToID)
	}

	return msg, nil
}

// FetchMessage retrieves a single message
func (db *SQLiteDatabase) FetchMessage(messageID string) (*Message, error) {
	var msg Message
	var textContent, photoURL, replyTo, forwardedFrom, sentAt, convID sql.NullString

	err := db.conn.QueryRow(`
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

	msg.DeliveryStatus = "sent"
	msg.Reactions, _ = db.fetchReactions(msg.MessageID)

	return &msg, nil
}

// RemoveMessage deletes a message
func (db *SQLiteDatabase) RemoveMessage(messageID, userID string) error {
	result, err := db.conn.Exec(
		"DELETE FROM messages WHERE id = ? AND sender_id = ?",
		messageID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove message: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rows == 0 {
		return ErrUnauthorized
	}

	return nil
}

// DuplicateMessage forwards a message to another conversation
func (db *SQLiteDatabase) DuplicateMessage(originalMsgID, targetConvID, senderID string) (*Message, error) {
	originalMsg, err := db.FetchMessage(originalMsgID)
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

	return db.PostMessage(newMsg)
}

// AddReaction adds an emoji reaction to a message
func (db *SQLiteDatabase) AddReaction(messageID, userID, emoji string) (*Reaction, error) {
	reactionID := newID()
	now := time.Now().UTC()

	_, err := db.conn.Exec(
		"INSERT INTO reactions (id, message_id, user_id, emoji, reacted_at) VALUES (?, ?, ?, ?, ?)",
		reactionID, messageID, userID, emoji, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add reaction: %w", err)
	}

	return db.FetchReaction(reactionID)
}

// RemoveReaction deletes a reaction
func (db *SQLiteDatabase) RemoveReaction(reactionID, userID string) error {
	result, err := db.conn.Exec(
		"DELETE FROM reactions WHERE id = ? AND user_id = ?",
		reactionID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove reaction: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rows == 0 {
		return ErrUnauthorized
	}

	return nil
}

// FetchReaction retrieves a single reaction
func (db *SQLiteDatabase) FetchReaction(reactionID string) (*Reaction, error) {
	var reaction Reaction
	err := db.conn.QueryRow(`
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

// CreateNewGroup creates a new group conversation
func (db *SQLiteDatabase) CreateNewGroup(name, creatorID string, memberIDs []string) (*Group, error) {
	groupID := newID()
	now := time.Now().UTC()

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO conversations (id, conv_type, group_name, created_at) VALUES (?, 'group', ?, ?)",
		groupID, name, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	_, err = tx.Exec(
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
		groupID, creatorID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add creator: %w", err)
	}

	for _, memberID := range memberIDs {
		if memberID != creatorID {
			_, _ = tx.Exec(
				"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
				groupID, memberID, now,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return db.FetchGroupInfo(groupID)
}

// FetchGroupInfo retrieves group information
func (db *SQLiteDatabase) FetchGroupInfo(groupID string) (*Group, error) {
	var group Group
	var photoURL sql.NullString

	err := db.conn.QueryRow(
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

	group.Participants, err = db.fetchParticipants(groupID)
	if err != nil {
		return nil, err
	}

	return &group, nil
}

// AddGroupMember adds a user to a group
func (db *SQLiteDatabase) AddGroupMember(groupID, userID, adderID string) error {
	isMember, err := db.CheckGroupMembership(adderID, groupID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrUnauthorized
	}

	isAlreadyMember, err := db.CheckGroupMembership(userID, groupID)
	if err != nil {
		return err
	}
	if isAlreadyMember {
		return fmt.Errorf("user already in group")
	}

	_, err = db.conn.Exec(
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
		groupID, userID, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to add group member: %w", err)
	}

	return nil
}

// RemoveGroupMember removes a user from a group
func (db *SQLiteDatabase) RemoveGroupMember(groupID, userID string) error {
	result, err := db.conn.Exec(
		"DELETE FROM conversation_participants WHERE conversation_id = ? AND user_id = ?",
		groupID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove group member: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rows == 0 {
		return ErrGroupNotFound
	}

	return nil
}

// RenameGroup changes the group name
func (db *SQLiteDatabase) RenameGroup(groupID, requesterID, newName string) error {
	isMember, err := db.CheckGroupMembership(requesterID, groupID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrUnauthorized
	}

	_, err = db.conn.Exec(
		"UPDATE conversations SET group_name = ? WHERE id = ? AND conv_type = 'group'",
		newName, groupID,
	)
	if err != nil {
		return fmt.Errorf("failed to rename group: %w", err)
	}

	return nil
}

// SetGroupPhoto updates the group photo
func (db *SQLiteDatabase) SetGroupPhoto(groupID, requesterID, photoURL string) error {
	isMember, err := db.CheckGroupMembership(requesterID, groupID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrUnauthorized
	}

	_, err = db.conn.Exec(
		"UPDATE conversations SET group_photo_url = ? WHERE id = ? AND conv_type = 'group'",
		photoURL, groupID,
	)
	if err != nil {
		return fmt.Errorf("failed to set group photo: %w", err)
	}

	return nil
}

// CheckGroupMembership checks if user is in a group
func (db *SQLiteDatabase) CheckGroupMembership(userID, groupID string) (bool, error) {
	return db.CheckConversationMembership(userID, groupID)
}
