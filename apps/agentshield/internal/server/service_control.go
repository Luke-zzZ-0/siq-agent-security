package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/state"
)

func (s *Server) AcceptedStopBootID() string {
	if s.serviceControl == nil {
		return ""
	}
	return s.serviceControl.AcceptedBootID()
}

func (s *Server) serviceControlGuard(w http.ResponseWriter, r *http.Request, method string) bool {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != method {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return false
	}
	if r.Header.Get("X-SIQ-Local-CLI") != "1" {
		writeJSON(w, 403, map[string]string{"error": "local CLI required"})
		return false
	}
	for _, header := range []string{"Origin", "Authorization", "Cookie", "Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User"} {
		if r.Header.Get(header) != "" {
			writeJSON(w, 403, map[string]string{"error": "signed local CLI request required"})
			return false
		}
	}
	if r.URL.RawQuery != "" {
		writeJSON(w, 400, map[string]string{"error": "query parameters not accepted"})
		return false
	}
	if s.serviceControl == nil {
		writeJSON(w, 503, map[string]string{"error": "stop control unavailable"})
		return false
	}
	return true
}

func (s *Server) serviceControlChallenge(w http.ResponseWriter, r *http.Request) {
	if !s.serviceControlGuard(w, r, http.MethodGet) {
		return
	}
	challenge, err := s.serviceControl.Challenge()
	if err != nil {
		writeJSON(w, 409, map[string]string{"error": "stop already accepted"})
		return
	}
	writeJSON(w, 200, challenge)
}

func decodeServiceStop(r *http.Request) (localcontrol.Message, error) {
	var message localcontrol.Message
	invalid := errors.New("invalid stop request")
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return message, invalid
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4097))
	if err != nil || len(raw) > 4096 {
		return message, invalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return message, invalid
	}
	allowed := map[string]bool{"schema_version": true, "action": true, "boot_id": true, "state_directory_id": true, "expires_at": true, "signature": true}
	seen := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return message, invalid
		}
		name, ok := token.(string)
		if !ok || !allowed[name] || seen[name] {
			return message, invalid
		}
		seen[name] = true
		var value json.RawMessage
		if err := d.Decode(&value); err != nil || bytes.Equal(value, []byte("null")) {
			return message, invalid
		}
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') || len(seen) != len(allowed) {
		return message, invalid
	}
	if d.Decode(new(any)) != io.EOF || json.Unmarshal(raw, &message) != nil {
		return message, invalid
	}
	return message, nil
}

func (s *Server) serviceControlStop(w http.ResponseWriter, r *http.Request) {
	if !s.serviceControlGuard(w, r, http.MethodPost) {
		return
	}
	request, err := decodeServiceStop(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid stop request"})
		return
	}
	var accepted state.ServiceStopAcceptance
	recordFailed := false
	err = s.serviceControl.Accept(request, func() error {
		var err error
		accepted, err = s.d.Store.RecordServiceStopAcceptance(s.d.StopWriter, s.d.Key, request, time.Now())
		recordFailed = err != nil
		return err
	})
	if err != nil {
		status := http.StatusForbidden
		if recordFailed {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": "stop not accepted"})
		return
	}
	writeJSON(w, http.StatusAccepted, accepted)
	s.d.RequestStop()
}
