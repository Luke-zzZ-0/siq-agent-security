package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/state"
)

func controlServer(t *testing.T) (*Server, *int) {
	t.Helper()
	s, st := newServer(t, "block")
	w, err := state.AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Release() })
	d := s.d
	d.StopWriter = w
	calls := 0
	d.RequestStop = func() { calls++ }
	s, err = New(d)
	if err != nil {
		t.Fatal(err)
	}
	return s, &calls
}

func signedControlRequest(t *testing.T, s *Server) localcontrol.Message {
	t.Helper()
	w := sessionRequest(t, s, "GET", "/v1/service-control/challenge", nil, "", nil, map[string]string{"X-SIQ-Local-CLI": "1"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var challenge localcontrol.Message
	if err := json.Unmarshal(w.Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	r, err := localcontrol.SignStop(challenge, s.d.Key, s.stateDirectoryID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestServiceControlAcceptedRecordAndReplay(t *testing.T) {
	s, calls := controlServer(t)
	r := signedControlRequest(t, s)
	s.d.RequestStop = func() {
		if _, err := s.d.Store.ReadServiceStopAcceptance(s.d.Key, r.BootID); err != nil {
			t.Error("not recorded before notification", err)
		}
		*calls++
	}
	headers := map[string]string{"X-SIQ-Local-CLI": "1", "Content-Type": "application/json"}
	w := sessionRequest(t, s, "POST", "/v1/service-control/stop", r, "", nil, headers)
	if w.Code != 202 || *calls != 1 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, *calls)
	}
	var result state.ServiceStopAcceptance
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.BootID != r.BootID {
		t.Fatal("invalid acceptance", err)
	}
	if w := sessionRequest(t, s, "POST", "/v1/service-control/stop", r, "", nil, headers); w.Code != 403 || *calls != 1 {
		t.Fatal("replay accepted")
	}
}

func TestServiceControlRecordFailureRetry(t *testing.T) {
	s, calls := controlServer(t)
	r := signedControlRequest(t, s)
	path := filepath.Join(s.d.Store.Dir, "service-stop-"+r.BootID+".json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"X-SIQ-Local-CLI": "1", "Content-Type": "application/json"}
	if w := sessionRequest(t, s, "POST", "/v1/service-control/stop", r, "", nil, headers); w.Code != 503 || *calls != 0 {
		t.Fatal("record failure stopped service")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if w := sessionRequest(t, s, "POST", "/v1/service-control/stop", r, "", nil, headers); w.Code != 202 || *calls != 1 {
		t.Fatal("retry failed", w.Code)
	}
}

func TestServiceControlRequestBoundaries(t *testing.T) {
	s, calls := controlServer(t)
	r := signedControlRequest(t, s)
	raw, _ := json.Marshal(r)
	for _, header := range []string{"Authorization", "Cookie", "Origin", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User"} {
		w := sessionRequest(t, s, "POST", "/v1/service-control/stop", r, "", nil, map[string]string{"X-SIQ-Local-CLI": "1", "Content-Type": "application/json", header: "untrusted"})
		if w.Code != 403 {
			t.Fatal("credential/browser accepted", header, w.Code)
		}
	}
	for _, body := range []string{
		string(raw) + "{}", strings.Replace(string(raw), `"action":"stop"`, `"action":"stop","action":"stop"`, 1),
		strings.Replace(string(raw), `"action":"stop"`, `"unknown":"stop"`, 1),
		strings.Replace(string(raw), `"action":"stop"`, `"action":null`, 1),
		strings.Repeat(" ", 4097), "[]",
	} {
		req := httptest.NewRequest("POST", "http://127.0.0.1:47611/v1/service-control/stop", strings.NewReader(body))
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-SIQ-Local-CLI", "1")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatal("bad body accepted", w.Code)
		}
	}
	r.Signature = strings.Repeat("0", 128)
	if w := sessionRequest(t, s, "POST", "/v1/service-control/stop", r, "", nil, map[string]string{"X-SIQ-Local-CLI": "1", "Content-Type": "application/json"}); w.Code != 403 {
		t.Fatal("invalid signature accepted")
	}
	if *calls != 0 {
		t.Fatal("invalid request notified stop")
	}
}
