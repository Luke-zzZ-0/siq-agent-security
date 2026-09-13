package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestWindowsTaskRuntimeProtocol(t *testing.T) {
	for raw, want := range map[string]windowsTaskRuntime{
		"SIQ_TASK_RUNTIME:3:0:0":           {"ready", 0, 0},
		"SIQ_TASK_RUNTIME:4:1:267009":      {"running", 1, 267009},
		"SIQ_TASK_RUNTIME:2:0:0":           {"queued", 0, 0},
		"SIQ_TASK_RUNTIME:3:0:-2147483648": {"ready", 0, -2147483648},
		"SIQ_TASK_RUNTIME:3:0:4294967295":  {"ready", 0, 4294967295},
	} {
		got, err := decodeWindowsTaskRuntime(raw)
		if err != nil || got != want {
			t.Fatalf("%q: %+v %v", raw, got, err)
		}
	}
	for _, raw := range []string{
		"", "SIQ_TASK_RUNTIME:0:0:0", "SIQ_TASK_RUNTIME:1:0:0", "SIQ_TASK_RUNTIME:3:1:0",
		"SIQ_TASK_RUNTIME:4:0:0", "SIQ_TASK_RUNTIME:4:2:0", "SIQ_TASK_RUNTIME:2:1:0",
		"SIQ_TASK_RUNTIME:3:-1:0", "SIQ_TASK_RUNTIME:03:0:0", "SIQ_TASK_RUNTIME:3:0:+0",
		"SIQ_TASK_RUNTIME:3:0:-0", "SIQ_TASK_RUNTIME:3:0:0\n", "SIQ_TASK_RUNTIME:3:0:0:extra",
		"SIQ_TASK_RUNTIME:3:0:-2147483649", "SIQ_TASK_RUNTIME:3:0:4294967296",
	} {
		if _, err := decodeWindowsTaskRuntime(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestWindowsOwnedTaskRuntime(t *testing.T) {
	for _, scenario := range []string{"ready", "running", "queued", "initial drift", "final drift", "runtime failed"} {
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
			if _, err := st.PrepareWindowsTask(w, key, []byte(expected), sid); err != nil {
				t.Fatal(err)
			}
			queries, calls := 0, 0
			query := func(name string) ([]byte, error) {
				queries++
				if scenario == "initial drift" || (scenario == "final drift" && queries == 2) {
					return []byte(strings.Replace(expected, "LeastPrivilege", "HighestAvailable", 1)), nil
				}
				return []byte(expected), nil
			}
			runtime := func(name, user string) (string, error) {
				calls++
				if name != `\SIQ-Agent-Security-`+instance.InstanceID || user != sid {
					t.Fatal("wrong identity")
				}
				if scenario == "runtime failed" {
					return "SIQ_TASK_RUNTIME:3:0:0", errors.New("failed")
				}
				if scenario == "running" {
					return "SIQ_TASK_RUNTIME:4:1:0", nil
				}
				if scenario == "queued" {
					return "SIQ_TASK_RUNTIME:2:0:0", nil
				}
				return "SIQ_TASK_RUNTIME:3:0:0", nil
			}
			got, err := readOwnedWindowsTaskRuntime(st, key, []byte(expected), sid, query, runtime)
			if scenario == "ready" || scenario == "running" || scenario == "queued" {
				if err != nil || got.State != scenario || queries != 2 {
					t.Fatalf("result %+v %v", got, err)
				}
			} else if err == nil || got != (windowsTaskRuntime{}) {
				t.Fatal("unconfirmed state returned")
			}
			if scenario == "initial drift" && calls != 0 {
				t.Fatal("queried runtime of foreign task")
			}
		})
	}
}
