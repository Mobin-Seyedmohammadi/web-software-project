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
	_ "modernc.org/sqlite"
)

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
)

type AppDatabase interface {
	LoginOrRegisterUser(username string) (*User, error)
	FindUserByID(id string) (*User, error)
	FindUserByUsername(username string) (*User, error)
	ChangeUsername(userID, newUsername string) (*User, error)
	SetUserPhoto(userID, photoURL string) (*User, error)
	FindUsers(searchQuery, excludeUserID string) ([]*User, error)

	FetchUserConversations(userID string) ([]*ConversationPreview, error)
	FetchConversationDetails(conversationID, requestingUserID string) (*ConversationFull, error)
	InitiatePrivateConversation(user1ID, user2ID string) (*ConversationFull, error)
	CheckConversationMembership(userID, conversationID string) (bool, error)
	ConversationExists(conversationID string) (bool, error)
	GroupExists(groupID string) (bool, error)

	PostMessage(msg *Message) (*Message, error)
	FetchMessage(messageID string) (*Message, error)
	RemoveMessage(messageID, userID string) error
	DuplicateMessage(originalMsgID, targetConvID, senderID string) (*Message, error)

	AddReaction(messageID, userID, emoji string) (*Reaction, error)
	RemoveReaction(reactionID, userID string) error
	FetchReaction(reactionID string) (*Reaction, error)

	CreateNewGroup(name, creatorID string, memberIDs []string) (*Group, error)
	FetchGroupInfo(groupID string) (*Group, error)
	AddGroupMember(groupID, userID, adderID string) error
	RemoveGroupMember(groupID, userID string) error
	RenameGroup(groupID, requesterID, newName string) error
	SetGroupPhoto(groupID, requesterID, photoURL string) error
	CheckGroupMembership(userID, groupID string) (bool, error)

	Close() error
}

type SQLiteDatabase struct {
	conn *sql.DB
}

type User struct {
	Identifier string  `json:"identifier"`
	Username   string  `json:"username"`
	PhotoURL   *string `json:"photoUrl,omitempty"`
}

type ConversationPreview struct {
	ConversationID       string    `json:"conversationId"`
	ConversationType     string    `json:"conversationType"`
	DisplayName          string    `json:"displayName"`
	DisplayPhotoURL      *string   `json:"displayPhotoUrl,omitempty"`
	LastMessageTimestamp time.Time `json:"lastMessageTimestamp"`
	LastMessageSnippet   *string   `json:"lastMessageSnippet,omitempty"`
	LastMessageIsPhoto   bool      `json:"lastMessageIsPhoto"`
}

type ConversationFull struct {
	ConversationID   string     `json:"conversationId"`
	ConversationType string     `json:"conversationType"`
	DisplayName      string     `json:"displayName"`
	DisplayPhotoURL  *string    `json:"displayPhotoUrl,omitempty"`
	Participants     []*User    `json:"participants"`
	Messages         []*Message `json:"messages"`
}

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

const (
	MessageTypeUser   = "user"
	MessageTypeSystem = "system"
)

type MessagePreview struct {
	MessageID      string  `json:"messageId"`
	SenderName     string  `json:"senderName"`
	ContentPreview *string `json:"contentPreview,omitempty"`
	HasPhoto       bool    `json:"hasPhoto"`
}

type Reaction struct {
	ReactionID string `json:"reactionId"`
	UserID     string `json:"userId"`
	Username   string `json:"username"`
	Emoji      string `json:"emoji"`
}

type Group struct {
	GroupID      string  `json:"groupId"`
	GroupName    string  `json:"groupName"`
	PhotoURL     *string `json:"photoUrl,omitempty"`
	Participants []*User `json:"participants"`
}

func NewDatabase(dbPath string) (AppDatabase, error) {
	dirPath := filepath.Dir(dbPath)
	if dirPath != "" && dirPath != "." {
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	conn.SetMaxOpenConns(1)

	db := &SQLiteDatabase{conn: conn}
	if err := db.initialize(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return db, nil
}

func (db *SQLiteDatabase) Close() error {
	return db.conn.Close()
}

func (db *SQLiteDatabase) initialize() error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	}
	for _, p := range pragmas {
		if _, err := db.conn.Exec(p); err != nil {
			return fmt.Errorf("failed to set pragma: %w", err)
		}
	}

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

	if _, err := db.conn.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema SQL: %w", err)
	}

	// Migrate databases created before message_type existed. SQLite has no
	// "ADD COLUMN IF NOT EXISTS", so the duplicate-column error is expected
	// and ignored on databases that already have it.
	if _, err := db.conn.Exec("ALTER TABLE messages ADD COLUMN message_type TEXT NOT NULL DEFAULT 'user'"); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("failed to migrate messages table: %w", err)
		}
	}

	return nil
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
		userID, username, now.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return db.FindUserByUsername(username)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{Identifier: userID, Username: username}, nil
}

