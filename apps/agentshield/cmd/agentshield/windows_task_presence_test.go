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

func TestWindowsTaskPresenceProtocol(t *testing.T) {
	for _, tc := range []struct {
		output, stderr string
		err            error
		present, valid bool
	}{
		{"SIQ_TASK_PRESENT", "", nil, true, true},
		{"SIQ_TASK_ABSENT", "", nil, false, true},
		{"SIQ_TASK_ABSENT", "", errors.New("failed"), false, false},
		{"SIQ_TASK_ABSENT", "warning", nil, false, false},
		{"SIQ_TASK_PRESENT\n", "", nil, false, false},
		{"", "", nil, false, false},
		{"SIQ_TASK_ABSENTSIQ_TASK_PRESENT", "", nil, false, false},
	} {
		present, err := windowsTaskPresenceResult(tc.output, tc.stderr, tc.err)
		if (err == nil) != tc.valid || present != tc.present {
			t.Fatalf("response %q: %t %v", tc.output, present, err)
		}
	}
	// Known UTF-16LE/base64 vector for EncodedCommand, including non-ASCII.
	if got := encodeWindowsPowerShell("A中"); got != "QQAtTg==" {
		t.Fatalf("encoded command: %s", got)
	}
}

func TestWindowsOwnedTaskPresence(t *testing.T) {
	for _, scenario := range []string{"present", "absent", "failed", "unowned", "foreign task", "query failed", "source changed"} {
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
			calls, queries := 0, 0
			presence := func(name, userSID string) (bool, error) {
				calls++
				if name != `\SIQ-Agent-Security-`+instance.InstanceID || userSID != sid {
					t.Fatal("wrong identity")
				}
				if scenario == "failed" {
					return false, errors.New("access denied")
				}
				if scenario == "source changed" {
					if err := os.WriteFile(filepath.Join(st.Dir, "SIQ-Agent-Security-"+instance.InstanceID+".xml"), []byte("drift"), 0600); err != nil {
						t.Fatal(err)
					}
					return false, nil
				}
				return scenario != "absent", nil
			}
			query := func(name string) ([]byte, error) {
				queries++
				if scenario == "query failed" {
					return nil, errors.New("gone")
				}
				if scenario == "foreign task" {
					return []byte(strings.Replace(expected, "LeastPrivilege", "HighestAvailable", 1)), nil
				}
				return []byte(expected), nil
			}
			var out bytes.Buffer
			err = inspectOwnedWindowsTask(st, key, []byte(expected), sid, presence, query, &out)
			if scenario == "present" || scenario == "absent" {
				if err != nil || out.String() != scenario+"\n" {
					t.Fatalf("result: %q %v", out.String(), err)
				}
			} else if err == nil || out.Len() != 0 {
				t.Fatal("unconfirmed presence accepted")
			}
			if scenario == "unowned" && calls != 0 {
				t.Fatal("queried unowned task")
			}
			if (scenario == "absent" || scenario == "failed" || scenario == "source changed") && queries != 0 {
				t.Fatal("queried configuration without presence")
			}
		})
	}
}
