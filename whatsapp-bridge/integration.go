package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"whatsapp-client/config"
	"whatsapp-client/domain"
	"whatsapp-client/handler"
	"whatsapp-client/repository"
	"whatsapp-client/service"

	"go.mau.fi/whatsmeow"
)

// ComponentsBundle holds all initialized services and handlers
type ComponentsBundle struct {
	Config          *config.Config
	MemberStore     repository.MemberStore
	WhatsAppService *service.WhatsAppServiceWrapper
	SevaService     *service.SevaService
	ReminderService *service.ReminderService
	SevaHandler     *handler.SevaHandler
	ReminderHandler *handler.ReminderHandler
}

// initializeRefactoredComponents initializes all services and handlers
func initializeRefactoredComponents(client *whatsmeow.Client, messageStore *MessageStore) (*ComponentsBundle, error) {
	log.Printf("🚦 Initializing refactored components...")

	cfg, err := config.LoadConfig("config/groups.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	log.Printf("🚦 Loaded configuration with %d seva types.", len(cfg.Groups))

	db, err := repository.OpenPostgresFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %w", err)
	}
	pgStore := repository.NewPostgresMemberStore(db)
	if err := pgStore.EnsureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure postgres schema: %w", err)
	}
	log.Printf("🚦 Initialized Postgres member store")

	pollFunc := func(c *whatsmeow.Client, ms service.MessageStore, receipient string, pollName string, pollOptions []string, selectableCount uint32) (bool, string) {
		return sendWhatsAppPoll(c, messageStore, receipient, pollName, pollOptions, selectableCount)
	}

	whatsAppSvc := service.NewWhatsAppServiceWrapper(
		client,
		messageStore,
		sendWhatsAppMessage,
		pollFunc,
	)
	log.Printf("🚦 Created WhatsApp service wrapper")

	// Initialize seva service
	sevaService := service.NewSevaService(pgStore, whatsAppSvc, cfg)
	log.Printf("🚦 Initialized seva service")

	// Initialize reminder service
	reminderService := service.NewReminderService(pgStore, whatsAppSvc, cfg, messageStore)
	log.Printf("🚦 Initialized reminder service")

	// Initialize seva handler
	sevaHandler := handler.NewSevaHandler(sevaService)
	log.Printf("🚦 Initialized seva handler")

	// Initialize reminder handler
	reminderHandler := handler.NewReminderHandler(reminderService)
	log.Printf("🚦 Initialized reminder handler")

	bundle := &ComponentsBundle{
		Config:          cfg,
		MemberStore:     pgStore,
		WhatsAppService: whatsAppSvc,
		SevaService:     sevaService,
		ReminderService: reminderService,
		SevaHandler:     sevaHandler,
		ReminderHandler: reminderHandler,
	}
	log.Println("🚦 All refactored components initialized successfully")
	return bundle, nil
}

