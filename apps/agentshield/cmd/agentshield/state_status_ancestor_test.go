package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"siq-agent-security/apps/agentshield/internal/state"
)

func TestStateStatusReportsAncestorBarrier(t *testing.T) {
	for _, test := range []struct {
		name, marker, status string
	}{
		{"future", `{"schema":"state-format/v1","program_version":"fixture","format_version":999,"published_at":"2026-09-13T09:00:00Z"}`, state.CompatStatusFuture},
		{"corrupt", `{"schema":"state-format/v1","schema":"state-format/v1"}`, state.CompatStatusCorrupt},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "nested-instance")
			t.Setenv("SIQ_AGENT_SECURITY_STATE_DIR", dir)
			if err := cmdInitialize([]string{"--port", "47619"}, io.Discard); err != nil {
				t.Fatal(err)
			}
			var before bytes.Buffer
			if err := cmdStateStatus(nil, &before); err != nil || !bytes.Contains(before.Bytes(), []byte(`"compatible":true`)) {
				t.Fatalf("compatible instance diagnosis: %s %v", before.Bytes(), err)
			}
			markerPath := filepath.Join(parent, state.StateFormatMarkerName)
			if err := os.WriteFile(markerPath, []byte(test.marker), 0600); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := cmdStateStatus(nil, &out); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Compatible bool   `json:"compatible"`
				Status     string `json:"status"`
				Recovery   string `json:"recovery"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Compatible || result.Status != test.status || result.Recovery == "" {
				t.Fatalf("ancestor barrier hidden: %s %v", out.Bytes(), err)
			}
			if raw, err := os.ReadFile(markerPath); err != nil || string(raw) != test.marker {
				t.Fatal("diagnosis changed ancestor marker", err)
			}
			if bytes.Contains(out.Bytes(), []byte(parent)) {
				t.Fatal("diagnosis exposed a private directory")
			}
		})
	}
}
