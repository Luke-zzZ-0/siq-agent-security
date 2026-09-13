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

func TestServiceStopResults(t *testing.T) {
	for _, clean := range []bool{true, false} {
		st, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		w, err := AcquireWriter(st.Dir)
		if err != nil {
			t.Fatal(err)
		}
		defer w.Release()
		key, _ := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
		directory, err := st.DirectoryID()
		if err != nil {
			t.Fatal(err)
		}
		now := time.Unix(1700000000, 0)
		c, err := localcontrol.New(key, directory, func() time.Time { return now })
		if err != nil {
			t.Fatal(err)
		}
		challenge, err := c.Challenge()
		if err != nil {
			t.Fatal(err)
		}
		request, err := localcontrol.SignStop(challenge, key, directory, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.RecordServiceStopResult(w, key, request.BootID, clean, now); err == nil {
			t.Fatal("result without acceptance")
		}
		if _, err := st.RecordServiceStopAcceptance(w, key, request, now); err != nil {
			t.Fatal(err)
		}
		if _, err := st.RecordServiceStopResult(nil, key, request.BootID, clean, now); err == nil {
			t.Fatal("missing writer accepted")
		}
		if _, err := st.RecordServiceStopResult(w, key, request.BootID, clean, now.Add(-time.Second)); err == nil {
			t.Fatal("backward time accepted")
		}
		first, err := st.RecordServiceStopResult(w, key, request.BootID, clean, now)
		if err != nil {
			t.Fatal(err)
		}
		if (first.Status == "drained") != clean {
			t.Fatal("failure marked drained")
		}
		second, err := st.RecordServiceStopResult(w, key, request.BootID, clean, now.Add(time.Second))
		if err != nil || first != second {
			t.Fatal("retry changed result", err)
		}
		if _, err := st.RecordServiceStopResult(w, key, request.BootID, !clean, now); err == nil {
			t.Fatal("conflicting result overwritten")
		}
		path := filepath.Join(st.Dir, "service-stop-"+request.BootID+".result.json")
		if err := os.WriteFile(path, []byte("unknown"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := st.RecordServiceStopResult(w, key, request.BootID, clean, now); err == nil {
			t.Fatal("unknown result overwritten")
		}
	}
}

func TestServiceStopResultContract(t *testing.T) {
	key, _ := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	raw, err := os.ReadFile("../../testdata/contracts/local-service-stop-acceptance.json")
	if err != nil {
		t.Fatal(err)
	}
	var accepted ServiceStopAcceptance
	if err := json.Unmarshal(raw, &accepted); err != nil {
		t.Fatal(err)
	}
	r, err := newServiceStopResult(key, accepted, true, time.Unix(1700000001, 0))
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	path := "../../testdata/contracts/local-service-stop-result.json"
	if os.Getenv("SIQ_UPDATE_CONTROL_FIXTURES") == "1" {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	saved, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(saved, raw) {
		t.Fatal("fixture differs", err)
	}
}
