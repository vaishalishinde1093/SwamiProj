package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"whatsapp-client/domain"
	"whatsapp-client/service"
)

// ReminderHandler handles HTTP requests for sending reminders
type ReminderHandler struct {
	reminderService *service.ReminderService
}

// NewReminderHandler creates a new reminder handler
func NewReminderHandler(reminderService *service.ReminderService) *ReminderHandler {
	return &ReminderHandler{
		reminderService: reminderService,
	}
}

// ReminderRequest represents the request body for reminder endpoints
type ReminderRequest struct {
	GroupNo       int      `json:"group_no"`
	MemberNames   []string `json:"member_names,omitempty"`   // Optional: specific members to remind
	CustomMessage string   `json:"custom_message,omitempty"` // Optional: custom reminder message
}

// ReminderResponse represents the response for reminder endpoints
type ReminderResponse struct {
	Success            bool                           `json:"success"`
	Message            string                         `json:"message"`
	TotalMembers       int                            `json:"total_members"`
	RemindersSent      int                            `json:"reminders_sent"`
	RemindersAttempted int                            `json:"reminders_attempted"`
	RemindersFailed    int                            `json:"reminders_failed"`
	Details            []service.MemberReminderStatus `json:"details,omitempty"`
}

// HandleReminders creates an HTTP handler for reminder endpoints (per seva type)
// This uses automatic poll vote detection to find non-voters
func (h *ReminderHandler) HandleReminders(sevaType domain.SevaType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req ReminderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}
		if req.GroupNo <= 0 {
			http.Error(w, "Valid group number is required", http.StatusBadRequest)
			return
		}
		log.Printf("⚡ Processing reminder request for %s group %d", sevaType, req.GroupNo)
		// Determine which method to use
		var result *service.ReminderResult
		var err error
		// If MemberNames present, send reminders to specific members
		if len(req.MemberNames) > 0 {
			result, err = h.reminderService.SendRemindersToSpecific(sevaType, req.GroupNo, req.MemberNames, req.CustomMessage)
		} else {
			// Automatic detection (default and recommended)
			result, err = h.reminderService.SendRemindersAutomatic(sevaType, req.GroupNo, req.CustomMessage)
		}
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			log.Printf("❌ Reminder request failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ReminderResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}
		// Success
		response := ReminderResponse{
			Success:            true,
			Message:            "Reminders sent successfully",
			TotalMembers:       result.TotalMembers,
			RemindersSent:      result.RemindersSent,
			RemindersAttempted: result.RemindersAttempted,
			RemindersFailed:    result.RemindersFailed,
			Details:            result.Details,
		}
		log.Printf("✅ Reminder request complete! %d sent, %d failed", result.RemindersSent, result.RemindersFailed)
		json.NewEncoder(w).Encode(response)
	}
}

// HandleRemindersToAll creates an HTTP handler for broadcast reminders (all members)
// Use this if you want to remind everyone regardless of poll votes
func (h *ReminderHandler) HandleRemindersToAll(sevaType domain.SevaType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req ReminderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}
		if req.GroupNo <= 0 {
			http.Error(w, "Valid group number is required", http.StatusBadRequest)
			return
		}
		log.Printf("⚡ Broadcasting reminders to ALL members of %s group %d", sevaType, req.GroupNo)
		result, err := h.reminderService.SendRemindersToAll(sevaType, req.GroupNo, req.CustomMessage)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			log.Printf("❌ Broadcast reminder failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ReminderResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}
		response := ReminderResponse{
			Success:            true,
			Message:            "Broadcast reminders sent successfully",
			TotalMembers:       result.TotalMembers,
			RemindersSent:      result.RemindersSent,
			RemindersAttempted: result.RemindersAttempted,
			RemindersFailed:    result.RemindersFailed,
			Details:            result.Details,
		}
		log.Printf("✅ Broadcast complete! %d sent, %d failed", result.RemindersSent, result.RemindersFailed)
		json.NewEncoder(w).Encode(response)
	}
}

// HandleGroupAnnouncement creates an HTTP handler for sending group announcements
// Sends a single message to the group listing members who haven’t completed seva
func (h *ReminderHandler) HandleGroupAnnouncement(sevaType domain.SevaType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req ReminderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}
		if req.GroupNo <= 0 {
			http.Error(w, "Valid group number is required", http.StatusBadRequest)
			return
		}
		message, err := h.reminderService.SendGroupAnnouncement(sevaType, req.GroupNo)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			log.Printf("❌ Group announcement failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ReminderResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}
		response := ReminderResponse{
			Success: true,
			Message: message,
		}
		log.Printf("✅ Group announcement sent successfully")
		json.NewEncoder(w).Encode(response)
	}
}
