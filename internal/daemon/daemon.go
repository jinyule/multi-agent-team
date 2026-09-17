// Package daemon owns the single local service process and its resources.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"multi-agent-team/internal/api"
	"multi-agent-team/internal/store"
)

func DefaultStateDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "multi-agent-team"), nil
}

func Run(ctx context.Context, stateDir string, ready func(string)) error {
	if !filepath.IsAbs(stateDir) {
		return errors.New("state directory must be absolute")
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(stateDir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("state directory must be a private directory with mode 0700")
	}
	lockPath := filepath.Join(stateDir, "teamd.lock")
	if err := privateFile(lockPath); err != nil {
		return err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("service already running or state directory cannot be locked: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	socket := filepath.Join(stateDir, "teamd.sock")
	if old, err := os.Lstat(socket); err == nil {
		if old.Mode()&os.ModeSocket == 0 {
			return errors.New("socket path exists and is not a socket")
		}
		if err := os.Remove(socket); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dbPath := filepath.Join(stateDir, "state.db")
	if err := privateFile(dbPath); err != nil {
		return err
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0600); err != nil {
		return err
	}
	server := &http.Server{Handler: api.New(s), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	if ready != nil {
		ready(socket)
	}
	select {
	case err := <-finished:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-finished
			return err
		}
		err := <-finished
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func privateFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("%s must be a private regular file", filepath.Base(path))
	}
	return nil
}
