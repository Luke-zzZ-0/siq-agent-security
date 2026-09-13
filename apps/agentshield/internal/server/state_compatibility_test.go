package server

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"siq-agent-security/apps/agentshield/internal/state"
	"testing"
	"time"
)

func TestIncompatibleStateRejectsHTTPAndMaintenance(t *testing.T) {
	s, st := newServer(t, "block")
	path := filepath.Join(st.Dir, state.StateFormatMarkerName)
	if err := os.WriteFile(path, []byte(`{"schema":"state-format/v1","program_version":"test","format_version":999,"published_at":"2026-09-13T09:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"/v1/decide", "/v1/grants", "/v1/raw-task-content/purge-expired", "/v1/skill-imports/local"} {
		req := httptest.NewRequest("POST", "http://127.0.0.1:47611"+endpoint, nil)
		req.RemoteAddr = "127.0.0.1:1234"
		out := httptest.NewRecorder()
		s.Handler().ServeHTTP(out, req)
		if out.Code != 503 || out.Body.String() != "{\"error\":\"state_incompatible\"}\n" {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	if err := s.PurgeExpiredRawContent(time.Now()); !errors.Is(err, state.ErrIncompatibleState) {
		t.Fatal(err)
	}
	if _, err := New(s.d); !errors.Is(err, state.ErrIncompatibleState) {
		t.Fatal(err)
	}
}
