// Package bridge exposes the entire Go engine to the Tauri + React frontend
// over a local-only HTTP JSON API (127.0.0.1, random high port passed to
// the frontend via stdout on boot). This is the seam the product brief
// calls out explicitly: TypeScript only ever talks to this API — every
// byte of actual data processing happens on the Go side of this boundary.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"mosaic/internal/cache"
	"mosaic/internal/export"
	"mosaic/internal/expression"
	"mosaic/internal/parser"
	"mosaic/internal/pipeline"
	"mosaic/internal/quality"
	"mosaic/internal/schema"
	"mosaic/internal/scheduler"
	"mosaic/internal/security"
	"mosaic/internal/storage"
	"mosaic/internal/transform"
)

// Server wires every backend package into a set of HTTP handlers.
type Server struct {
	mux     *http.ServeMux
	jobs    *pipeline.Engine
	store   *storage.Store
	vault   *security.Vault
	cache   *cache.Cache
	schedMu sync.Mutex
	schedules map[string]scheduler.Schedule
	history   map[string][]scheduler.RunRecord
}

// New builds a Server with the given storage root and vault passphrase
// (the Rust shell supplies the passphrase from the OS keychain).
func New(storageRoot, vaultPassphrase string) (*Server, error) {
	store, err := storage.NewStore(storageRoot)
	if err != nil {
		return nil, err
	}
	s := &Server{
		mux:       http.NewServeMux(),
		jobs:      pipeline.NewEngine(),
		store:     store,
		vault:     security.NewVault(vaultPassphrase),
		cache:     cache.New(),
		schedules: map[string]scheduler.Schedule{},
		history:   map[string][]scheduler.RunRecord{},
	}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/formats", s.handleFormats)
	s.mux.HandleFunc("/api/nodes", s.handleNodeTypes)
	s.mux.HandleFunc("/api/import", s.handleImport)
	s.mux.HandleFunc("/api/expression/validate", s.handleValidateExpression)
	s.mux.HandleFunc("/api/pipeline/run", s.handleRunPipeline)
	s.mux.HandleFunc("/api/jobs/", s.handleJobRoute)
	s.mux.HandleFunc("/api/export", s.handleExport)
	s.mux.HandleFunc("/api/quality/score", s.handleQualityScore)
	s.mux.HandleFunc("/api/projects", s.handleProjects)
	s.mux.HandleFunc("/api/projects/", s.handleProjectByID)
	s.mux.HandleFunc("/api/scheduler/", s.handleScheduler)
	s.mux.HandleFunc("/api/vault/", s.handleVault)
}

// Handler returns the CORS-wrapped mux, ready to pass to http.ListenAndServe.
func (s *Server) Handler() http.Handler { return withCORS(s.mux) }

// withCORS allows only the local Tauri webview origin (tauri://localhost /
// http://localhost:*) to call the API — this server never listens on a
// non-loopback interface.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || strings.Contains(origin, "localhost") || strings.HasPrefix(origin, "tauri://") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "engine": "mosaic-go"})
}

func (s *Server) handleFormats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, parser.List())
}

func (s *Server) handleNodeTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, transform.List())
}

// ---- Import / Profiling --------------------------------------------------

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("use POST"))
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	format := r.FormValue("format")
	head := make([]byte, 4096)
	n, _ := file.Read(head)
	head = head[:n]
	file.Seek(0, 0)

	var p parser.Parser
	if format != "" {
		p, err = parser.Get(format)
	} else {
		var score float64
		p, score = parser.Detect(header.Filename, head)
		if p == nil || score == 0 {
			writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("could not detect a format for %q", header.Filename))
			return
		}
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	opt := parser.Options{HasHeader: true, SampleLimit: 5000}
	res, err := p.Parse(file, opt)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	report := schema.Profile(res.Columns, res.Rows, 200)
	writeJSON(w, http.StatusOK, map[string]any{
		"format": p.Name(),
		"report": report,
	})
}

// ---- Expression validation ------------------------------------------------

