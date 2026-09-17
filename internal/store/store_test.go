package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"multi-agent-team/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestRollbackAndEventPagination(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	path := t.TempDir()
	p, err := s.CreateProject(ctx, "project", "Project", path)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreateProject(ctx, "project", "Project", path)
	if err != nil || again != p {
		t.Fatalf("retry: %+v %v", again, err)
	}
	if _, err := s.CreateProject(ctx, "duplicate", "Other", path); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate path: %v", err)
	}
	if _, err := s.CreateGoal(ctx, "recover", "missing", "Goal", ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.CreateGoal(ctx, "recover", p.ID, "Goal", ""); err != nil {
		t.Fatalf("rejected request was retained: %v", err)
	}
	for i := 0; i < 105; i++ {
		id := fmt.Sprintf("page-%d", i)
		if _, err := s.CreateGoal(ctx, id, p.ID, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.Events(ctx, 0)
	if err != nil || len(first) != 100 {
		t.Fatalf("first page %d %v", len(first), err)
	}
	second, err := s.Events(ctx, first[len(first)-1].Sequence)
	if err != nil || len(second) != 7 {
		t.Fatalf("second page %d %v", len(second), err)
	}
	if second[0].Sequence <= first[len(first)-1].Sequence {
		t.Fatal("overlapping event pages")
	}
	if _, err := s.Events(ctx, -1); !errors.Is(err, store.ErrInvalid) {
		t.Fatal("negative cursor accepted")
	}
}

func TestUnsupportedSchemaAndClosedDatabaseFailExplicitly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version=999"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := store.Open(path); err == nil {
		s.Close()
		t.Fatal("future schema accepted")
	}
	if s, err := store.Open(filepath.Join(path, "invalid")); err == nil {
		s.Close()
		t.Fatal("file parent accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.db")
	os.WriteFile(bad, []byte("corrupt"), 0600)
	if s, err := store.Open(bad); err == nil {
		s.Close()
		t.Fatal("corrupt database accepted")
	}
	s := openStore(t)
	s.Close()
	ctx := context.Background()
	if _, err := s.Snapshot(ctx); err == nil {
		t.Fatal("closed store snapshot succeeded")
	}
	if _, err := s.Events(ctx, 0); err == nil {
		t.Fatal("closed store events succeeded")
	}
	if _, err := s.CreateGoal(ctx, "id", "project", "Goal", ""); err == nil {
		t.Fatal("closed store write succeeded")
	}
}

func TestTeamIdentityIsPersistentAndIndependentOfEngine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	var first []store.Member
	for i := 0; i < 2; i++ {
		s, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		snap, err := s.Snapshot(context.Background())
		_ = s.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(snap.Members) != 6 {
			t.Fatalf("expected six persistent members, got %d", len(snap.Members))
		}
		if snap.ProtocolVersion != 1 {
			t.Fatalf("protocol version = %d", snap.ProtocolVersion)
		}
		if i == 0 {
			first = snap.Members
		} else {
			for j := range first {
				if first[j] != snap.Members[j] {
					t.Fatal("member identity changed on reopen")
				}
			}
		}
	}
}

func TestGoalSurvivesReopenWithoutClaimingExecution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "register-project", "nano-harness", t.TempDir())
	if err != nil {
		_ = s.Close()
		t.Fatal(err)
	}
	g, err := s.CreateGoal(ctx, "submit-goal", p.ID, "外部控制接口", "提交、中断和恢复会话")
	if err != nil {
		_ = s.Close()
		t.Fatal(err)
	}
	if g.State != "awaiting_engine" || g.Version != 1 {
		t.Fatalf("unexpected goal: %+v", g)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	snap, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Goals) != 1 || snap.Goals[0] != g {
		t.Fatalf("goal lost or altered: %+v", snap.Goals)
	}
	events, err := s.Events(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EntityID != p.ID || events[1].EntityID != g.ID || events[1].Sequence != snap.Cursor {
		t.Fatalf("inconsistent event cursor: %+v / %+v", events, snap)
	}
	tail, err := s.Events(ctx, snap.Cursor)
	if err != nil || len(tail) != 0 {
		t.Fatalf("events after cursor = %+v, %v", tail, err)
	}
}

func TestIdempotencyPreventsDuplicateGoalsUnderConcurrentRetries(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "project", "Project", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	goals := make(chan store.Goal, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g, err := s.CreateGoal(ctx, "same-request", p.ID, "Goal", "Description")
			if err != nil {
				t.Error(err)
				return
			}
			goals <- g
		}()
	}
	wg.Wait()
	close(goals)
	var id string
	for g := range goals {
		if id != "" && id != g.ID {
			t.Fatal("duplicate goal created")
		}
		id = g.ID
	}
	snap, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Goals) != 1 || snap.Cursor != 2 {
		t.Fatalf("retry changed state: %+v", snap)
	}
	_, err = s.CreateGoal(ctx, "same-request", p.ID, "Different goal", "Description")
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("wanted request conflict, got %v", err)
	}
	snap, err = s.Snapshot(ctx)
	if err != nil || len(snap.Goals) != 1 || snap.Cursor != 2 {
		t.Fatal("rejected request changed state")
	}
}

func TestInvalidInputsAndMissingProjectDoNotProduceEvents(t *testing.T) {
	s := openStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "", "Valid", t.TempDir()); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("empty id: %v", err)
	}
	if _, err := s.CreateProject(ctx, "id", " ", t.TempDir()); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("empty name: %v", err)
	}
	if _, err := s.CreateProject(ctx, "id", "Valid", "/path-that-does-not-exist"); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("missing path: %v", err)
	}
	if _, err := s.CreateGoal(ctx, "id", "missing", "Goal", ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing project: %v", err)
	}
	snap, err := s.Snapshot(ctx)
	if err != nil || snap.Cursor != 0 {
		t.Fatalf("rejected input persisted: %+v / %v", snap, err)
	}
}
