package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"runtime"

	"whatsapp-client/repository"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	"rsc.io/qr"
	"github.com/mdp/qrterminal"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// API key validation middleware
func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next(w, r)
			return
		}

		apiKey := os.Getenv("API_KEY")

		if apiKey == "" {
			http.Error(w, "API_KEY not set", http.StatusExpectationFailed)
			return
		}

		providedKey := r.Header.Get("Authorization")
		if providedKey == "" || providedKey != "Bearer "+apiKey {
			http.Error(w, "Unauthorized: Invalid or missing API key", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

type QRCodeState struct {
	mu         sync.RWMutex
	qrCode     string
	isLoggedIn bool
	timestamp  time.Time
}

var qrState = &QRCodeState{}

// update QR code
func (q *QRCodeState) SetQRCode(code string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.qrCode = code
	q.timestamp = time.Now()
	q.isLoggedIn = false
	
	// Print QR code to terminal
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📱 WHATSAPP QR CODE - Scan with your phone:")
	fmt.Println(strings.Repeat("=", 60))
	qrterminal.GenerateHalfBlock(code, qrterminal.L, os.Stdout)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("QR Code expires in 3 minutes\nScan with iOS or Android WhatsApp app\n\n")
}

// mark as logged in
func (q *QRCodeState) SetLoogedIn() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.isLoggedIn = true
	q.qrCode = ""
}

// Get currrent qr code
func (q *QRCodeState) GetQRCode() (string, bool, time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.qrCode, q.isLoggedIn, q.timestamp
}

// Message represents a chat message for our client
type Message struct {
	ID        string
	ChatID    string
	Time      time.Time
	Sender    string
	Content   string
	IsFromMe  bool
	MediaType string
	Filename  string
}

// Database handler for storing message history
type MessageStore struct {
	db          *sql.DB
	pollDB      *sql.DB
	pollDialect string
}

func (store *MessageStore) Close() error {
	if store.pollDB != nil && store.pollDB != store.db {
		_ = store.pollDB.Close()
	}
	return store.db.Close()
}

func (store *MessageStore) StoreLidMapping(lid string, phoneNumber string, groupJID string, source string) error {
	if lid == "" || phoneNumber == "" {
		return nil
	}

	_, err := store.db.Exec(`
    INSERT INTO lid_phone_mapping(lid, phone_number, group_jid, timestamp, source)
    VALUES(?, ?, ?, CURRENT_TIMESTAMP, ?)
    ON CONFLICT(lid) DO UPDATE SET
        phone_number = excluded.phone_number,
        group_jid = excluded.group_jid,
        timestamp = CURRENT_TIMESTAMP,
        source = excluded.source
    `, lid, phoneNumber, groupJID, source)

	return err
}

// Initialize message store
func NewMessageStore() (*MessageStore, error) {
	// Create directory for database if it doesn't exist
	if err := os.MkdirAll("store", 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	// Open SQLite database for messages
	db, err := sql.Open("sqlite3", "file:store/messages.db?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open messages database: %w", err)
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS chats (
            jid TEXT PRIMARY KEY,
            name TEXT,
            last_message_time TIMESTAMP
        );
        CREATE TABLE IF NOT EXISTS messages (
			id TEXT,
			chat_jid TEXT,
			sender TEXT,
			content TEXT,
			timestamp TIMESTAMP,
			is_from_me BOOLEAN,
			media_type TEXT,
			filename TEXT,
			url TEXT,
			media_key BLOB,
			file_sha256 BLOB,
			file_enc_sha256 BLOB,
			file_length INTEGER,
			PRIMARY KEY (id, chat_jid),
			FOREIGN KEY (chat_jid) REFERENCES chats(jid)
		);
		
		CREATE TABLE IF NOT EXISTS lid_phone_mapping (
			lid TEXT PRIMARY KEY,
			phone_number TEXT NOT NULL,
			group_jid TEXT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			source TEXT
		);
		
		CREATE INDEX IF NOT EXISTS idx_lid_phone_mapping_phone ON lid_phone_mapping(phone_number);
		CREATE INDEX IF NOT EXISTS idx_lid_phone_mapping_group ON lid_phone_mapping(group_jid);
		
		CREATE TABLE IF NOT EXISTS poll_data (
			poll_id TEXT PRIMARY KEY,
			poll_name TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			poll_options TEXT NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS poll_votes (
			poll_id TEXT,
			voter_jid TEXT NOT NULL,
			voted_option_names TEXT NOT NULL,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (poll_id, voter_jid),
			FOREIGN KEY (poll_id) REFERENCES poll_data(poll_id)
		);
		
		CREATE INDEX IF NOT EXISTS idx_poll_votes_poll_id ON poll_votes(poll_id);
		CREATE INDEX IF NOT EXISTS idx_poll_data_chat_jid ON poll_data(chat_jid);
		
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	store := &MessageStore{db: db, pollDB: db, pollDialect: "sqlite"}
	if os.Getenv("POSTGRES_DSN") != "" {
		pollDB, err := repository.OpenPostgresFromEnv()
		if err != nil {
			return nil, err
		}
		if err := pollDB.Ping(); err != nil {
			_ = pollDB.Close()
			return nil, fmt.Errorf("failed to ping postgres: %w", err)
		}
		if err := ensurePostgresPollSchema(context.Background(), pollDB); err != nil {
			_ = pollDB.Close()
			return nil, fmt.Errorf("failed to ensure postgres poll schema: %w", err)
		}
		store.pollDB = pollDB
		store.pollDialect = "postgres"
	}

	return store, nil
}

// Store a chat in the database
func (store *MessageStore) StoreChat(jid, name string, lastMessageTime time.Time) error {
	_, err := store.db.Exec(`
        INSERT OR REPLACE INTO chats (jid, name, last_message_time) VALUES (?, ?, ?)`,
		jid, name, lastMessageTime)
	return err
}

// Store a message in the database
func (store *MessageStore) StoreMessage(id, chatJID, sender, content string, timestamp time.Time, isFromMe bool,
	mediaType, filename, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) error {
	// only store if there's actual content or media
	if content == "" && mediaType == "" {
		return nil
	}

	_, err := store.db.Exec(`
        INSERT OR REPLACE INTO messages
        (id, chat_jid, sender, content, timestamp, is_from_me, media_type, filename, url, media_key, file_sha256, file_enc_sha256, file_length)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, chatJID, sender, content, timestamp, isFromMe, mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength)
	return err
}

// Get messages from a chat
func (store *MessageStore) GetMessages(chatJID string, limit int) ([]Message, error) {
	rows, err := store.db.Query(`
        SELECT sender, content, timestamp, is_from_me, media_type, filename
        FROM messages
        WHERE chat_jid = ?
        ORDER BY timestamp DESC
        LIMIT ?`, chatJID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.Sender, &msg.Content, &msg.Time, &msg.IsFromMe, &msg.MediaType, &msg.Filename)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// Get message IDs sent by me from a specific chat
func (store *MessageStore) GetMyMessageIDs(chatJID string, limit int) ([]string, error) {
	rows, err := store.db.Query(`
        SELECT id FROM messages
        WHERE chat_jid = ? AND is_from_me = 1
        ORDER BY timestamp DESC
        LIMIT ?`, chatJID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messageIDs []string
	for rows.Next() {
		var msgID string
		err := rows.Scan(&msgID)
		if err != nil {
			return nil, err
		}
		messageIDs = append(messageIDs, msgID)
	}

	return messageIDs, nil
}

// Function to send a WhatsApp poll
func sendWhatsAppPoll(client *whatsmeow.Client, messageStore *MessageStore, recipient string, pollName string, pollOptions []string, selectableCount uint32) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	// Create JID for recipient
	var recipientJID types.JID
	var err error

	// check if recipient is a JID
	isJID := strings.Contains(recipient, "@")
	if isJID {
		// Parse to JID string
		recipientJID, err = types.ParseJID(recipient)
		if err != nil {
			return false, fmt.Sprintf("Error parsing JID: %v", err)
		}
	} else {
		// Create JID from phone number
		recipientJID = types.JID{
			User:   recipient,
			Server: "s.whatsapp.net", // for personal chats
		}
	}

	// If this is group, capture LID-phone mapping before sending
	if recipientJID.Server == types.GroupServer || recipientJID.Server == types.HiddenUserServer {
		groupInfo, err := client.GetGroupInfo(context.TODO(), recipientJID)
		if err == nil && groupInfo != nil {
			for _, participant := range groupInfo.Participants {
				// Store mapping for all participants
				lid := participant.LID
				phoneJID := participant.PhoneNumber // Use PhoneNumber field, not JID

				if lid != (types.JID{}) && phoneJID != (types.JID{}) {
					err = messageStore.StoreLidMapping(phoneJID.User, phoneJID.String(), recipientJID.String(), "group_participants")
					if err != nil {
						log.Printf("❗ Failed to store LID mapping %s ⇔ %s (from group participants): %v", phoneJID.User, phoneJID.String(), err)
					}
					log.Printf("💡 Captured LID mapping: %s ⇔ %s (from group participants)", phoneJID.User, phoneJID.String())
				}
			}
		}

	}

	// Validate poll options
	if len(pollOptions) < 2 {
		return false, "Polls must have at least 2 options"
	}
	if len(pollOptions) > 12 {
		return false, "Poll can have maximum 12 options"
	}

	// Set default selectable count if not provided
	if selectableCount == 0 {
		selectableCount = 1 // Default to single selection
	}

	// Use official BuildPollCreation method - this is the correct way!
	pollMsg := client.BuildPollCreation(pollName, pollOptions, int(selectableCount))

	sendMsg, err := client.SendMessage(context.Background(), recipientJID, pollMsg)
	if err != nil {
		return false, fmt.Sprintf("Error sending poll: %v", err)
	}

	// Store poll data for future vote matching
	messageStore.StorePollData(sendMsg.ID, pollName, recipientJID.String(), pollOptions)
	log.Printf("✅ Poll data stored with ID: %v (%s, %s, %d options)", sendMsg.ID, pollName, recipientJID.String(), len(pollOptions))

	return true, fmt.Sprintf("Poll sent successfully with %d options", len(pollOptions))
}

// Delete messages by IDs from database
func (store *MessageStore) DeleteMessages(chatJID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	// Build the query with placeholders for the IN clause
	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs)+1)
	args[0] = chatJID
	for i, id := range messageIDs {
		placeholders[i] = "?"
		args[i+1] = id
	}
	q := fmt.Sprintf("DELETE FROM messages WHERE chat_jid = ? AND id IN (%s)", strings.Join(placeholders, ","))
	_, err := store.db.Exec(q, args...)
	return err
}

// Get all chats
func (store *MessageStore) GetChats() (map[string]time.Time, error) {
	rows, err := store.db.Query("SELECT jid, last_message_time FROM chats ORDER BY last_message_time DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make(map[string]time.Time)
	for rows.Next() {
		var jid string
		var lastMessageTime time.Time
		err := rows.Scan(&jid, &lastMessageTime)
		if err != nil {
			return nil, err
		}
		chats[jid] = lastMessageTime
	}

	return chats, nil
}

// Extract text content from a message
func extractTextContent(msg *waProto.Message) string {
	if msg == nil {
		return ""
	}

	// Try to get text content
	if text := msg.GetConversation(); text != "" {
		return text
	} else if extendedText := msg.GetExtendedTextMessage(); extendedText != nil {
		return extendedText.GetText()
	}

	// For now, we're ignoring non-text messages
	return ""
}

type SendMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SendMessageRequest represents the request body for the send message API
type SendMessageRequest struct {
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
	MediaPath string `json:"media_path,omitempty"`
}

// SendPollRequest represents the request body for the send poll API
type SendPollRequest struct {
	Recipient       string   `json:"recipient"`
	PollName        string   `json:"poll_name"`
	PollOptions     []string `json:"poll_options"`
	SelectableCount uint32   `json:"selectable_count,omitempty"`
}

// Function to send a WhatsApp message
func sendWhatsAppMessage(client *whatsmeow.Client, recipient string, message string, mediaPath string) (bool, string) {
	if !client.IsConnected() {
		return false, "Not connected to WhatsApp"
	}

	// Create JID for recipient
	var recipientJID types.JID
	var err error
	isJID := strings.Contains(recipient, "@")
	if isJID {
		// Parse JID string
		recipientJID, err = types.ParseJID(recipient)
		if err != nil {
			return false, fmt.Sprintf("Error parsing JID: %v", err)
		}
	} else {
		// Create JID from phone number
		recipientJID = types.JID{
			User:   recipient,
			Server: "s.whatsapp.net", // For personal chats
		}
	}

	msg := &waProto.Message{}

	// Check if we have media to send
	if mediaPath != "" {
		mediaData, err := os.ReadFile(mediaPath)
		if err != nil {
			return false, fmt.Sprintf("Error reading media file: %v", err)
		}

		// Determine media type and mime type based on file extension
		fIndex := strings.ToLower(mediaPath[strings.LastIndex(mediaPath, ".")+1:])
		var mediaType whatsmeow.MediaType
		var mimeType string

		// Handle different media types
		switch fIndex {
		case "jpg", "jpeg":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/jpeg"
		case "png":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/png"
		case "gif":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/gif"
		case "webp":
			mediaType = whatsmeow.MediaImage
			mimeType = "image/webp"
			// Document types (any other file type):
		default:
			mediaType = whatsmeow.MediaDocument
			mimeType = "application/octet-stream"
		}
		log.Printf("MimeType. %v", mimeType)
		// Upload media to WhatsApp servers
		resp, err := client.Upload(context.Background(), mediaData, mediaType)
		if err != nil {
			return false, fmt.Sprintf("Error uploading media: %v", err)
		}

		fmt.Printf("Media uploaded: %v", resp)

		// Create the appropriate message type based on media type
		switch mediaType {
		case whatsmeow.MediaImage:
			msg.ImageMessage = nil
		case whatsmeow.MediaAudio:
			msg.AudioMessage = nil
		default:
			msg.Conversation = proto.String(message)
		}
	} else {
		msg.Conversation = proto.String(message)
	}
	// Send message
	_, err = client.SendMessage(context.Background(), recipientJID, msg)
	if err != nil {
		return false, fmt.Sprintf("Error sending message: %v", err)
	}

	return true, fmt.Sprintf("Message sent to %s", recipient)
}

// matchPollOptions matches selected option hashes with original poll option names
// Uses SHA-256 hashing as per WhatsApp's poll voting protocol
func matchPollOptions(selectedHashes [][]byte, pollOptions []string) []string {
	var votedNames []string

	// Create a map of option hash -> option name for quick lookup
	optionHashMap := make(map[string]string)
	for _, option := range pollOptions {
		hash := sha256.Sum256([]byte(option))
		optionHashMap[string(hash[:])] = option
	}

	// Match each selected hash with option names
	for _, selectedHash := range selectedHashes {
		if optionName, found := optionHashMap[string(selectedHash)]; found {
			votedNames = append(votedNames, optionName)
		}
	}

	return votedNames
}

// Handle regular incoming messages with media support
func handleMessage(client *whatsmeow.Client, messageStore *MessageStore, msg *events.Message, logger waLog.Logger) {
	// Check if this is a poll vote message
	if msg.Message.GetPollUpdateMessage() != nil {
		pollKey := msg.Message.GetPollUpdateMessage().GetPollCreationMessageKey()
		if pollKey != nil {

			// Extract voter information
			voterName := msg.Info.PushName
			if voterName == "" {
				voterName = msg.Info.Sender.User
			}
			pollMessageID := pollKey.GetID()
			voterJID := msg.Info.Sender.String()
			voterUser := msg.Info.Sender.User

			// Try to capture LID -> Phone mapping
			if strings.Contains(voterJID, "@lid") {
				logger.Debugf("Poll vote from LID: %s (mapping will be built from message sends)", voterJID)
			}

			// Decrypt poll vote to get selected option names!
			votedOptionNames := []string{}
			pollVote, err := client.DecryptPollVote(context.Background(), msg)
			if err != nil {
				logger.Warnf("Failed to decrypt poll vote: %v", err)
			} else if pollVote != nil {
				selectedHashes := pollVote.GetSelectedOptions()
				logger.Debugf("Decrypted %d selected option hashes", len(selectedHashes))

				//retrive origina poll data to match hashes
				_, pollOptions, err := messageStore.GetPollData(pollMessageID)

				if err != nil {
					logger.Warnf("Failed to retrive poll data for %s: %v", pollMessageID, err)
				} else {

					// Match selected hashes with original poll options
					votedOptionNames = matchPollOptions(selectedHashes, pollOptions)
					logger.Debugf("Matched %d voted options: %v", len(votedOptionNames), votedOptionNames)

					// Store the vote with option names
					err = messageStore.StorePollVote(pollMessageID, voterUser, votedOptionNames)
					if err != nil {
						logger.Warnf("Failed to store poll vote: %v", err)
					} else {
						logger.Debugf("Poll vote stored: %s voted for %v on poll %s", voterName, votedOptionNames, pollMessageID)
					}
				}
			}

			// err = messageStore.StoreMessage(
			// 	msg.Info.ID,
			// 	chatJID,
			// 	voterUser,
			// 	fmt.Sprintf("POLL_VOTE:%v:%v", pollMessageID, votedOptionNames), // Include voted names
			// 	msg.Info.Timestamp,
			// 	false,       // Poll votes are never from "me"
			// 	"poll_vote", // Special media type
			// 	"",
			// 	"",
			// 	nil,
			// 	nil,
			// 	nil,
			// 	0,
			// )
			// if err != nil {
			// 	logger.Warnf("Failed to store poll vote message: %v", err)
			// }
			// Don't process further - poll votes don't have regular content
			return
		}

		// Save message to database
		_ = msg.Info.Chat.String()
		sender := msg.Info.Sender.User

		// name := GetChatName(client, messageStore, msg.Info.Chat, chatJID, nil, sender, logger)

		// Update chat in database with the message timestamp (keeps last message time updated)
		// err := messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
		// if err != nil {
		// 	logger.Warnf("Failed to store chat: %v", err)
		// }

		// Extract text content
		content := extractTextContent(msg.Message)

		// Extract media info
		mediaType, filename, url, _, _, _, _ := extractMediaInfo(msg.Message)

		// Skip if there's no content and no media
		if content == "" && mediaType == "" {
			return
		}

		// Store message in database
		// err = messageStore.StoreMessage(
		// 	msg.Info.ID,
		// 	chatJID,
		// 	sender,
		// 	content,
		// 	msg.Info.Timestamp,
		// 	msg.Info.IsFromMe,
		// 	mediaType,
		// 	filename,
		// 	url,
		// 	mediaKey,
		// 	fileSHA256,
		// 	fileEncSHA256,
		// 	fileLength,
		// )
		// if err != nil {
		// 	logger.Warnf("Failed to store message: %v", err)
		// }

		// Log message reception
		timestamp := msg.Info.Timestamp.Format("2006-01-02 15:04:05")
		direction := "<-"
		if msg.Info.IsFromMe {
			direction = "->"
		}
		// Log based on message type
		logger.Debugf(
			"%s | %s | %s | %s | %s | %s | %s",
			timestamp, direction, sender, mediaType, filename, url, content,
		)
	}
}

func extractMediaInfo(message *waE2E.Message) (mediaType string, fileName string, url string, mediaKey []byte, fileSHA256 []byte, fileEncSHA256 []byte, fileLength uint64) {
	return "", "", "", nil, nil, nil, 0
}

// StorePollData stores poll creation information (poll ID, name, options)
func (store *MessageStore) StorePollData(pollID string, pollName string, chatJID string, pollOptions []string) error {
	// Convert poll options to JSON
	optionsJSON, err := json.Marshal(pollOptions)
	if err != nil {
		return fmt.Errorf("failed to marshal poll options: %w", err)
	}

	switch store.pollDialect {
	case "postgres":
		_, err = store.pollDB.Exec(
			`INSERT INTO poll_data (poll_id, poll_name, chat_jid, poll_options)
			 VALUES ($1, $2, $3, $4::jsonb)
			 ON CONFLICT (poll_id) DO UPDATE SET
			 	poll_name = EXCLUDED.poll_name,
			 	chat_jid = EXCLUDED.chat_jid,
			 	poll_options = EXCLUDED.poll_options,
			 	timestamp = now()`,
			pollID, pollName, chatJID, string(optionsJSON),
		)
		return err
	default:
		_, err = store.pollDB.Exec(
			`INSERT OR REPLACE INTO poll_data (poll_id, poll_name, chat_jid, poll_options)
			 VALUES (?, ?, ?, ?)`,
			pollID, pollName, chatJID, string(optionsJSON),
		)
		return err
	}
}

// GetPollData retrieves poll creation information
func (store *MessageStore) GetPollData(pollID string) (pollName string, pollOptions []string, err error) {
	var optionsJSON string
	switch store.pollDialect {
	case "postgres":
		err = store.pollDB.QueryRow(
			`SELECT poll_name, poll_options::text FROM poll_data WHERE poll_id = $1`, pollID,
		).Scan(&pollName, &optionsJSON)
	default:
		err = store.pollDB.QueryRow(
			`SELECT poll_name, poll_options FROM poll_data WHERE poll_id = ?`, pollID,
		).Scan(&pollName, &optionsJSON)
	}
	if err != nil {
		return "", nil, err
	}
	// Parse poll options from JSON
	err = json.Unmarshal([]byte(optionsJSON), &pollOptions)
	if err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal poll options: %w", err)
	}
	return pollName, pollOptions, nil
}

func (store *MessageStore) StorePollVote(pollID, voterJID string, votedOptionNames []string) error {
	// Convert voted option names to JSON
	optionsJSON, err := json.Marshal(votedOptionNames)
	if err != nil {
		return fmt.Errorf("failed to marshal voted options: %w", err)
	}

	switch store.pollDialect {
	case "postgres":
		_, err = store.pollDB.Exec(
			`INSERT INTO poll_votes (poll_id, voter_jid, voted_option_names)
			 VALUES ($1, $2, $3::jsonb)
			 ON CONFLICT (poll_id, voter_jid) DO UPDATE SET
			 	voted_option_names = EXCLUDED.voted_option_names,
			 	timestamp = now()`,
			pollID, voterJID, string(optionsJSON),
		)
		return err
	default:
		_, err = store.pollDB.Exec(
			`INSERT OR REPLACE INTO poll_votes (poll_id, voter_jid, voted_option_names)
			 VALUES (?, ?, ?)`,
			pollID, voterJID, string(optionsJSON),
		)
		return err
	}
}

func ensurePostgresPollSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS poll_data (
			poll_id TEXT PRIMARY KEY,
			poll_name TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			poll_options JSONB NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_poll_data_chat_jid ON poll_data(chat_jid)`,
		`CREATE INDEX IF NOT EXISTS idx_poll_data_chat_jid_ts ON poll_data(chat_jid, timestamp DESC)`,
		`CREATE TABLE IF NOT EXISTS poll_votes (
			poll_id TEXT NOT NULL,
			voter_jid TEXT NOT NULL,
			voted_option_names JSONB NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (poll_id, voter_jid),
			FOREIGN KEY (poll_id) REFERENCES poll_data(poll_id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_poll_votes_poll_id ON poll_votes(poll_id)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func startRESTServer(client *whatsmeow.Client, messageStore *MessageStore, port int) {
	serverAddr := fmt.Sprintf("0.0.0.0:%d", port)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"whatsapp-bridge"}`))
	})

	ln, err := net.Listen("tcp", serverAddr)
	if err != nil {
		log.Fatalf("failed to bind REST server on %s: %v", serverAddr, err)
	}

	log.Printf("Starting REST API server on %s...\n", serverAddr)

	go func() {
		if err := http.Serve(ln, nil); err != nil {
			log.Printf("REST API server error: %v\n", err)
		}
	}()

	componentsBundle, err := initializeRefactoredComponents(client, messageStore)
	if err != nil {
		log.Println("Continuing with legacy endpoints only...")
	} else {
		registerRefactoredHandlers(componentsBundle)
		registerAdminHandlers(componentsBundle)
	}

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		qrCode, _, timestamp := qrState.GetQRCode()
		status := map[string]interface{}{
			"connected": client.IsConnected(),
			"logged_in": client.IsLoggedIn(),
			"has_qr_code": qrCode != "",
			"qr_timestamp": timestamp,
			"qr_age_seconds": 0,
		}
		
		if !timestamp.IsZero() {
			status["qr_age_seconds"] = int(time.Since(timestamp).Seconds())
		}
		
		json.NewEncoder(w).Encode(status)
	})

	http.HandleFunc("/qr-generate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Check if client is already logged in
		if client.IsLoggedIn() {
			w.Write([]byte(`{"status": "already_logged_in", "message": "Client is already logged in"}`))
			return
		}
		
		// Force disconnect and reconnect to generate new QR code
		if client.IsConnected() {
			client.Disconnect()
			time.Sleep(1 * time.Second)
		}
		
		// Start the connection process in a goroutine
		go func() {
			err := client.Connect()
			if err != nil {
				log.Printf("Failed to connect: %v", err)
				return
			}
		}()
		
		w.Write([]byte(`{"status": "generating", "message": "QR code generation started. Check /qr-terminal in 2-3 seconds."}`))
	})

	http.HandleFunc("/qr-terminal", func(w http.ResponseWriter, r *http.Request) {
		qrCode, _, _ := qrState.GetQRCode()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if qrCode == "" {
			w.Write([]byte("No QR code available"))
			return
		}
		
		// Generate QR code as ASCII art for web display
		var buf strings.Builder
		buf.WriteString("\n" + strings.Repeat("=", 60) + "\n")
		buf.WriteString("📱 WHATSAPP QR CODE - Scan with your phone:\n")
		buf.WriteString(strings.Repeat("=", 60) + "\n")
		
		// Capture qrterminal output
		oldStdout := os.Stdout
		pr, pw, _ := os.Pipe()
		os.Stdout = pw
		
		qrterminal.GenerateHalfBlock(qrCode, qrterminal.L, pw)
		pw.Close()
		
		os.Stdout = oldStdout
		
		// Read from pipe and write to response
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			buf.WriteString(scanner.Text() + "\n")
		}
		
		buf.WriteString(strings.Repeat("=", 60) + "\n")
		buf.WriteString("QR Code expires in 3 minutes\n")
		buf.WriteString("Scan with iOS or Android WhatsApp app\n\n")
		
		w.Write([]byte(buf.String()))
	})

	http.HandleFunc("/qr-raw", func(w http.ResponseWriter, r *http.Request) {
		qrCode, _, _ := qrState.GetQRCode()
		w.Header().Set("Content-Type", "text/plain")
		if qrCode == "" {
			w.Write([]byte("No QR code available"))
			return
		}
		// Return the raw QR code content
		w.Write([]byte(qrCode))
	})

	http.HandleFunc("/qr-debug", func(w http.ResponseWriter, r *http.Request) {
		qrCode, _, _ := qrState.GetQRCode()
		w.Header().Set("Content-Type", "text/plain")
		if qrCode == "" {
			w.Write([]byte("No QR code available"))
			return
		}
		w.Write([]byte(fmt.Sprintf("QR Code (%d chars): %s\nUser-Agent: %s\nIsAndroid: %v", 
			len(qrCode), qrCode, r.Header.Get("User-Agent"), 
			strings.Contains(strings.ToLower(r.Header.Get("User-Agent")), "android"))))
	})

	http.HandleFunc("/login-android", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		qrCode, isLoggedIn, timestamp := qrState.GetQRCode()

		//if already logged in
		if isLoggedIn {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "already_logged_in",
				"message":   "WhatsApp is already connected",
				"connected": client.IsConnected(),
			})
			return
		}

		//if QR code is not avaialble
		if qrCode == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "waiting",
				"message": "waiting for QR code generation, please try again in a moment",
			})
			return
		}

		if time.Since(timestamp) > 3*time.Minute {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "expired",
				"message": "QR code has expired. please restart the application",
			})
			return
		}

		// Try different formats for Android
		formats := []string{
			qrCode,                           // Original
			"WA:" + qrCode,                   // With WA: prefix
			strings.Replace(qrCode, ",", "", -1), // Remove commas
			"2," + qrCode,                   // Add prefix
		}

		for i, format := range formats {
			log.Printf("Trying Android format %d: %s...", i+1, format[:min(50, len(format))])
			qrCodeImg, err := qr.Encode(format, qr.M)
			if err != nil {
				continue
			}

			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("X-Android-Format", fmt.Sprintf("%d", i+1))

			png.Encode(w, qrCodeImg.Image())
			return
		}

		http.Error(w, "Failed to generate QR code in any format", http.StatusInternalServerError)
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		qrCode, isLoggedIn, timestamp := qrState.GetQRCode()

		//if already logged in
		if isLoggedIn {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":    "already_logged_in",
				"message":   "WhatsApp is already connected",
				"connected": client.IsConnected(),
			})
			return
		}

		//if QR code is not avaialble
		if qrCode == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "waiting",
				"message": "waiting for QR code generation, please try again in a moment",
			})
			return
		}

		if time.Since(timestamp) > 3*time.Minute {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "expired",
				"message": "QR code has expired. please restart the application",
			})
			return
		}

		// Generate QR code - use simpler approach for Android compatibility
		log.Printf("Generating QR code with %d characters", len(qrCode))
		
		// Use the original QR code without any modifications
		qrCodeImg, err := qr.Encode(qrCode, qr.M) // Use medium error correction instead of high
		if err != nil {
			http.Error(w, "Failed to generate QR code image", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "User-Agent")

		png.Encode(w, qrCodeImg.Image())
	})
	// server is already listening
}

