package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestWindowsTaskRegisterConfirmation(t *testing.T) {
	for _, args := range [][]string{nil, {"--confirm-register=false"}, {"--confirm-register", "extra"}} {
		var out bytes.Buffer
		if cmdWindowsTaskRegister(args, &out) == nil || out.Len() != 0 {
			t.Fatal("registration accepted without exact confirmation")
		}
	}
}

func TestWindowsTaskExclusiveRegistration(t *testing.T) {
	for _, scenario := range []string{"create", "reuse", "unknown presence", "foreign existing", "creation race", "readback failed", "created drift", "source changed", "unowned"} {
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
			if _, err := st.Initialize(w, 0); err != nil {
				t.Fatal(err)
			}
			instance, err := st.ReadLocalInstance()
			if err != nil {
				t.Fatal(err)
			}
			key, err := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
			if err != nil {
				t.Fatal(err)
			}
			const sid = "S-1-5-21-100-200-300-1001"
			expected, err := renderWindowsTask(`C:\SIQ\siq.exe`, `C:\SIQ\state`, instance.InstanceID, sid)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "unowned" {
				if _, err := st.PrepareWindowsTask(w, key, []byte(expected), sid); err != nil {
					t.Fatal(err)
				}
			}
			var steps []string
			presence := func(name, user string) (bool, error) {
				steps = append(steps, "presence")
				if name != `\SIQ-Agent-Security-`+instance.InstanceID || user != sid {
					t.Fatal("wrong identity")
				}
				if scenario == "unknown presence" {
					return false, errors.New("denied")
				}
				if scenario == "source changed" {
					if err := os.WriteFile(filepath.Join(st.Dir, "SIQ-Agent-Security-"+instance.InstanceID+".xml"), []byte("changed"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return scenario == "reuse" || scenario == "foreign existing", nil
			}
			create := func(name, user string, xml []byte) error {
				steps = append(steps, "create")
				if name != `\SIQ-Agent-Security-`+instance.InstanceID || user != sid || string(xml) != expected {
					t.Fatal("wrong creation request")
				}
				if scenario == "creation race" {
					return errors.New("already exists")
				}
				return nil
			}
			query := func(name string) ([]byte, error) {
				steps = append(steps, "query")
				if scenario == "readback failed" {
					return nil, errors.New("failed")
				}
				if scenario == "foreign existing" || scenario == "created drift" {
					return []byte(strings.Replace(expected, "LeastPrivilege", "HighestAvailable", 1)), nil
				}
				return []byte(expected), nil
			}
			var out bytes.Buffer
			err = registerOwnedWindowsTask(st, key, []byte(expected), sid, presence, query, create, &out)
			wantSteps := map[string]string{
				"create": "presence,create,query", "reuse": "presence,query", "unknown presence": "presence",
				"foreign existing": "presence,query", "creation race": "presence,create", "readback failed": "presence,create,query",
				"created drift": "presence,create,query", "source changed": "presence", "unowned": "",
			}[scenario]
			if strings.Join(steps, ",") != wantSteps {
				t.Fatalf("steps: %v", steps)
			}
			if scenario == "create" || scenario == "reuse" {
				if err != nil || out.String() != expected {
					t.Fatalf("registration: %v", err)
				}
			} else if err == nil || out.Len() != 0 {
				t.Fatal("unconfirmed registration accepted")
			}
			if scenario != "unowned" && scenario != "source changed" {
				if _, err := st.VerifyWindowsTask(key, []byte(expected), sid); err != nil {
					t.Fatal("local recovery state lost", err)
				}
			}
		})
	}
}
