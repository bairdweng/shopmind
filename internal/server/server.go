package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"shopmind/internal/agent"
	"shopmind/internal/config"
	"shopmind/internal/vault"
)

type Server struct {
	mux    *http.ServeMux
	static string
	store  *vault.Store
}

func New(staticDir string, store *vault.Store) *Server {
	s := &Server{mux: http.NewServeMux(), static: staticDir, store: store}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.mux.ServeHTTP(w, r)
			return
		}
		s.serveStatic(w, r)
	})
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/vault/init", s.handleVaultInit)
	s.mux.HandleFunc("/api/vault/unlock", s.handleVaultUnlock)
	s.mux.HandleFunc("/api/settings", s.requireUnlock(s.handleSettings))
	s.mux.HandleFunc("/api/setup/cursor", s.requireUnlock(s.handleSetupCursor))
}

func (s *Server) requireUnlock(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.store.Unlocked() {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未解锁"})
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"service":     "shopmind",
		"initialized": s.store.Initialized(),
		"unlocked":    s.store.Unlocked(),
		"dataDir":     config.DataDir(),
	})
}

func (s *Server) handleVaultInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.Init(req.Password); err != nil {
		writeVaultErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "unlocked": true})
}

func (s *Server) handleVaultUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.store.Unlock(req.Password); err != nil {
		writeVaultErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "unlocked": true})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pub, err := s.store.Public()
		if err != nil {
			writeVaultErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, pub)
	case http.MethodPost:
		var req struct {
			CursorAPIKey   string `json:"cursor_api_key"`
			CursorAgentBin string `json:"cursor_agent_bin"`
			UpdateAPIKey   bool   `json:"update_api_key"`
			ClearAPIKey    bool   `json:"clear_api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		pub, err := s.store.Apply(vault.Patch{
			CursorAPIKey:   req.CursorAPIKey,
			CursorAgentBin: req.CursorAgentBin,
			UpdateAPIKey:   req.UpdateAPIKey,
			ClearAPIKey:    req.ClearAPIKey,
		})
		if err != nil {
			writeVaultErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":       true,
			"settings": pub,
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleSetupCursor(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	switch r.Method {
	case http.MethodGet:
		if action != "" && action != "status" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
			return
		}
		p, err := s.store.Payload()
		if err != nil {
			writeVaultErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, agent.CheckEnv(p))
	case http.MethodPost:
		switch action {
		case "install":
			out, err := agent.InstallCLI(r.Context())
			p, _ := s.store.Payload()
			st := agent.CheckEnv(p)
			if err != nil {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"ok":     false,
					"output": out,
					"error":  err.Error(),
					"status": st,
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":     true,
				"output": out,
				"status": st,
			})
		case "test":
			p, err := s.store.Payload()
			if err != nil {
				writeVaultErr(w, err)
				return
			}
			out, err := agent.TestConnection(r.Context(), p)
			if err != nil {
				writeJSON(w, http.StatusOK, map[string]interface{}{
					"ok":     false,
					"output": out,
					"error":  err.Error(),
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":     true,
				"output": out,
			})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action required: install | test"})
		}
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if s.static == "" {
		http.NotFound(w, r)
		return
	}
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	full := filepath.Join(s.static, filepath.Clean(path))
	if !strings.HasPrefix(full, s.static) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if _, err := os.Stat(full); os.IsNotExist(err) {
		full = filepath.Join(s.static, "index.html")
	}
	http.ServeFile(w, r, full)
}

func writeVaultErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vault.ErrLocked):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	case errors.Is(err, vault.ErrBadPassword), errors.Is(err, vault.ErrEmptyPassword),
		errors.Is(err, vault.ErrAlreadyInit), errors.Is(err, vault.ErrNotInitialized):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
