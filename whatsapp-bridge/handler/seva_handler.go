package handler

import (
    "encoding/json"
    "log"
    "net/http"

    "whatsapp-client/domain"
    "whatsapp-client/service"
)

// SevaHandler handles HTTP requests for seva automation
type SevaHandler struct {
    sevaService *service.SevaService
}

// NewSevaHandler creates a new seva handler
func NewSevaHandler(sevaService *service.SevaService) *SevaHandler {
    return &SevaHandler{
        sevaService: sevaService,
    }
}

// SevaRequest represents the request body for seva automation
type SevaRequest struct {
    GroupNo int `json:"group_no"`
}

// SevaResponse represents the response for seva automation
type SevaResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

// HandleSendSeva creates an HTTP handler for sending seva automation
// This sends both a message and a poll to the group
func (h *SevaHandler) HandleSendSeva(sevaType domain.SevaType) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req SevaRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request format", http.StatusBadRequest)
            return
        }

        if req.GroupNo <= 0 {
            http.Error(w, "Valid group number is required", http.StatusBadRequest)
            return
        }

        log.Printf("⚡ Processing seva automation request for %s group %d", sevaType, req.GroupNo)
        err := h.sevaService.SendSevaAutomation(sevaType, req.GroupNo)
        w.Header().Set("Content-Type", "application/json")

        if err != nil {
            log.Printf("❌ Seva automation failed: %v", err)
            w.WriteHeader(http.StatusInternalServerError)
            json.NewEncoder(w).Encode(SevaResponse{
                Success: false,
                Message: err.Error(),
            })
            return
        }

        response := SevaResponse{
            Success: true,
            Message: "Seva automation sent successfully",
        }
        log.Printf("✅ Seva automation sent successfully")
        json.NewEncoder(w).Encode(response)
    }
}
