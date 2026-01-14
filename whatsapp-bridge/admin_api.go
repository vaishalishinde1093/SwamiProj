package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"whatsapp-client/config"
	"whatsapp-client/domain"
)

const adminConfigPath = "config/groups.yaml"

type adminAPI struct {
	bundle *ComponentsBundle

	mu sync.RWMutex // protects config reloads

	fileMu sync.Mutex
	locks  map[string]*sync.Mutex
}

func newAdminAPI(bundle *ComponentsBundle) *adminAPI {
	return &adminAPI{
		bundle: bundle,
		locks:  map[string]*sync.Mutex{},
	}
}

func (a *adminAPI) lockForPath(path string) *sync.Mutex {
	a.fileMu.Lock()
	defer a.fileMu.Unlock()
	if m, ok := a.locks[path]; ok {
		return m
	}
	m := &sync.Mutex{}
	a.locks[path] = m
	return m
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func readFileHash(path string) (string, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return sha256Hex(b), b, nil
}

func parseGroupPath(path string) (sevaType string, groupNo int, tail string, ok bool) {
	prefix := "/api/admin/v1/groups/"
	if !strings.HasPrefix(path, prefix) {
		return "", 0, "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return "", 0, "", false
	}
	parts := strings.Split(rest, "/")
	sevaType = parts[0]
	if len(parts) == 1 {
		return sevaType, 0, "", true
	}
	gn, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, "", false
	}
	groupNo = gn
	if len(parts) >= 3 {
		tail = strings.Join(parts[2:], "/")
	}
	return sevaType, groupNo, tail, true
}

func (a *adminAPI) configHash() (string, error) {
	h, _, err := readFileHash(adminConfigPath)
	if err != nil {
		return "", err
	}
	return h, nil
}

func (a *adminAPI) reloadConfigFromDisk() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	cfg, err := config.LoadConfig(adminConfigPath)
	if err != nil {
		return "", err
	}

	// Update existing config in-place so services holding the pointer see updates.
	*(a.bundle.Config) = *cfg

	return a.configHash()
}

func (a *adminAPI) handleConfig(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.mu.RLock()
	cfg := a.bundle.Config
	a.mu.RUnlock()

	h, err := a.configHash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"hash":   h,
		"config": cfg,
	})
}

func (a *adminAPI) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h, err := a.reloadConfigFromDisk()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to reload config: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"hash":    h,
	})
}

type adminGroup struct {
	SevaType    string `json:"seva_type"`
	Number      int    `json:"number"`
	JID         string `json:"jid"`
	Name        string `json:"name"`
	CSVPath     string `json:"csv_path"`
	MaxAdhyas   int    `json:"max_adhyas"`
	MaxPollSize int    `json:"max_poll_size"`
}

func toAdminGroup(sevaType string, g config.GroupConfig) adminGroup {
	return adminGroup{
		SevaType:    sevaType,
		Number:      g.Number,
		JID:         g.JID,
		Name:        g.Name,
		CSVPath:     g.CSVPath,
		MaxAdhyas:   g.MaxAdhyas,
		MaxPollSize: g.MaxPollSize,
	}
}