func registerRefactoredHandlers(bundle *ComponentsBundle) {
	log.Println("🚦 Registering v2 API endpoints...")

	// Seva Automation Endpoints
	http.HandleFunc("/api/v2/ekadashi-bhagavat-seva",
		apiKeyMiddleware(bundle.SevaHandler.HandleSendSeva(domain.SevaTypeEkadashiBhagavat)))
	log.Println("🚦 Registered: POST /api/v2/ekadashi-bhagavat-seva")

	http.HandleFunc("/api/v2/durga-paath",
		apiKeyMiddleware(bundle.SevaHandler.HandleSendSeva(domain.SevaTypeDurgaPaath)))
	log.Println("🚦 Registered: POST /api/v2/durga-paath")

	http.HandleFunc("/api/v2/saptahik-swami-seva",
		apiKeyMiddleware(bundle.SevaHandler.HandleSendSeva(domain.SevaTypeSaptahikSwami)))
	log.Println("🚦 Registered: POST /api/v2/saptahik-swami-seva")

	http.HandleFunc("/api/v2/malhari",
		apiKeyMiddleware(bundle.SevaHandler.HandleSendSeva(domain.SevaTypeMalhari)))
	log.Println("🚦 Registered: POST /api/v2/malhari")

	http.HandleFunc("/api/v2/darbar",
		apiKeyMiddleware(bundle.SevaHandler.HandleSendSeva(domain.SevaTypeDarbar)))
	log.Println("🚦 Registered: POST /api/v2/darbar")

	// Individual Reminder Endpoints (send private messages)
	http.HandleFunc("/api/v2/ekadashi-bhagavat-seva/send-reminders",
		apiKeyMiddleware(bundle.ReminderHandler.HandleReminders(domain.SevaTypeEkadashiBhagavat)))
	log.Println("🚦 Registered: POST /api/v2/ekadashi-bhagavat-seva/send-reminders")

	http.HandleFunc("/api/v2/durga-paath/send-reminders",
		apiKeyMiddleware(bundle.ReminderHandler.HandleReminders(domain.SevaTypeDurgaPaath)))
	log.Println("🚦 Registered: POST /api/v2/durga-paath/send-reminders")

	http.HandleFunc("/api/v2/saptahik-swami-seva/send-reminders",
		apiKeyMiddleware(bundle.ReminderHandler.HandleReminders(domain.SevaTypeSaptahikSwami)))
	log.Println("🚦 Registered: POST /api/v2/saptahik-swami-seva/send-reminders")

	http.HandleFunc("/api/v2/malhari/send-reminders",
		apiKeyMiddleware(bundle.ReminderHandler.HandleReminders(domain.SevaTypeMalhari)))
	log.Println("🚦 Registered: POST /api/v2/malhari/send-reminders")

	http.HandleFunc("/api/v2/darbar/send-reminders",
		apiKeyMiddleware(bundle.ReminderHandler.HandleReminders(domain.SevaTypeDarbar)))
	log.Println("🚦 Registered: POST /api/v2/darbar/send-reminders")

	// Group Announcement Endpoints (send message to group listing pending members)
	http.HandleFunc("/api/v2/ekadashi-bhagavat-seva/group-announcement",
		apiKeyMiddleware(bundle.ReminderHandler.HandleGroupAnnouncement(domain.SevaTypeEkadashiBhagavat)))
	log.Println("🚦 Registered: POST /api/v2/ekadashi-bhagavat-seva/group-announcement")

	http.HandleFunc("/api/v2/durga-paath/group-announcement",
		apiKeyMiddleware(bundle.ReminderHandler.HandleGroupAnnouncement(domain.SevaTypeDurgaPaath)))
	log.Println("🚦 Registered: POST /api/v2/durga-paath/group-announcement")

	http.HandleFunc("/api/v2/saptahik-swami-seva/group-announcement",
		apiKeyMiddleware(bundle.ReminderHandler.HandleGroupAnnouncement(domain.SevaTypeSaptahikSwami)))
	log.Println("🚦 Registered: POST /api/v2/saptahik-swami-seva/group-announcement")

	http.HandleFunc("/api/v2/malhari/group-announcement",
		apiKeyMiddleware(bundle.ReminderHandler.HandleGroupAnnouncement(domain.SevaTypeMalhari)))
	log.Println("🚦 Registered: POST /api/v2/malhari/group-announcement")

	http.HandleFunc("/api/v2/darbar/group-announcement",
		apiKeyMiddleware(bundle.ReminderHandler.HandleGroupAnnouncement(domain.SevaTypeDarbar)))
	log.Println("🚦 Registered: POST /api/v2/darbar/group-announcement")

	log.Println("🚦 All v2 API endpoints registered successfully")

	http.HandleFunc("/api/v2/whatsapp-logout", apiKeyMiddleware(func(wr http.ResponseWriter, r *http.Request) {
		err := bundle.WhatsAppService.Client.Logout(context.Background())
		if err != nil {
			log.Printf("Logout Error %v", err)
			http.Error(wr, fmt.Sprintf("Logout Failed: %v", err), http.StatusInternalServerError)
		}

		response := struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}{
			Success: true,
			Message: "Logged out successfully, please restart the bridge to login again",
		}
		wr.Header().Set("Content-Type", "application/json")
		json.NewEncoder(wr).Encode(response)
	}))

}
