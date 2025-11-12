package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// API key validation middleware
func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	db *sql.DB
}

func (store *MessageStore) Close() error {
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

	return &MessageStore{db: db}, nil
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
	log.Printf("✅ Poll data stored with ID: %v", sendMsg.ID, pollName, recipientJID.String(), len(pollOptions))

	return true, fmt.Sprintf("Poll sent successfully with %d options", sendMsg.ID, pollName, recipientJID, len(pollOptions))
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
		fmt.Println("MimeType. %v", mimeType)
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
			pollMessageID := pollKey.GetId()
			chatJID := msg.Info.Chat.String()
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
				logger.Infof("Decrypted %d selected option hashes", len(selectedHashes))

				//retrive origina poll data to match hashes
				_, pollOptions, err := messageStore.GetPollData(pollMessageID)

				if err != nil {
					logger.Warnf("Failed to retrive poll data for %s: %v", pollMessageID, err)
				} else {

					// Match selected hashes with original poll options
					votedOptionNames := matchPollOptions(selectedHashes, pollOptions)
					logger.Infof("Matched %d voted options: %v", len(votedOptionNames), votedOptionNames)

					// Store the vote with option names
					err = messageStore.StorePollVote(pollMessageID, voterUser, votedOptionNames)
					if err != nil {
						logger.Warnf("Failed to store poll vote: %v", err)
					} else {
						logger.Infof("Poll vote stored: %s voted for %v on poll %s", voterName, votedOptionNames, pollMessageID)
					}
				}
			}

			// Store poll vote as a special message type (for backwards compatibility)
			err = messageStore.StoreMessage(
				msg.Info.ID,
				chatJID,
				voterUser,
				fmt.Sprintf("POLL_VOTE:%v:%v:%v", pollMessageID, votedOptionNames), // Include voted names
				msg.Info.Timestamp,
				false,       // Poll votes are never from "me"
				"poll_vote", // Special media type
				"",
				"",
				nil,
				nil,
				nil,
				0,
			)
			if err != nil {
				logger.Warnf("Failed to store poll vote message: %v", err)
			}
			// Don't process further - poll votes don't have regular content
			return
		}

		// Save message to database
		chatJID := msg.Info.Chat.String()
		sender := msg.Info.Sender.User

		// Get appropriate chat name (pass nil for conversation since we don't have one for regular messages)
		name := GetChatName(client, messageStore, msg.Info.Chat, chatJID, nil, sender, logger)

		// Update chat in database with the message timestamp (keeps last message time updated)
		err := messageStore.StoreChat(chatJID, name, msg.Info.Timestamp)
		if err != nil {
			logger.Warnf("Failed to store chat: %v", err)
		}

		// Extract text content
		content := extractTextContent(msg.Message)

		// Extract media info
		mediaType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg.Message)

		// Skip if there's no content and no media
		if content == "" && mediaType == "" {
			return
		}

		// Store message in database
		err = messageStore.StoreMessage(
			msg.Info.ID,
			chatJID,
			sender,
			content,
			msg.Info.Timestamp,
			msg.Info.IsFromMe,
			mediaType,
			filename,
			url,
			mediaKey,
			fileSHA256,
			fileEncSHA256,
			fileLength,
		)
		if err != nil {
			logger.Warnf("Failed to store message: %v", err)
		} else {
			// Log message reception
			timestamp := msg.Info.Timestamp.Format("2006-01-02 15:04:05")
			direction := "<-"
			if msg.Info.IsFromMe {
				direction = "->"
			}
			// Log based on message type
			logger.Infof(
				"%s | %s | %s | %s | %s | %s | %s",
				timestamp, direction, sender, mediaType, filename, url, content,
			)
		}
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
	_, err = store.db.Exec(
		`INSERT OR REPLACE INTO poll_data (poll_id, poll_name, chat_jid, poll_options)
            VALUES (?, ?, ?, ?)`,
		pollID, pollName, chatJID, string(optionsJSON),
	)
	return err
}

// GetPollData retrieves poll creation information
func (store *MessageStore) GetPollData(pollID string) (pollName string, pollOptions []string, err error) {
	var optionsJSON string
	err = store.db.QueryRow(
		`SELECT poll_name, poll_options FROM poll_data WHERE poll_id = ?`, pollID,
	).Scan(&pollName, &optionsJSON)
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
	_, err = store.db.Exec(
		`INSERT OR REPLACE INTO poll_votes (poll_id, voter_jid, voted_option_names)
            VALUES (?, ?, ?)`,
		pollID, voterJID, string(optionsJSON),
	)
	return err
}

// Start a REST API server to expose the WhatsApp client functionality
func startRESTServer(client *whatsmeow.Client, messageStore *MessageStore, port int) {
	// Initialize Refactored Components (Phase 2)
	componentsBundle, err := initializeRefactoredComponents(client, messageStore)
	if err != nil {
		log.Println("Continuing with legacy endpoints only...")
	} else {
		// Register the new v2 endpoints (seva automation + reminders)
		registerRefactoredHandlers(componentsBundle)
		// Handler for getting CSV data for all groups
		// Handler for Mathaji single group automation (similar to durga-paath-send)
	}
	// Start the server
	serverAddr := fmt.Sprintf(":%d", port)
	log.Printf("Starting REST API server on %s...\n", serverAddr)

	// Run server in a goroutine so it doesn't block
	go func() {
		if err := http.ListenAndServe(serverAddr, nil); err != nil {
			log.Printf("REST API server error: %v\n", err)
		}
	}()
}

func main() {
	// Set up logger
	logger := waLog.Stdout("Client", "DEBUG", true)
	logger.Infof("Starting WhatsApp Client...")

	// Create database connection for storing session data
	dbLog := waLog.Stdout("Database", "DEBUG", true)

	// Create directory for database if it doesn't exist
	if err := os.MkdirAll("store", 0755); err != nil {
		logger.Errorf("Failed to create store directory: %v", err)
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", "file:store/whatsapp.db?_foreign_keys=on", dbLog)
	if err != nil {
		logger.Errorf("Failed to connect to database: %v", err)
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
		case *events.LoggedOut:
			logger.Warnf("Device logged out, please scan QR code to log in again")
		}
	})

	// Create channel to track connection success
	connectedChan := make(chan bool, 1)

	// Connect to WhatsApp
	if client.Store.ID == nil {
		// No ID stored, this is a new client, need to pair with phone
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			logger.Errorf("Failed to connect: %v", err)
			return
		}

		// Print QR code for pairing with phone
		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan this QR code with your WhatsApp app:")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			} else if evt.Event == "success" || evt.Event == "timeout" || evt.Event == "error" {
				break
			}
		}

		// Wait for connection
		select {
		case <-connectedChan:
			fmt.Println("Successfully connected and authenticated!")
		case <-time.After(1 * time.Minute):
			logger.Errorf("Timeout waiting for QR code scan!")
			return
		}
	} else {
		err = client.Connect()
		if err != nil {
			logger.Errorf("failed to connect: %v", err)
			return
		}
		connectedChan <- true
	}

	// Wait a moment for connection to stabilize
	time.Sleep(2 * time.Second)

	if !client.IsConnected() {
		logger.Errorf("Failed to establish stable connection")
		return
	}

	fmt.Println("Client connected to WhatsApp! Type 'help' for commands.")

	// Start REST API server
	startRESTServer(client, messageStore, 8081)

	// Create a channel to keep the main goroutine alive
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

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
		if displayName != "" && displayName != "" {
			name = displayName
		} else if convName != "" && convName != "" {
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

	rows, err := store.db.Query(`
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
