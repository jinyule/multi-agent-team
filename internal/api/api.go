package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"multi-agent-team/internal/store"
	"net/http"
	"strconv"
)

func New(s *store.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, 200, map[string]any{"status": "ok", "version": "0.1.0", "protocol_version": 1})
	})
	mux.HandleFunc("GET /v1/snapshot", func(w http.ResponseWriter, r *http.Request) {
		value, err := s.Snapshot(r.Context())
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, value)
	})
	mux.HandleFunc("GET /v1/events", func(w http.ResponseWriter, r *http.Request) {
		var after int64
		if raw := r.URL.Query().Get("after"); raw != "" {
			var err error
			after, err = strconv.ParseInt(raw, 10, 64)
			if err != nil {
				failure(w, store.ErrInvalid)
				return
			}
		}
		value, err := s.Events(r.Context(), after)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 200, value)
	})
	mux.HandleFunc("POST /v1/projects", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			RequestID string `json:"request_id"`
			Name      string `json:"name"`
			Path      string `json:"path"`
		}
		if !decode(w, r, &input) {
			return
		}
		value, err := s.CreateProject(r.Context(), input.RequestID, input.Name, input.Path)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 201, value)
	})
	mux.HandleFunc("POST /v1/projects/{id}/goals", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			RequestID   string `json:"request_id"`
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		if !decode(w, r, &input) {
			return
		}
		value, err := s.CreateGoal(r.Context(), input.RequestID, r.PathValue("id"), input.Title, input.Description)
		if err != nil {
			failure(w, err)
			return
		}
		respond(w, 201, value)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Header.Get("Origin") != "" {
			respond(w, 403, map[string]string{"error": "origin_rejected"})
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	typeName, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typeName != "application/json" {
		respond(w, 415, map[string]string{"error": "json_required"})
		return false
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respond(w, 413, map[string]string{"error": "request_too_large"})
		} else {
			failure(w, store.ErrInvalid)
		}
		return false
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		failure(w, store.ErrInvalid)
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		failure(w, store.ErrInvalid)
		return false
	}
	return true
}

func failure(w http.ResponseWriter, err error) {
	status, code := 500, "internal_error"
	switch {
	case errors.Is(err, store.ErrInvalid):
		status, code = 400, "invalid_input"
	case errors.Is(err, store.ErrNotFound):
		status, code = 404, "not_found"
	case errors.Is(err, store.ErrConflict):
		status, code = 409, "request_conflict"
	}
	respond(w, status, map[string]string{"error": code})
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