func (db *SQLiteDatabase) FindUserByID(id string) (*User, error) {
	var user User
	var photoURL sql.NullString

	err := db.conn.QueryRow(
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

func (db *SQLiteDatabase) FindUserByUsername(username string) (*User, error) {
	var user User
	var photoURL sql.NullString

	err := db.conn.QueryRow(
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

func (db *SQLiteDatabase) ChangeUsername(userID, newUsername string) (*User, error) {
	existingUser, err := db.FindUserByUsername(newUsername)
	if err == nil && existingUser.Identifier != userID {
		return nil, ErrUsernameExists
	}
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	_, err = db.conn.Exec("UPDATE users SET username = ? WHERE identifier = ?", newUsername, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return nil, ErrUsernameExists
		}
		return nil, fmt.Errorf("failed to update username: %w", err)
	}
	return db.FindUserByID(userID)
}

func (db *SQLiteDatabase) SetUserPhoto(userID, photoURL string) (*User, error) {
	_, err := db.conn.Exec("UPDATE users SET photo_url = ? WHERE identifier = ?", photoURL, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to set photo: %w", err)
	}
	return db.FindUserByID(userID)
}

func (db *SQLiteDatabase) FindUsers(searchQuery, excludeUserID string) ([]*User, error) {
	query := "SELECT identifier, username, photo_url FROM users WHERE identifier != ?"
	args := []interface{}{excludeUserID}

	if searchQuery != "" {
		query += " AND username LIKE ?"
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

func (db *SQLiteDatabase) FetchUserConversations(userID string) ([]*ConversationPreview, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	// Mark all messages as received for this user
	_, _ = db.conn.Exec(`
		UPDATE message_delivery SET received_at = ?
		WHERE recipient_id = ? AND received_at IS NULL
	`, now, userID)

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
				   ROW_NUMBER() OVER (PARTITION BY conversation_id ORDER BY sent_at DESC) as rn
			FROM messages
		) m ON c.id = m.conversation_id AND m.rn = 1
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

func (db *SQLiteDatabase) FetchConversationDetails(conversationID, requestingUserID string) (*ConversationFull, error) {
	// Existence is checked before membership so a bad/unknown conversationId
	// reports 404 rather than being indistinguishable from "not a member".
	var conv ConversationFull
	var groupName, groupPhoto sql.NullString

	err := db.conn.QueryRow(
		"SELECT id, conv_type, group_name, group_photo_url FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&conv.ConversationID, &conv.ConversationType, &groupName, &groupPhoto)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch conversation: %w", err)
	}

	isMember, err := db.CheckConversationMembership(requestingUserID, conversationID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrUserNotInConversation
	}

	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	// Mark messages in this conversation as received and read for the requesting user
	_, _ = db.conn.Exec(`
		UPDATE message_delivery
		SET received_at = COALESCE(received_at, ?),
		    read_at = COALESCE(read_at, ?)
		WHERE recipient_id = ?
		AND message_id IN (SELECT id FROM messages WHERE conversation_id = ?)
	`, now, now, requestingUserID, conversationID)

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

func (db *SQLiteDatabase) fetchMessages(conversationID string) ([]*Message, error) {
	query := `
		SELECT
			m.id, m.sender_id, u.username, m.text_content, m.photo_url,
			m.sent_at, m.reply_to_id, m.forwarded_from_id, m.message_type,
			COALESCE((SELECT COUNT(*) FROM message_delivery md WHERE md.message_id = m.id), 0) as total_r,
			COALESCE((SELECT COUNT(*) FROM message_delivery md WHERE md.message_id = m.id AND md.received_at IS NOT NULL), 0) as recv_r,
			COALESCE((SELECT COUNT(*) FROM message_delivery md WHERE md.message_id = m.id AND md.read_at IS NOT NULL), 0) as read_r
		FROM messages m
		JOIN users u ON m.sender_id = u.identifier
		WHERE m.conversation_id = ?
		ORDER BY m.sent_at DESC
	`

	rows, err := db.conn.Query(query, conversationID)
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
			rows.Close()
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
		}
		if forwardedFrom.Valid {
			msg.ForwardedFromID = &forwardedFrom.String
		}

		if totalR > 0 && readR >= totalR {
			msg.DeliveryStatus = "read"
		} else if totalR > 0 && recvR >= totalR {
			msg.DeliveryStatus = "received"
		} else {
			msg.DeliveryStatus = "sent"
		}

		messages = append(messages, &msg)
	}
	rowsErr := rows.Err()
	rows.Close()
	if rowsErr != nil {
		return nil, rowsErr
	}

	// Fetch reply previews and reactions after closing the outer rows: the
	// connection pool has only one connection, so a nested query while rows
	// above is still open would deadlock waiting for itself to free up.
	for _, msg := range messages {
		if msg.ReplyToID != nil {
			msg.ReplyPreview, _ = db.fetchMessageSnippet(*msg.ReplyToID)
		}
		msg.Reactions, _ = db.fetchReactions(msg.MessageID)
	}

	return messages, nil
}

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

func (db *SQLiteDatabase) fetchReactions(messageID string) ([]*Reaction, error) {
	rows, err := db.conn.Query(`
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
	defer rows.Close()

	reactions := []*Reaction{}
	for rows.Next() {
		var reaction Reaction
		if err := rows.Scan(&reaction.ReactionID, &reaction.UserID, &reaction.Username, &reaction.Emoji); err != nil {
			return reactions, fmt.Errorf("failed to scan reaction: %w", err)
		}
		reactions = append(reactions, &reaction)
	}
	return reactions, rows.Err()
}

func (db *SQLiteDatabase) InitiatePrivateConversation(user1ID, user2ID string) (*ConversationFull, error) {
	if user1ID == user2ID {
		// The existing-conversation lookup below is a self-join on
		// conversation_participants keyed only by user ID; with the same ID
		// on both sides it would match the first private conversation this
		// user happens to be in with someone else, not a conversation with
		// themselves. Reject outright instead.
		return nil, ErrCannotMessageSelf
	}

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
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.Exec(
		"INSERT INTO conversations (id, conv_type, created_at) VALUES (?, 'private', ?)",
		convID, now,
	); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	if _, err = tx.Exec(
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?), (?, ?, ?)",
		convID, user1ID, now, convID, user2ID, now,
	); err != nil {
		return nil, fmt.Errorf("failed to add participants: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return db.FetchConversationDetails(convID, user1ID)
}

// ConversationExists reports whether a conversation (private or group) with
// this ID exists at all, independent of the requester's membership. Callers
// use this to tell a nonexistent conversation (404) apart from one the
// requester simply isn't a member of (403).
func (db *SQLiteDatabase) ConversationExists(conversationID string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM conversations WHERE id = ?",
		conversationID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check conversation existence: %w", err)
	}
	return count > 0, nil
}

// GroupExists is like ConversationExists but only matches group conversations.
func (db *SQLiteDatabase) GroupExists(groupID string) (bool, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM conversations WHERE id = ? AND conv_type = 'group'",
		groupID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check group existence: %w", err)
	}
	return count > 0, nil
}

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

func (db *SQLiteDatabase) PostMessage(msg *Message) (*Message, error) {
	msg.MessageID = newID()
	msg.SentAt = time.Now().UTC()
	if msg.MessageType == "" {
		msg.MessageType = MessageTypeUser
	}

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

	sentAtStr := msg.SentAt.Format("2006-01-02 15:04:05")

	_, err := db.conn.Exec(`
		INSERT INTO messages (id, conversation_id, sender_id, text_content, photo_url, reply_to_id, forwarded_from_id, message_type, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.MessageID, msg.ConversationID, msg.SenderID, textContent, photoURL, replyTo, forwardedFrom, msg.MessageType, sentAtStr)

	if err != nil {
		return nil, fmt.Errorf("failed to post message: %w", err)
	}

	// Insert delivery records for all recipients. Recipient IDs are collected
	// first and the rows closed before issuing the inserts: the connection
	// pool has only one connection, so an Exec while recipientRows is still
	// open would deadlock waiting for itself to free up.
	// System messages (join/leave/rename/photo announcements) don't need
	// read receipts, so they're skipped.
	if msg.MessageType != MessageTypeSystem {
		recipientRows, err := db.conn.Query(
			"SELECT user_id FROM conversation_participants WHERE conversation_id = ? AND user_id != ?",
			msg.ConversationID, msg.SenderID,
		)
		if err == nil {
			var recipientIDs []string
			for recipientRows.Next() {
				var recipientID string
				if scanErr := recipientRows.Scan(&recipientID); scanErr == nil {
					recipientIDs = append(recipientIDs, recipientID)
				}
			}
			recipientRows.Close()

			for _, recipientID := range recipientIDs {
				_, _ = db.conn.Exec(
					"INSERT OR IGNORE INTO message_delivery (message_id, recipient_id) VALUES (?, ?)",
					msg.MessageID, recipientID,
				)
			}
		}
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

func (db *SQLiteDatabase) RemoveMessage(messageID, userID string) error {
	// First check if message exists
	var senderID string
	err := db.conn.QueryRow("SELECT sender_id FROM messages WHERE id = ?", messageID).Scan(&senderID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMessageNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to check message: %w", err)
	}
	if senderID != userID {
		return ErrUnauthorized
	}

	_, err = db.conn.Exec("DELETE FROM messages WHERE id = ?", messageID)
	if err != nil {
		return fmt.Errorf("failed to remove message: %w", err)
	}
	return nil
}

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

func (db *SQLiteDatabase) AddReaction(messageID, userID, emoji string) (*Reaction, error) {
	reactionID := newID()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	_, err := db.conn.Exec(
		"INSERT OR REPLACE INTO reactions (id, message_id, user_id, emoji, reacted_at) VALUES (?, ?, ?, ?, ?)",
		reactionID, messageID, userID, emoji, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to add reaction: %w", err)
	}

	// Fetch the actual reaction (may have a different ID if replaced)
	var reaction Reaction
	err = db.conn.QueryRow(`
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

func (db *SQLiteDatabase) RemoveReaction(reactionID, userID string) error {
	// Check ownership
	var ownerID string
	err := db.conn.QueryRow("SELECT user_id FROM reactions WHERE id = ?", reactionID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCommentNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to check reaction: %w", err)
	}
	if ownerID != userID {
		return ErrUnauthorized
	}

	_, err = db.conn.Exec("DELETE FROM reactions WHERE id = ?", reactionID)
	if err != nil {
		return fmt.Errorf("failed to remove reaction: %w", err)
	}
	return nil
}

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

func (db *SQLiteDatabase) CreateNewGroup(name, creatorID string, memberIDs []string) (*Group, error) {
	groupID := newID()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.Exec(
		"INSERT INTO conversations (id, conv_type, group_name, created_at) VALUES (?, 'group', ?, ?)",
		groupID, name, now,
	); err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	if _, err = tx.Exec(
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
		groupID, creatorID, now,
	); err != nil {
		return nil, fmt.Errorf("failed to add creator: %w", err)
	}

	for _, memberID := range memberIDs {
		if memberID != creatorID {
			_, _ = tx.Exec(
				"INSERT OR IGNORE INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
				groupID, memberID, now,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return db.FetchGroupInfo(groupID)
}

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

func (db *SQLiteDatabase) AddGroupMember(groupID, userID, adderID string) error {
	exists, err := db.GroupExists(groupID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrGroupNotFound
	}

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

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = db.conn.Exec(
		"INSERT INTO conversation_participants (conversation_id, user_id, joined_at) VALUES (?, ?, ?)",
		groupID, userID, now,
	)
	if err != nil {
		return fmt.Errorf("failed to add group member: %w", err)
	}
	return nil
}

func (db *SQLiteDatabase) RemoveGroupMember(groupID, userID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
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
	if _, err := tx.Exec(
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

func (db *SQLiteDatabase) RenameGroup(groupID, requesterID, newName string) error {
	exists, err := db.GroupExists(groupID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrGroupNotFound
	}

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

func (db *SQLiteDatabase) SetGroupPhoto(groupID, requesterID, photoURL string) error {
	exists, err := db.GroupExists(groupID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrGroupNotFound
	}

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

func (db *SQLiteDatabase) CheckGroupMembership(userID, groupID string) (bool, error) {
	return db.CheckConversationMembership(userID, groupID)
}