func (s *Server) handleValidateExpression(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := expression.Compile(body.Expression); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

// ---- Pipeline execution & Job Engine --------------------------------------

func (s *Server) handleRunPipeline(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Definition pipeline.Definition          `json:"definition"`
		Sources    map[string][]schema.Row      `json:"sources"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	jobID := fmt.Sprintf("job_%d", time.Now().UnixNano())
	_, err := s.jobs.Submit(jobID, body.Definition.Name, &body.Definition, pipeline.Sources(body.Sources),
		nil, func(job *pipeline.Job, results map[string][]schema.Row) {
			s.recordHistory(body.Definition.ID, job)
		})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": jobID})
}

func (s *Server) recordHistory(pipelineID string, job *pipeline.Job) {
	s.schedMu.Lock()
	defer s.schedMu.Unlock()
	s.history[pipelineID] = append(s.history[pipelineID], scheduler.RunRecord{
		StartedAt: job.StartedAt,
		Duration:  job.FinishedAt.Sub(job.StartedAt),
		RowCount:  job.RowsProcessed,
	})
}

func (s *Server) handleJobRoute(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	job, ok := s.jobs.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("job %q not found", id))
		return
	}
	if len(parts) == 1 {
		writeJSON(w, http.StatusOK, job)
		return
	}
	switch parts[1] {
	case "pause":
		job.Pause()
	case "resume":
		job.Resume()
	case "cancel":
		job.Cancel()
	case "stream":
		s.streamJob(w, r, job)
		return
	default:
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown job action %q", parts[1]))
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// streamJob implements Server-Sent Events so the Job Engine panel gets live
// progress (rows/sec, memory) without polling.
func (s *Server) streamJob(w http.ResponseWriter, r *http.Request, job *pipeline.Job) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ctx := r.Context()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data, _ := json.Marshal(job)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			if job.Status == pipeline.StatusCompleted || job.Status == pipeline.StatusFailed || job.Status == pipeline.StatusCancelled {
				return
			}
		}
	}
}

// ---- Export Studio ---------------------------------------------------

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Format  string          `json:"format"`
		Columns []string        `json:"columns"`
		Rows    []schema.Row    `json:"rows"`
		Options export.Options  `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writer, ok := export.Registry[body.Format]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown export format %q", body.Format))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="export.%s"`, body.Format))
	if err := writer(w, body.Columns, body.Rows, body.Options); err != nil {
		log.Printf("export error: %v", err)
	}
}

// ---- Data Quality Score ------------------------------------------------

func (s *Server) handleQualityScore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Schema  schema.Schema   `json:"schema"`
		Rows    []schema.Row    `json:"rows"`
		Weights *quality.Weights `json:"weights"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w2 := quality.DefaultWeights()
	if body.Weights != nil {
		w2 = *body.Weights
	}
	writeJSON(w, http.StatusOK, quality.Score(body.Schema, body.Rows, w2))
}

// ---- Projects --------------------------------------------------------

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.store.List()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var p storage.Project
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.store.Save(&p); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("unsupported method"))
	}
}

func (s *Server) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	p, err := s.store.Load(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// ---- Scheduler (user-defined operating hours) -----------------------------

func (s *Server) handleScheduler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/scheduler/")

	switch r.Method {
	case http.MethodPost:
		var sched scheduler.Schedule
		if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		s.schedMu.Lock()
		s.schedules[sched.PipelineID] = sched
		s.schedMu.Unlock()
		writeJSON(w, http.StatusOK, sched)
	case http.MethodGet:
		s.schedMu.Lock()
		sched, ok := s.schedules[id]
		history := s.history[id]
		s.schedMu.Unlock()
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Errorf("no schedule configured for pipeline %q", id))
			return
		}
		status, err := scheduler.Evaluate(sched, history, time.Now())
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("unsupported method"))
	}
}

// ---- Secrets Vault -----------------------------------------------------

func (s *Server) handleVault(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/vault/")
	switch r.Method {
	case http.MethodPost:
		var body struct{ Value string `json:"value"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := s.vault.Set(key, body.Value); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "stored"})
	case http.MethodDelete:
		s.vault.Delete(key)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("unsupported method"))
	}
}

var _ = context.Background
