package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"shopmind/internal/agent"
	"shopmind/internal/clip"
	"shopmind/internal/config"
	"shopmind/internal/gitsync"
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
	s.mux.HandleFunc("/api/git", s.requireUnlock(s.handleGit))
	s.mux.HandleFunc("/api/clip/env", s.requireUnlock(s.handleClipEnv))
	s.mux.HandleFunc("/api/clip/text2srt", s.requireUnlock(s.handleClipText2SRT))
}

func (s *Server) handleClipEnv(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, clip.CheckEnv())
}

func (s *Server) handleClipText2SRT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "解析上传失败: " + err.Error()})
		return
	}
	file, _, err := r.FormFile("audio")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少音频文件"})
		return
	}
	defer file.Close()

	suffix := ".mp3"
	if fh := r.MultipartForm.File["audio"]; len(fh) > 0 && filepath.Ext(fh[0].Filename) != "" {
		suffix = strings.ToLower(filepath.Ext(fh[0].Filename))
	}
	tmp, err := os.CreateTemp("", "shopmind-audio-*"+suffix)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	audioPath := tmp.Name()
	defer os.Remove(audioPath)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "保存音频失败: " + err.Error()})
		return
	}
	tmp.Close()

	text := strings.TrimSpace(r.FormValue("text"))
	language := strings.TrimSpace(r.FormValue("language"))
	if language == "" {
		language = "zh"
	}

	srt, output, err := clip.TextToSRT(r.Context(), audioPath, text, language)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":     false,
			"error":  err.Error(),
			"output": output,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"srt":    srt,
		"output": output,
	})
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

func (s *Server) handleGit(w http.ResponseWriter, r *http.Request) {
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	switch r.Method {
	case http.MethodGet:
		if action != "" && action != "status" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
			return
		}
		writeJSON(w, http.StatusOK, gitsync.Check(r.Context()))
	case http.MethodPost:
		var (
			out string
			err error
		)
		switch action {
		case "init":
			out, err = gitsync.InitRepo(r.Context())
		case "set_remote":
			var req struct {
				URL string `json:"url"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			out, err = gitsync.SetRemote(r.Context(), req.URL)
		case "push":
			out, err = gitsync.PushForce(r.Context())
		case "overwrite":
			out, err = gitsync.OverwriteLocal(r.Context())
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action required: init | set_remote | push | overwrite"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"ok":     false,
				"output": out,
				"error":  err.Error(),
				"status": gitsync.Check(r.Context()),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok":     true,
			"output": out,
			"status": gitsync.Check(r.Context()),
		})
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