func main() {
	if os.Getenv("IMPORT_MEMBERS") == "1" {
		if err := importMembersFromCSVToPostgres("config/groups.yaml"); err != nil {
			log.Fatalf("failed to import members: %v", err)
		}
		return
	}

	port := 8081
	if p := os.Getenv("PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	// Set up logger
	level := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if level == "" {
		level = "INFO"
	}
	logger := waLog.Stdout("Client", level, true)
	logger.Infof("Starting WhatsApp Client...")

	// Create database connection for storing session data
	//
	// For persistent logins across redeploys on Railway, prefer a persistent SQL store:
	// - Set WHATSAPP_STORE_DSN to use Postgres (recommended)
	// - Otherwise, falls back to SQLite file (use a Railway Volume if you want persistence)
	storeDSN := strings.TrimSpace(os.Getenv("WHATSAPP_STORE_DSN"))
	if storeDSN == "" {
		storeDSN = strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	}

	var (
		container *sqlstore.Container
		err       error
	)

	if storeDSN != "" {
		container, err = sqlstore.New(context.Background(), "pgx", storeDSN, waLog.Stdout("sqlstore", "INFO", true))
	} else {
		sqlitePath := strings.TrimSpace(os.Getenv("WHATSAPP_SQLITE_PATH"))
		if sqlitePath == "" {
			sqlitePath = "store/whatsapp.db"
		}

		if err := os.MkdirAll(filepath.Dir(sqlitePath), 0755); err != nil {
			logger.Errorf("Failed to create store directory: %v", err)
		}

		sqliteDSN := fmt.Sprintf("file:%s?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL", sqlitePath)
		container, err = sqlstore.New(context.Background(), "sqlite3", sqliteDSN, waLog.Stdout("sqlstore", "INFO", true))
	}
	if err != nil {
		log.Fatalf("Failed to create SQL store: %v", err)
	}

	// Get device store - This contains session information
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		if err == sql.ErrNoRows {
			// No device exists, create one
			deviceStore = container.NewDevice()
			logger.Infof("Created new device")
		} else {
			logger.Errorf("Failed to get device: %v", err)
			return
		}
	}

	// Create client instance
	client := whatsmeow.NewClient(deviceStore, logger)
	if client == nil {
		logger.Errorf("Failed to create WhatsApp Client")
		return
	}

	// Initialize message store
	messageStore, err := NewMessageStore()
	if err != nil {
		logger.Errorf("Failed to initialize message store: %v", err)
		return
	}
	defer messageStore.Close()

	// Setup event handling for messages and history sync
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			// Process regular messages
			handleMessage(client, messageStore, v, logger)
		case *events.HistorySync:
			logger.Infof("Received history sync event with %d conversations", len(v.Data.Conversations))
		case *events.Connected:
			logger.Infof("Connected to WhatsApp")
			qrState.SetLoogedIn()
		case *events.LoggedOut:
			logger.Warnf("Device logged out, please scan QR code to log in again")
			qrState.SetQRCode("")
		}
	})

	// Start REST API server
	startRESTServer(client, messageStore, port)

	time.Sleep(500 * time.Millisecond)

	// Keep the process alive for Railway even if WhatsApp connection fails/disconnects.
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		// Create channel to track connection success
		connectedChan := make(chan bool, 1)

		// Connect to WhatsApp
		if client.Store.ID == nil {

			pairPhone := strings.TrimSpace(os.Getenv("WHATSAPP_PAIR_PHONE"))
			pairMethod := strings.ToLower(strings.TrimSpace(os.Getenv("WHATSAPP_PAIR_METHOD")))
			if pairMethod == "" {
				pairMethod = "qr"
			}

			// No ID stored, this is a new client, need to pair with phone
			ctx := context.Background()
			qrChan, _ := client.GetQRChannel(ctx)
			err = client.Connect()
			if err != nil {
				logger.Errorf("Failed to connect: %v", err)
				return
			}

			if pairMethod == "phone" {
				select {
				case evt := <-qrChan:
					if evt.Event == "code" {
						qrState.SetQRCode(evt.Code)
						logger.Infof("QR Code for WhatsApp pairing: %s", evt.Code)
					}
				case <-time.After(15 * time.Second):
					logger.Warnf("Timeout waiting for initial login webscoket")
				}

				if pairPhone == "" {
					logger.Errorf("whatsapp phone not set")
					return
				}

				osName := "Linux"
				switch runtime.GOOS {
				case "darwin":
					osName = "Mac OS"
				case "windows":
					osName = "Windows"
				case "linux":
					osName = "Linux"
				}
				clientDisplayName := fmt.Sprintf("Chrome (%s)", osName)

				code, err := client.PairPhone(ctx, pairPhone, true, whatsmeow.PairClientChrome, clientDisplayName)
				if err != nil {
					logger.Errorf("Failed to generate pair code : %v", err)
					return
				}

				logger.Warnf("\n phone number pairing enabled with code %s", code)
				for evt := range qrChan {
					if evt.Event == "code" {
						qrState.SetQRCode(evt.Code)
						logger.Infof("QR Code for WhatsApp pairing: %s", evt.Code)
					} else if evt.Event == "success" {
						connectedChan <- true
						break
					} else if evt.Event == "error" {
						logger.Errorf("Paring error : %v", evt.Error)
						return
					}
				}
			}

			// If not using phone pairing, listen for QR codes
			if pairMethod != "phone" {
				go func() {
					for evt := range qrChan {
						if evt.Event == "code" {
							qrState.SetQRCode(evt.Code)
							logger.Infof("QR Code for WhatsApp pairing: %s", evt.Code)
						} else if evt.Event == "success" {
							connectedChan <- true
							break
						} else if evt.Event == "error" {
							logger.Errorf("QR error: %v", evt.Error)
							return
						}
					}
				}()
			}

			// Wait for connection
			select {
			case <-connectedChan:
				fmt.Println("Successfully connected and authenticated!")
			case <-time.After(5 * time.Minute):
				logger.Errorf("Timeout waiting for QR code scan!")
				return
			}
		} else {
			err = client.Connect()
			if err != nil {
				logger.Errorf("failed to connect: %v", err)
				return
			}

			qrState.SetLoogedIn()
			connectedChan <- true
		}

		// Wait a moment for connection to stabilize
		time.Sleep(2 * time.Second)

		if !client.IsConnected() {
			logger.Errorf("Failed to establish stable connection")
			return
		}

		fmt.Println("Client connected to WhatsApp! Type 'help' for commands.")
	}()

	fmt.Println("REST server is running. Press Ctrl+C to disconnect and exit.")

	// Wait for termination signal
	<-exitChan

	fmt.Println("Disconnecting...")
	// Disconnect client
	client.Disconnect()

}

