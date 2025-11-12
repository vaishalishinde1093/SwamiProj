package service

// This file exists solely to provide type definitions for the WhatsApp service wrapper.
// The actual implementations use the existing sendWhatsAppMessage and sendWhatsAppPoll
// functions from main.go through function injection to avoid circular dependencies.

import (
	"go.mau.fi/whatsmeow"
)

// MessageStore interface for querying messages and LID mappings
// This interface is shared across multiple services
type MessageStore interface {
	StoreLidMapping(lid, phoneNumber, groupJID, source string) error
	GetCompletedMembersNames(chatJID string) (map[string]bool, error)
}

// StoredMessage represents a message from the database
type StoredMessage struct {
	ID        string
	ChatJID   string
	Sender    string
	Content   string
	Timestamp int64
	IsFromMe  bool
}

// WhatsAppClient interface for WhatsApp messaging operations
type WhatsAppClient interface {
	SendMessage(recipientJID, message, mediaPath string) (bool, string)
	SendPoll(recipientJID, pollName string, options []string, selectableCount uint32) (bool, string)
	IsConnected() bool
}

// WhatsAppServiceWrapper wraps the whatsmeow client with function injection
type WhatsAppServiceWrapper struct {
	Client       *whatsmeow.Client
	MessageStore MessageStore
	sendMsgFunc  func(*whatsmeow.Client, string, string, string) (bool, string)
	sendPollFunc func(*whatsmeow.Client, MessageStore, string, string, []string, uint32) (bool, string)
}

// NewWhatsAppServiceWrapper creates a new WhatsApp service wrapper
func NewWhatsAppServiceWrapper(
	client *whatsmeow.Client,
	messageStore MessageStore,
	sendMsgFunc func(*whatsmeow.Client, string, string, string) (bool, string),
	sendPollFunc func(*whatsmeow.Client, MessageStore, string, string, []string, uint32) (bool, string),
) *WhatsAppServiceWrapper {
	return &WhatsAppServiceWrapper{
		Client:       client,
		MessageStore: messageStore,
		sendMsgFunc:  sendMsgFunc,
		sendPollFunc: sendPollFunc,
	}
}

// SendMessage implements WhatsAppClient interface
func (w *WhatsAppServiceWrapper) SendMessage(recipientJID, message, mediaPath string) (bool, string) {
	return w.sendMsgFunc(w.Client, recipientJID, message, mediaPath)
}

// SendPoll implements WhatsAppClient interface
func (w *WhatsAppServiceWrapper) SendPoll(recipientJID, pollName string, options []string, selectableCount uint32) (bool, string) {
	return w.sendPollFunc(w.Client, w.MessageStore, recipientJID, pollName, options, selectableCount)
}

// IsConnected implements WhatsAppClient interface
func (w *WhatsAppServiceWrapper) IsConnected() bool {
	return w.Client.IsConnected() && w.Client.IsLoggedIn()
}
