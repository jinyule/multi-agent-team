package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

var (
	ErrInvalid  = errors.New("invalid_input")
	ErrNotFound = errors.New("not_found")
	ErrConflict = errors.New("request_conflict")
)

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type Goal struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Version     int    `json:"version"`
}

type Event struct {
	Sequence  int64  `json:"sequence"`
	Kind      string `json:"kind"`
	EntityID  string `json:"entity_id"`
	CreatedAt string `json:"created_at"`
}

type Snapshot struct {
	ProtocolVersion int       `json:"protocol_version"`
	Projects        []Project `json:"projects"`
	Members         []Member  `json:"members"`
	Goals           []Goal    `json:"goals"`
	Cursor          int64     `json:"cursor"`
}

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT NOT NULL, path TEXT NOT NULL UNIQUE);
CREATE TABLE members (id TEXT PRIMARY KEY, name TEXT NOT NULL, role TEXT NOT NULL, position INTEGER NOT NULL);
CREATE TABLE goals (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id), title TEXT NOT NULL, description TEXT NOT NULL, state TEXT NOT NULL CHECK(state='awaiting_engine'), version INTEGER NOT NULL CHECK(version>0));
CREATE TABLE requests (id TEXT PRIMARY KEY, digest TEXT NOT NULL, response BLOB NOT NULL);
CREATE TABLE events (sequence INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, entity_id TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')));
INSERT INTO members VALUES
('lead','团队负责人','lead',0), ('product','产品成员','product',1),
('architecture','架构成员','architecture',2), ('development','开发成员','development',3),
('testing','测试成员','testing',4), ('review','评审成员','review',5);
PRAGMA user_version=1;
`

func Open(path string) (*Store, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	if _, err := db.Exec(`PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000;`); err != nil {
		return fail(err)
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if version > 1 {
		return fail(fmt.Errorf("unsupported schema version %d", version))
	}
	if version == 0 {
		tx, err := db.Begin()
		if err != nil {
			return fail(err)
		}
		if _, err := tx.Exec(schema); err != nil {
			_ = tx.Rollback()
			return fail(err)
		}
		if err := tx.Commit(); err != nil {
			return fail(err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Snapshot(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{ProtocolVersion: 1, Projects: []Project{}, Members: []Member{}, Goals: []Goal{}}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return snap, err
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence),0) FROM events").Scan(&snap.Cursor); err != nil {
		return snap, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,name,path FROM projects ORDER BY rowid")
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Projects = append(snap.Projects, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snap, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,name,role FROM members ORDER BY position")
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Role); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Members = append(snap.Members, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snap, err
	}
	rows, err = tx.QueryContext(ctx, "SELECT id,project_id,title,description,state,version FROM goals ORDER BY rowid DESC")
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.Title, &g.Description, &g.State, &g.Version); err != nil {
			rows.Close()
			return snap, err
		}
		snap.Goals = append(snap.Goals, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snap, err
	}
	return snap, tx.Commit()
}

func (s *Store) Events(ctx context.Context, after int64) ([]Event, error) {
	if after < 0 {
		return nil, ErrInvalid
	}
	events := []Event{}
	rows, err := s.db.QueryContext(ctx, "SELECT sequence,kind,entity_id,created_at FROM events WHERE sequence>? ORDER BY sequence LIMIT 100", after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Sequence, &e.Kind, &e.EntityID, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

var requestPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func validText(value string, limit int, required bool) bool {
	return utf8.ValidString(value) && len(value) <= limit && (!required || strings.TrimSpace(value) != "") && !strings.ContainsRune(value, '\x00')
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func write[T any](s *Store, ctx context.Context, requestID, kind string, input any, create func(*sql.Tx) (T, string, error)) (T, error) {
	var zero T
	if !requestPattern.MatchString(requestID) {
		return zero, ErrInvalid
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return zero, err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(append([]byte(kind+"\n"), encoded...)))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	var previous string
	var response []byte
	err = tx.QueryRowContext(ctx, "SELECT digest,response FROM requests WHERE id=?", requestID).Scan(&previous, &response)
	if err == nil {
		if previous != digest {
			return zero, ErrConflict
		}
		var value T
		if err := json.Unmarshal(response, &value); err != nil {
			return zero, err
		}
		return value, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return zero, err
	}
	value, id, err := create(tx)
	if err != nil {
		return zero, err
	}
	response, err = json.Marshal(value)
	if err != nil {
		return zero, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO requests(id,digest,response) VALUES(?,?,?)", requestID, digest, response); err != nil {
		return zero, err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO events(kind,entity_id) VALUES(?,?)", kind, id); err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return value, nil
}

func (s *Store) CreateProject(ctx context.Context, requestID, name, path string) (Project, error) {
	name = strings.TrimSpace(name)
	if !validText(name, 160, true) || !validText(path, 4096, true) || !filepath.IsAbs(path) {
		return Project{}, ErrInvalid
	}
	return write(s, ctx, requestID, "project.created", []string{name, path}, func(tx *sql.Tx) (Project, string, error) {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return Project{}, "", ErrInvalid
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return Project{}, "", ErrInvalid
		}
		var existing string
		err = tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE path=?", resolved).Scan(&existing)
		if err == nil {
			return Project{}, "", ErrConflict
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return Project{}, "", err
		}
		p := Project{ID: newID(), Name: name, Path: resolved}
		_, err = tx.ExecContext(ctx, "INSERT INTO projects(id,name,path) VALUES(?,?,?)", p.ID, p.Name, p.Path)
		return p, p.ID, err
	})
}

func (s *Store) CreateGoal(ctx context.Context, requestID, projectID, title, description string) (Goal, error) {
	title = strings.TrimSpace(title)
	if !validText(projectID, 128, true) || !validText(title, 240, true) || !validText(description, 32000, false) {
		return Goal{}, ErrInvalid
	}
	return write(s, ctx, requestID, "goal.created", []string{projectID, title, description}, func(tx *sql.Tx) (Goal, string, error) {
		var existing string
		err := tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE id=?", projectID).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return Goal{}, "", ErrNotFound
		}
		if err != nil {
			return Goal{}, "", err
		}
		g := Goal{ID: newID(), ProjectID: projectID, Title: title, Description: description, State: "awaiting_engine", Version: 1}
		_, err = tx.ExecContext(ctx, "INSERT INTO goals(id,project_id,title,description,state,version) VALUES(?,?,?,?,?,?)", g.ID, g.ProjectID, g.Title, g.Description, g.State, g.Version)
		return g, g.ID, err
	})
}