func GetChatName(client *whatsmeow.Client, messageStore *MessageStore, jid types.JID, chatJID string, conversation interface{}, sender string, logger waLog.Logger) string {
	var name string

	// Try to use a name that already exists in database with a name
	err := messageStore.db.QueryRow("SELECT name FROM chats WHERE jid = ?", chatJID).Scan(&name)
	if err == nil && name != "" {
		logger.Infof("Using existing chat name for %s: %s", chatJID, name)
		return name
	}

	// Need to determine chat name
	if jid.Server == "g.us" {
		// This is a group chat
		logger.Infof("Getting name for group: %s", chatJID)
		// Try to extract from conversation data if provided (from history sync)
		var displayName string
		var convName string
		if conversation != nil {
			v := reflect.ValueOf(conversation)
			if v.Kind() == reflect.Ptr && !v.IsNil() {
				elem := v.Elem()
				// Try to extract DisplayName field
				displayNameField := elem.FieldByName("DisplayName")
				if displayNameField.IsValid() && displayNameField.Kind() == reflect.Ptr && !displayNameField.IsNil() {
					displayName = displayNameField.Elem().String()
				}
				// Try to find Name field
				nameField := elem.FieldByName("Name")
				if nameField.IsValid() && nameField.Kind() == reflect.Ptr && !nameField.IsNil() {
					convName = nameField.Elem().String()
				}
			}
		}

		// Use the name we found
		if displayName != "" {
			name = displayName
		} else if convName != "" {
			name = convName
		} else {
			// If we didn't get a name, try group info
			groupInfo, err := client.GetGroupInfo(context.TODO(), jid)
			if err == nil && groupInfo.Name != "" {
				name = groupInfo.Name
			} else {
				// Fallback name for groups
				name = fmt.Sprintf("Group: %s", jid.User)
			}
		}
		logger.Infof("Using group name: %s", name)
	} else {
		// This is an individual contact
		logger.Infof("Getting name for contact: %s", chatJID)
		// Just use contact info (full name)
		contact, err := client.Store.Contacts.GetContact(context.Background(), jid)
		if err == nil && contact.FullName != "" {
			name = contact.FullName
		} else if sender != "" {
			// Fallback to sender
			name = sender
		} else {
			// Last fallback to JID
			name = jid.User
		}
		logger.Infof("Using contact name: %s", name)
	}
	return name
}

