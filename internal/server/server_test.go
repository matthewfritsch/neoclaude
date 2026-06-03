package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
)

type stubProvider struct{}

func (s stubProvider) Sessions() []SessionInfo {
	return []SessionInfo{
		{ID: 1, Name: "test-buf", Cwd: "/tmp", Status: "idle", Pid: 1234},
	}
}

func (s stubProvider) Status() StatusInfo {
	return StatusInfo{ActiveBuffer: 0, TotalBuffers: 1, Uptime: "5s"}
}

func TestServerSessionsEndpoint(t *testing.T) {
	srv, err := New(stubProvider{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Start()
	defer srv.Stop()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", srv.sockPath)
			},
		},
	}

	resp, err := client.Get("http://unix/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}

	var sessions []SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "test-buf" {
		t.Fatalf("unexpected: %+v", sessions)
	}
}

func TestServerStatusEndpoint(t *testing.T) {
	srv, err := New(stubProvider{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Start()
	defer srv.Stop()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", srv.sockPath)
			},
		},
	}

	resp, err := client.Get("http://unix/api/status")
	if err != nil {
		t.Fatalf("GET /api/status: %v", err)
	}
	defer resp.Body.Close()

	var info StatusInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.TotalBuffers != 1 {
		t.Fatalf("unexpected: %+v", info)
	}
}
