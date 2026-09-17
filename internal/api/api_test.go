package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"multi-agent-team/internal/api"
	"multi-agent-team/internal/store"
)

func fixture(t *testing.T) http.Handler {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return api.New(s)
}

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHTTPGoalSubmissionAndRetry(t *testing.T) {
	h := fixture(t)
	body, _ := json.Marshal(map[string]string{"request_id": "project", "name": "Project", "path": t.TempDir()})
	w := send(h, "POST", "/v1/projects", string(body))
	if w.Code != 201 {
		t.Fatalf("register project: %d %s", w.Code, w.Body.String())
	}
	var p store.Project
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	body = []byte(`{"request_id":"goal","title":"目标","description":"验收标准"}`)
	w = send(h, "POST", "/v1/projects/"+p.ID+"/goals", string(body))
	if w.Code != 201 {
		t.Fatalf("submit goal: %d %s", w.Code, w.Body.String())
	}
	first := w.Body.String()
	w = send(h, "POST", "/v1/projects/"+p.ID+"/goals", string(body))
	if w.Code != 201 || w.Body.String() != first {
		t.Fatalf("retry changed result: %d %s", w.Code, w.Body.String())
	}
	w = send(h, "GET", "/v1/snapshot", "")
	var snap store.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if len(snap.Goals) != 1 || snap.Cursor != 2 || len(snap.Members) != 6 {
		t.Fatalf("snapshot inconsistent: %+v", snap)
	}
}

func TestHTTPRejectsMalformedAndUnauthorizedRequests(t *testing.T) {
	h := fixture(t)
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"/v1/projects", `{}`, 400},
		{"/v1/projects", `{"request_id":"x","name":"x","path":"/tmp","role":"admin"}`, 400},
		{"/v1/projects", `{} {}`, 400},
		{"/v1/projects/missing/goals", `{"request_id":"x","title":"goal"}`, 404},
		{"/v1/projects", fmt.Sprintf(`{"description":%q}`, strings.Repeat("x", 70000)), 413},
	} {
		w := send(h, "POST", tc.path, tc.body)
		if w.Code != tc.status {
			t.Errorf("%s: got %d wanted %d: %s", tc.path, w.Code, tc.status, w.Body.String())
		}
	}
	if w := send(h, "GET", "/v1/events?after=-1", ""); w.Code != 400 {
		t.Errorf("negative cursor: %d", w.Code)
	}
	r := httptest.NewRequest("GET", "/v1/snapshot", nil)
	r.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("browser origin not rejected: %d", w.Code)
	}
}
