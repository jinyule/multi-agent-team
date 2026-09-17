package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealBinarySurvivesClientDisconnectAndProcessRestart(t *testing.T) {
	state, err := os.MkdirTemp("/tmp", "team-e2e-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(state)
	binary := os.Getenv("TEAM_E2E_DAEMON")
	if binary == "" {
		binary = filepath.Join(t.TempDir(), "teamd")
		build := exec.Command("go", "build", "-o", binary, "../../cmd/teamd")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build failed: %v %s", err, out)
		}
	}
	socket := filepath.Join(state, "teamd.sock")
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	request := func(method, path string, payload any) (int, []byte) {
		t.Helper()
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		r, err := http.NewRequest(method, "http://local"+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, out
	}
	start := func() *exec.Cmd {
		t.Helper()
		cmd := exec.Command(binary, "--state-dir", state)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			r, err := client.Get("http://local/healthz")
			if err == nil {
				r.Body.Close()
				if r.StatusCode == 200 {
					return cmd
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("service did not expose a healthy Unix socket")
		return nil
	}
	cmd := start()
	status, data := request("POST", "/v1/projects", map[string]string{"request_id": "project", "name": "Test project", "path": state})
	if status != 201 {
		t.Fatalf("project: %d %s", status, data)
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	goal := map[string]string{"request_id": "goal", "title": "Persistent goal", "description": "Survive process restart"}
	status, original := request("POST", "/v1/projects/"+p.ID+"/goals", goal)
	if status != 201 {
		t.Fatalf("goal: %d %s", status, original)
	}
	transport.CloseIdleConnections()
	if status, _ := request("GET", "/v1/snapshot", nil); status != 200 {
		t.Fatal("client disconnect stopped service")
	}
	second := exec.Command(binary, "--state-dir", state)
	if out, err := second.CombinedOutput(); err == nil || !strings.Contains(string(out), "already running") {
		t.Fatalf("second instance was not rejected: %v %s", err, out)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	transport.CloseIdleConnections()
	cmd = start()
	status, repeated := request("POST", "/v1/projects/"+p.ID+"/goals", goal)
	if status != 201 || !bytes.Equal(original, repeated) {
		t.Fatalf("restart lost idempotency: %d %s", status, repeated)
	}
	_, data = request("GET", "/v1/snapshot", nil)
	var snap struct {
		Goals  []json.RawMessage `json:"goals"`
		Cursor int               `json:"cursor"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Goals) != 1 || snap.Cursor != 2 {
		t.Fatalf("recovery duplicated data: %s", data)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unclean shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
