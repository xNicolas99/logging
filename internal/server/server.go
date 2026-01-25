package server

import (
	"bufio"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"

	"github.com/jules/http-monitor/internal/config"
	"github.com/jules/http-monitor/internal/monitor"
	"github.com/jules/http-monitor/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	cfg     *config.Config
	storage storage.Storage
	monitor *monitor.Monitor
	logPath string
}

func NewServer(cfg *config.Config, s storage.Storage, m *monitor.Monitor, logPath string) *Server {
	return &Server{
		cfg:     cfg,
		storage: s,
		monitor: m,
		logPath: logPath,
	}
}

func (s *Server) Start(port int) error {
	http.HandleFunc("/api/targets", s.handleTargets)
	http.HandleFunc("/api/measurements", s.handleMeasurements)
	http.HandleFunc("/api/run", s.handleRunNow)
	http.HandleFunc("/api/config/interval", s.handleInterval)
	http.HandleFunc("/logs", s.handleLogs)

	// Serve static files
	fsys, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	http.Handle("/", http.FileServer(http.FS(fsys)))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Starting web server on %s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.URL == "" {
			http.Error(w, "Name and URL required", http.StatusBadRequest)
			return
		}
		if err := s.monitor.AddTarget(req.Name, req.URL); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.cfg.Targets)
}

func (s *Server) handleMeasurements(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	measurements, err := s.storage.GetMeasurements(target, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measurements)
}

func (s *Server) handleRunNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.monitor.RunNow()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Measurements triggered"))
}

func (s *Server) handleInterval(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		// Return interval in minutes
		json.NewEncoder(w).Encode(map[string]int{"interval": s.cfg.Interval / 60})
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Interval int `json:"interval"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.monitor.SetInterval(req.Interval); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open(s.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Log file not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Simple tail implementation: read last 100 lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	start := 0
	if len(lines) > 100 {
		start = len(lines) - 100
	}

	w.Header().Set("Content-Type", "text/plain")
	for i := start; i < len(lines); i++ {
		fmt.Fprintln(w, lines[i])
	}
}
