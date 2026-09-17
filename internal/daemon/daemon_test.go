package daemon_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multi-agent-team/internal/daemon"
)

func stateDir(t *testing.T) string {
	t.Helper()
	path, err := os.MkdirTemp("/tmp", "team-daemon-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(path) })
	return path
}

func TestServiceOwnsSocketAndWaitsForGracefulShutdown(t *testing.T) {
	dir := stateDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- daemon.Run(ctx, dir, func(path string) { ready <- path }) }()
	var socket string
	select {
	case socket = <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("not ready")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	response, err := client.Get("http://local/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("bad health: %s, %v", body, err)
	}
	for _, name := range []string{"teamd.sock", "teamd.lock", "state.db"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("private resource %s: %v", name, err)
		}
	}
	if err := daemon.Run(ctx, dir, nil); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("duplicate service: %v", err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("shutdown failed to quiesce")
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket survives graceful shutdown: %v", err)
	}
}

func TestStartupRejectsUnsafeStateAndPreservesUnownedFiles(t *testing.T) {
	for _, name := range []string{"teamd.sock", "teamd.lock", "state.db"} {
		t.Run(name, func(t *testing.T) {
			dir := stateDir(t)
			path := filepath.Join(dir, name)
			if err := os.WriteFile(path, []byte("must survive"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := daemon.Run(context.Background(), dir, nil); err == nil {
				t.Fatal("unsafe file accepted")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "must survive" {
				t.Fatal("unowned file modified")
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		dir := stateDir(t)
		destination := filepath.Join(dir, "outside")
		if err := os.WriteFile(destination, []byte("private"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(destination, filepath.Join(dir, "state.db")); err != nil {
			t.Fatal(err)
		}
		if err := daemon.Run(context.Background(), dir, nil); err == nil {
			t.Fatal("symlink accepted")
		}
		got, _ := os.ReadFile(destination)
		if string(got) != "private" {
			t.Fatal("followed symlink")
		}
	})
	t.Run("shared directory", func(t *testing.T) {
		dir := stateDir(t)
		if err := os.Chmod(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := daemon.Run(context.Background(), dir, nil); err == nil {
			t.Fatal("shared directory accepted")
		}
	})
	t.Run("relative", func(t *testing.T) {
		if err := daemon.Run(context.Background(), "relative", nil); err == nil {
			t.Fatal("relative directory accepted")
		}
	})
	t.Run("file as directory", func(t *testing.T) {
		path := filepath.Join(stateDir(t), "file")
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if err := daemon.Run(context.Background(), path, nil); err == nil {
			t.Fatal("file accepted")
		}
	})
	t.Run("invalid database", func(t *testing.T) {
		dir := stateDir(t)
		if err := os.WriteFile(filepath.Join(dir, "state.db"), []byte("not a database"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := daemon.Run(context.Background(), dir, nil); err == nil {
			t.Fatal("corrupt database accepted")
		}
	})
	t.Run("socket path too long", func(t *testing.T) {
		dir := filepath.Join(stateDir(t), strings.Repeat("a", 95))
		if err := daemon.Run(context.Background(), dir, nil); err == nil {
			t.Fatal("unusable socket accepted")
		}
	})
}

func TestStaleSocketCanBeRecovered(t *testing.T) {
	dir := stateDir(t)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "teamd.sock"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	l.SetUnlinkOnClose(false)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := daemon.Run(ctx, dir, func(string) { cancel() }); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultStateDirectoryIsPrivateApplicationNamespace(t *testing.T) {
	path, err := daemon.DefaultStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) || filepath.Base(path) != "multi-agent-team" {
		t.Fatalf("unexpected path %s", path)
	}
}
