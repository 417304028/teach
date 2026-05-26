package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"hermesclaw/internal/app"
	"hermesclaw/internal/config"
	"hermesclaw/internal/generate"
	"hermesclaw/internal/qq"
	"hermesclaw/internal/rag"
	"hermesclaw/internal/store"
	"hermesclaw/internal/wechat"
)

type Server struct {
	Config    config.Config
	Store     store.Store
	RAG       rag.Service
	App       app.Service
	Generator generate.Service
	QQ        qq.Handler
	WeChat    wechat.ClawbotHandler
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.redirectAdmin)
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/ingest", s.ingest)
	mux.HandleFunc("/api/chat", s.chat)
	mux.HandleFunc("/api/generate/mindmap", s.generateMindmap)
	mux.HandleFunc("/api/generate/ppt", s.generatePPT)
	mux.HandleFunc("/api/generate/exercises", s.generateExercises)
	mux.HandleFunc("/api/generate/outline", s.generateOutline)
	mux.HandleFunc("/api/files/", s.file)
	mux.Handle("/onebot/event", s.QQ)
	mux.Handle("/clawbot/event", s.WeChat)
	mux.HandleFunc("/admin", s.adminDashboard)
	mux.HandleFunc("/admin/materials", s.adminMaterials)
	mux.HandleFunc("/admin/jobs", s.adminJobs)
	mux.HandleFunc("/admin/files", s.adminFiles)
	return withLogging(mux)
}

func (s Server) redirectAdmin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusFound)
}

func (s Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stats": s.Store.Stats()})
}

func (s Server) ingest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	report, err := s.RAG.IngestPath(r.Context(), req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s Server) chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req struct {
		UserID  string `json:"user_id"`
		Channel string `json:"channel"`
		Message string `json:"message"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := s.App.HandleMessage(r.Context(), req.UserID, emptyDefault(req.Channel, "api"), req.Message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s Server) generateMindmap(w http.ResponseWriter, r *http.Request) {
	s.generate(w, r, s.Generator.GenerateMindmap)
}

func (s Server) generatePPT(w http.ResponseWriter, r *http.Request) {
	s.generate(w, r, s.Generator.GeneratePPTX)
}

func (s Server) generateExercises(w http.ResponseWriter, r *http.Request) {
	s.generate(w, r, s.Generator.GenerateExercises)
}

func (s Server) generateOutline(w http.ResponseWriter, r *http.Request) {
	s.generate(w, r, s.Generator.GenerateOutline)
}

func (s Server) generate(w http.ResponseWriter, r *http.Request, fn func(context.Context, generate.Request) (generate.Response, error)) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req generate.Request
	if !decodeJSON(w, r, &req) {
		return
	}
	resp, err := fn(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s Server) file(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/files/")
	file, ok, err := s.Store.GetFile(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !file.Pinned && !file.ExpiresAt.IsZero() && time.Now().After(file.ExpiresAt) {
		http.Error(w, "file expired", http.StatusGone)
		return
	}
	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(file.Name)+`"`)
	http.ServeFile(w, r, file.Path)
}

func (s Server) adminDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	renderAdmin(w, "Dashboard", map[string]any{"Stats": s.Store.Stats()})
}

func (s Server) adminMaterials(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	materials, err := s.Store.ListMaterials()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	renderAdmin(w, "Materials", map[string]any{"Materials": materials})
}

func (s Server) adminJobs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	jobs, err := s.Store.ListJobs(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	renderAdmin(w, "Jobs", map[string]any{"Jobs": jobs})
}

func (s Server) adminFiles(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	files, err := s.Store.ListFiles(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	renderAdmin(w, "Files", map[string]any{"Files": files})
}

func (s Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, password, ok := r.BasicAuth()
	if ok && user == s.Config.AdminUser && password == s.Config.AdminPassword {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="Hermesclaw"`)
	http.Error(w, "admin auth required", http.StatusUnauthorized)
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		_ = start
	})
}