func (a *adminAPI) handleGroups(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}

	if r.URL.Path == "/api/admin/v1/groups" || r.URL.Path == "/api/admin/v1/groups/" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		a.mu.RLock()
		groups := a.bundle.Config.Groups
		a.mu.RUnlock()

		var out []adminGroup
		for st, gl := range groups {
			for _, g := range gl {
				out = append(out, toAdminGroup(st, g))
			}
		}

		h, err := a.configHash()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"hash":   h,
			"groups": out,
		})
		return
	}

	sevaType, groupNo, tail, ok := parseGroupPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if tail == "members" {
		a.handleGroupMembers(w, r, sevaType, groupNo)
		return
	}
	if tail != "" {
		http.NotFound(w, r)
		return
	}

	a.mu.RLock()
	groups := a.bundle.Config.Groups
	a.mu.RUnlock()

	gl, ok := groups[sevaType]
	if !ok {
		http.Error(w, "seva_type not found", http.StatusNotFound)
		return
	}

	if groupNo == 0 {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var out []adminGroup
		for _, g := range gl {
			out = append(out, toAdminGroup(sevaType, g))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"groups": out,
		})
		return
	}

	idx := -1
	for i := range gl {
		if gl[i].Number == groupNo {
			idx = i
			break
		}
	}
	if idx == -1 {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, toAdminGroup(sevaType, gl[idx]))
		return
	case http.MethodPut:
		var req adminGroup
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.Number != 0 && req.Number != groupNo {
			http.Error(w, "cannot change group number", http.StatusBadRequest)
			return
		}
		if req.SevaType != "" && req.SevaType != sevaType {
			http.Error(w, "cannot change seva type", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.JID) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.CSVPath) == "" {
			http.Error(w, "jid, name, csv_path are required", http.StatusBadRequest)
			return
		}
		if req.MaxPollSize <= 0 {
			http.Error(w, "max_poll_size must be > 0", http.StatusBadRequest)
			return
		}
		if req.MaxAdhyas <= 0 {
			http.Error(w, "max_adhyas must be > 0", http.StatusBadRequest)
			return
		}

		a.mu.Lock()
		a.bundle.Config.Groups[sevaType][idx] = config.GroupConfig{
			Number:      groupNo,
			JID:         strings.TrimSpace(req.JID),
			Name:        strings.TrimSpace(req.Name),
			CSVPath:     strings.TrimSpace(req.CSVPath),
			MaxAdhyas:   req.MaxAdhyas,
			MaxPollSize: req.MaxPollSize,
		}
		updatedCfg := a.bundle.Config
		a.mu.Unlock()

		if err := persistConfigToDisk(updatedCfg); err != nil {
			http.Error(w, fmt.Sprintf("failed to persist config: %v", err), http.StatusInternalServerError)
			return
		}

		h, err := a.configHash()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"hash":    h,
			"group":   toAdminGroup(sevaType, a.bundle.Config.Groups[sevaType][idx]),
		})
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func persistConfigToDisk(cfg *config.Config) error {
	out, err := marshalYAML(cfg)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(adminConfigPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(adminConfigPath, out, 0644)
}

type groupMembersResponse struct {
	SevaType string         `json:"seva_type"`
	GroupNo  int            `json:"group_no"`
	Version  int64          `json:"version"`
	Members  []domain.Member `json:"members"`
}

type updateGroupMembersRequest struct {
	ExpectedVersion int64        `json:"expected_version"`
	Members      []domain.Member `json:"members"`
}

func (a *adminAPI) handleGroupMembers(w http.ResponseWriter, r *http.Request, sevaType string, groupNo int) {
	if groupNo <= 0 {
		http.Error(w, "valid group number is required", http.StatusBadRequest)
		return
	}

	a.mu.RLock()
	groups := a.bundle.Config.Groups
	a.mu.RUnlock()

	gl, ok := groups[sevaType]
	if !ok {
		http.Error(w, "seva_type not found", http.StatusNotFound)
		return
	}

	var gc *config.GroupConfig
	for i := range gl {
		if gl[i].Number == groupNo {
			gc = &gl[i]
			break
		}
	}
	if gc == nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}

	csvPath := strings.TrimSpace(gc.CSVPath)
	if csvPath == "" {
		http.Error(w, "group csv_path is empty", http.StatusInternalServerError)
		return
	}

	key := fmt.Sprintf("%s:%d", sevaType, groupNo)
	m := a.lockForPath(key)
	m.Lock()
	defer m.Unlock()

	switch r.Method {
	case http.MethodGet:
		members, version, err := a.bundle.MemberStore.GetGroupMembers(domain.SevaType(sevaType), groupNo)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to load members: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, groupMembersResponse{SevaType: sevaType, GroupNo: groupNo, Version: version, Members: members})
		return

	case http.MethodPut:
		var req updateGroupMembersRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		newVersion, err := a.bundle.MemberStore.ReplaceGroupMembers(domain.SevaType(sevaType), groupNo, req.Members, req.ExpectedVersion)
		if err != nil {
			if err.Error() == "conflict" {
				_, currentVersion, verr := a.bundle.MemberStore.GetGroupMembers(domain.SevaType(sevaType), groupNo)
				if verr != nil {
					http.Error(w, fmt.Sprintf("conflict; failed to read current version: %v", verr), http.StatusInternalServerError)
					return
				}
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":            "conflict",
					"current_version":  currentVersion,
					"expected_version": req.ExpectedVersion,
				})
				return
			}
			http.Error(w, fmt.Sprintf("failed to write members: %v", err), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"version": newVersion,
		})
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

type memberGroupRef struct {
	SevaType  string `json:"seva_type"`
	GroupNo   int    `json:"group_no"`
	GroupName string `json:"group_name"`
	AdhyayNo  int    `json:"adhyay_no"`
}

type globalMember struct {
	Key         string           `json:"key"`
	Name        string           `json:"name"`
	PhoneNumber string           `json:"phone_number,omitempty"`
	Groups      []memberGroupRef `json:"groups"`
}

func (a *adminAPI) handleMembersDirectory(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	a.mu.RLock()
	groups := a.bundle.Config.Groups
	a.mu.RUnlock()

	byKey := map[string]*globalMember{}

	for sevaType, gl := range groups {
		for _, g := range gl {
			members, _, err := a.bundle.MemberStore.GetGroupMembers(domain.SevaType(sevaType), g.Number)
			if err != nil {
				continue
			}
			for _, m := range members {
				key := strings.TrimSpace(m.PhoneNumber)
				if key == "" {
					key = "name:" + strings.ToLower(strings.TrimSpace(m.Name))
				} else {
					key = "phone:" + key
				}
				gm, ok := byKey[key]
				if !ok {
					gm = &globalMember{Key: key, Name: strings.TrimSpace(m.Name), PhoneNumber: strings.TrimSpace(m.PhoneNumber)}
					byKey[key] = gm
				}
				gm.Groups = append(gm.Groups, memberGroupRef{SevaType: sevaType, GroupNo: g.Number, GroupName: g.Name, AdhyayNo: m.AdhyayNo})
			}
		}
	}

	var out []globalMember
	for _, v := range byKey {
		out = append(out, *v)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"members": out,
	})
}

func (a *adminAPI) handleHealth(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})
}

func registerAdminHandlers(bundle *ComponentsBundle) {
	api := newAdminAPI(bundle)

	http.HandleFunc("/api/admin/v1/health", apiKeyMiddleware(api.handleHealth))
	http.HandleFunc("/api/admin/v1/config", apiKeyMiddleware(api.handleConfig))
	http.HandleFunc("/api/admin/v1/config/reload", apiKeyMiddleware(api.handleReloadConfig))
	http.HandleFunc("/api/admin/v1/groups", apiKeyMiddleware(api.handleGroups))
	http.HandleFunc("/api/admin/v1/groups/", apiKeyMiddleware(api.handleGroups))
	http.HandleFunc("/api/admin/v1/members", apiKeyMiddleware(api.handleMembersDirectory))
}

var marshalYAML = func(v any) ([]byte, error) {
	return nil, errors.New("yaml marshaller not initialized")
}