func (store *MessageStore) GetCompletedMembersNames(charJID string) (map[string]bool, error) {

	completedMembers := make(map[string]bool)

	var (
		rows *sql.Rows
		err  error
	)
	if store.pollDialect == "postgres" {
		rows, err = store.pollDB.Query(`
			SELECT pv.voted_option_names::text
			FROM poll_votes pv
			JOIN poll_data pd ON pd.poll_id = pv.poll_id
			WHERE pd.poll_id IN (
				SELECT poll_id
				FROM poll_data
				WHERE chat_jid = $1
				ORDER BY timestamp DESC
				LIMIT 2
			)
			AND pv.voted_option_names IS NOT NULL
			AND pv.voted_option_names <> 'null'::jsonb
		`, charJID)
	} else {
		rows, err = store.pollDB.Query(`
			SELECT pv.voted_option_names
			FROM poll_votes pv
			WHERE pv.poll_id IN(
				SELECT poll_id
				FROM poll_data
				where chat_jid = ?
				ORDER BY timestamp DESC
				LIMIT 2
				) and pv.voted_option_names != 'null'
		`, charJID)
	}

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var votedOptionsJSON string
		if err := rows.Scan(&votedOptionsJSON); err != nil {
			log.Printf("Error scanning poll votes: %v", err)
			continue
		}

		var votedOptions []string
		if err := json.Unmarshal([]byte(votedOptionsJSON), &votedOptions); err != nil {
			log.Printf("Error unmarshalling voted options: %v", err)
			continue
		}

		for _, fullOption := range votedOptions {
			namePart := fullOption
			if idx := strings.Index(fullOption, " - "); idx != -1 {
				namePart = fullOption[:idx]
			}
			completedMembers[strings.TrimSpace((namePart))] = true
			log.Printf("Completed Seva (voted for): '%s' (from option: '%s')", strings.TrimSpace(namePart), fullOption)
		}
	}

	log.Printf("Total completed members : %d", len(completedMembers))
	return completedMembers, rows.Err()
}
