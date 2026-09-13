package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/signing"
)

func TestServiceStopAcceptanceOwnership(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Release()
	key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	directory, err := st.DirectoryID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	control, err := localcontrol.New(key, directory, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := control.Challenge()
	if err != nil {
		t.Fatal(err)
	}
	request, err := localcontrol.SignStop(challenge, key, directory, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordServiceStopAcceptance(nil, key, request, now); err == nil {
		t.Fatal("missing writer accepted")
	}
	if _, err := st.RecordServiceStopAcceptance(w, key, request, now.Add(30*time.Second)); err == nil {
		t.Fatal("expired request accepted")
	}
	other, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ow, err := AcquireWriter(other.Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ow.Release()
	if _, err := other.RecordServiceStopAcceptance(ow, key, request, now); err == nil {
		t.Fatal("foreign directory accepted")
	}
	first, err := st.RecordServiceStopAcceptance(w, key, request, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.RecordServiceStopAcceptance(w, key, request, now.Add(time.Second))
	if err != nil || first != second {
		t.Fatal("idempotent replay changed record", err)
	}
	cloned, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other.Dir, "service-stop-"+first.BootID+".json"), cloned, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := other.ReadServiceStopAcceptance(key, first.BootID); err == nil {
		t.Fatal("foreign directory read succeeded")
	}
	// A different valid request for the same boot must not overwrite acceptance.
	now = now.Add(time.Second)
	challenge, err = control.Challenge()
	if err != nil {
		t.Fatal(err)
	}
	changed, err := localcontrol.SignStop(challenge, key, directory, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordServiceStopAcceptance(w, key, changed, now); err == nil {
		t.Fatal("conflicting request replaced record")
	}
	path := filepath.Join(st.Dir, "service-stop-"+first.BootID+".json")
	if err := os.WriteFile(path, []byte("unknown"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReadServiceStopAcceptance(key, first.BootID); err == nil {
		t.Fatal("tampered record accepted")
	}
	if _, err := st.RecordServiceStopAcceptance(w, key, request, now); err == nil {
		t.Fatal("unknown record overwritten")
	}
	if raw, err := os.ReadFile(path); err != nil || string(raw) != "unknown" {
		t.Fatal("unknown record changed")
	}
}

func TestServiceStopAcceptanceContract(t *testing.T) {
	key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/contracts/local-service-stop-request.json")
	if err != nil {
		t.Fatal(err)
	}
	var request localcontrol.Message
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	r, err := newServiceStopAcceptance(key, request, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	path := "../../testdata/contracts/local-service-stop-acceptance.json"
	if os.Getenv("SIQ_UPDATE_CONTROL_FIXTURES") == "1" {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, raw) {
		t.Fatal("fixture differs")
	}
}
