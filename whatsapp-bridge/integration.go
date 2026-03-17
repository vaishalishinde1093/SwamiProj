package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"whatsapp-client/config"
	"whatsapp-client/domain"
	"whatsapp-client/handler"
	"whatsapp-client/repository"
	"whatsapp-client/service"

	"go.mau.fi/whatsmeow"
)

var csvUpdateLocks sync.Map

func csvUpdateLockForPath(path string) *sync.Mutex {
	if path == "" {
		m := &sync.Mutex{}
		return m
	}
	if v, ok := csvUpdateLocks.Load(path); ok {
		return v.(*sync.Mutex)
	}
	m := &sync.Mutex{}
	actual, _ := csvUpdateLocks.LoadOrStore(path, m)
	return actual.(*sync.Mutex)
}

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

	http.HandleFunc("/api/v2/chaitra-navratri",
		apiKeyMiddleware(bundle.SevaHandler.HandleSendSeva(domain.SevaTypeChaitraNavratri)))
	log.Println("🚦 Registered: POST /api/v2/chaitra-navratri")

	http.HandleFunc("/api/v2/ekadashi-bhagavat-seva/update/adhay",
		apiKeyMiddleware(handleUpdateAdhyayCSV(bundle, domain.SevaTypeEkadashiBhagavat)))
	log.Println("🚦 Registered: POST /api/v2/ekadashi-bhagavat-seva/update/adhay")

	http.HandleFunc("/api/v2/durga-paath/update/adhay",
		apiKeyMiddleware(handleUpdateAdhyayCSV(bundle, domain.SevaTypeDurgaPaath)))
	log.Println("🚦 Registered: POST /api/v2/durga-paath/update/adhay")

	http.HandleFunc("/api/v2/saptahik-swami-seva/update/adhay",
		apiKeyMiddleware(handleUpdateAdhyayCSV(bundle, domain.SevaTypeSaptahikSwami)))
	log.Println("🚦 Registered: POST /api/v2/saptahik-swami-seva/update/adhay")

	http.HandleFunc("/api/v2/malhari/update/adhay",
		apiKeyMiddleware(handleUpdateAdhyayCSV(bundle, domain.SevaTypeMalhari)))
	log.Println("🚦 Registered: POST /api/v2/malhari/update/adhay")

	http.HandleFunc("/api/v2/darbar/update/adhay",
		apiKeyMiddleware(handleUpdateAdhyayCSV(bundle, domain.SevaTypeDarbar)))
	log.Println("🚦 Registered: POST /api/v2/darbar/update/adhay")

	http.HandleFunc("/api/v2/chaitra-navratri/update/adhay",
		apiKeyMiddleware(handleUpdateAdhyayCSV(bundle, domain.SevaTypeChaitraNavratri)))
	log.Println("🚦 Registered: POST /api/v2/chaitra-navratri/update/adhay")

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

	http.HandleFunc("/api/v2/chaitra-navratri/send-reminders",
		apiKeyMiddleware(bundle.ReminderHandler.HandleReminders(domain.SevaTypeChaitraNavratri)))
	log.Println("🚦 Registered: POST /api/v2/chaitra-navratri/send-reminders")

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

	http.HandleFunc("/api/v2/chaitra-navratri/group-announcement",
		apiKeyMiddleware(bundle.ReminderHandler.HandleGroupAnnouncement(domain.SevaTypeChaitraNavratri)))
	log.Println("🚦 Registered: POST /api/v2/chaitra-navratri/group-announcement")

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

func handleUpdateAdhyayCSV(bundle *ComponentsBundle, sevaType domain.SevaType) http.HandlerFunc {
	type request struct {
		GroupNo  int    `json:"group_no"`
		GroupNo2 int    `json:"groupNo"`
		Op       string `json:"op"`
		Op2      string `json:"operation"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		groupNo := req.GroupNo
		if groupNo == 0 {
			groupNo = req.GroupNo2
		}
		op := strings.TrimSpace(req.Op)
		if op == "" {
			op = strings.TrimSpace(req.Op2)
		}
		op = strings.ToUpper(op)

		if groupNo <= 0 {
			http.Error(w, "valid group_no is required", http.StatusBadRequest)
			return
		}
		if op != "INCRE" && op != "DCRE" {
			http.Error(w, "op must be INCRE or DCRE", http.StatusBadRequest)
			return
		}

		gl, ok := bundle.Config.Groups[string(sevaType)]
		if !ok {
			http.Error(w, "seva_type not found", http.StatusNotFound)
			return
		}

		var csvPath string
		maxAdhyas := 0
		found := false
		for _, g := range gl {
			if g.Number == groupNo {
				csvPath = strings.TrimSpace(g.CSVPath)
				maxAdhyas = g.MaxAdhyas
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "group not found", http.StatusNotFound)
			return
		}
		if csvPath == "" {
			http.Error(w, "group csv_path is empty", http.StatusInternalServerError)
			return
		}
		if maxAdhyas <= 0 {
			maxAdhyas = 0
		}

		m := csvUpdateLockForPath(csvPath)
		m.Lock()
		defer m.Unlock()

		csvRepo := repository.NewCSVRepository()
		members, err := csvRepo.ReadMembers(csvPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read csv: %v", err), http.StatusInternalServerError)
			return
		}
		if len(members) == 0 {
			http.Error(w, "no members found in csv", http.StatusBadRequest)
			return
		}

		updated := make([]domain.Member, len(members))
		for i, member := range members {
			newAdhyay := member.AdhyayNo
			switch op {
			case "INCRE":
				newAdhyay = newAdhyay + 1
				if maxAdhyas > 0 && newAdhyay > maxAdhyas {
					newAdhyay = 1
				}
			case "DCRE":
				newAdhyay = newAdhyay - 1
				if maxAdhyas > 0 && newAdhyay < 1 {
					newAdhyay = maxAdhyas
				}
			}
			updated[i] = member
			updated[i].AdhyayNo = newAdhyay
		}

		if err := csvRepo.WriteMembers(csvPath, updated, groupNo); err != nil {
			http.Error(w, fmt.Sprintf("failed to write csv: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"success":   true,
			"seva_type": string(sevaType),
			"group_no":  groupNo,
			"op":        op,
			"members":   len(updated),
		})
	}
}
