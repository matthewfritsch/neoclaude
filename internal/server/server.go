package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SessionInfo is the JSON shape returned by the sessions API.
type SessionInfo struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Cwd       string `json:"cwd"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Pid       int    `json:"pid"`
}

// StatusInfo is the JSON shape for the top-level status endpoint.
type StatusInfo struct {
	ActiveBuffer int    `json:"active_buffer"`
	TotalBuffers int    `json:"total_buffers"`
	Uptime       string `json:"uptime"`
}

// Provider supplies session and status data to the server.
type Provider interface {
	Sessions() []SessionInfo
	Status() StatusInfo
}

// Server is an HTTP server on a Unix domain socket.
type Server struct {
	listener net.Listener
	srv      *http.Server
	sockPath string
}

// SocketPath returns the well-known socket location.
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "neoclaude.sock")
	}
	return "/tmp/neoclaude.sock"
}

// New creates a server bound to the Unix socket. Call Start() to begin serving.
func New(provider Provider) (*Server, error) {
	path := SocketPath()
	os.Remove(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	s := &Server{
		listener: ln,
		sockPath: path,
		srv:      &http.Server{Handler: mux},
	}

	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, provider.Sessions())
	})

	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		for _, s := range provider.Sessions() {
			if s.ID == id {
				writeJSON(w, s)
				return
			}
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, provider.Status())
	})

	return s, nil
}

// Start begins serving in a background goroutine.
func (s *Server) Start() {
	go s.srv.Serve(s.listener)
}

// Stop shuts down the server and removes the socket file.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
	os.Remove(s.sockPath)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
