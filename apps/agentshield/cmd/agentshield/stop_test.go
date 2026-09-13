package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestStopCompletionChecks(t *testing.T) {
	for _, scenario := range []string{"released", "busy", "missing", "failed", "tampered", "wrong acceptance"} {
		t.Run(scenario, func(t *testing.T) {
			st, err := state.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			w, err := state.AcquireWriter(st.Dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Release()
			key, _ := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
			directory, err := st.DirectoryID()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
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
			accepted, err := st.RecordServiceStopAcceptance(w, key, request, now)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "missing" {
				if _, err := st.RecordServiceStopResult(w, key, request.BootID, scenario != "failed", now); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "tampered" {
				if err := os.WriteFile(filepath.Join(st.Dir, "service-stop-"+request.BootID+".result.json"), []byte("unknown"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario != "busy" {
				if err := w.Release(); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "wrong acceptance" {
				accepted.RequestSHA256 = strings.Repeat("0", 64)
			}
			result, err := waitLocalStopResult(st, key, accepted, 0)
			if scenario == "released" {
				if err != nil || result.Status != "drained" {
					t.Fatal("completion not verified", err)
				}
				writer, err := state.AcquireWriter(st.Dir)
				if err != nil {
					t.Fatal("verification lock leaked", err)
				}
				_ = writer.Release()
			} else if err == nil || result != (state.ServiceStopResult{}) {
				t.Fatal("unconfirmed completion accepted")
			}
			if (scenario == "missing" || scenario == "busy") && !strings.Contains(err.Error(), request.BootID) {
				t.Fatal("missing recovery identity")
			}
		})
	}
}

func TestStopConfirmation(t *testing.T) {
	for _, args := range [][]string{nil, {"--confirm-stop=false"}, {"--confirm-stop", "extra"}, {"--confirm-stop", "--recover", ""}} {
		if cmdStop(args, &bytes.Buffer{}) == nil {
			t.Fatal("invalid confirmation accepted")
		}
	}
}
