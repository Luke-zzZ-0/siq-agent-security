package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestWindowsTaskUnregister(t *testing.T) {
	for _, scenario := range []string{"idle", "absent", "running", "queued", "foreign", "presence failed", "delete failed", "still present", "final query failed", "reappeared"} {
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
			if _, err = st.Initialize(w, 0); err != nil {
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
			record, err := st.PrepareWindowsTask(w, key, []byte(expected), sid)
			if err != nil {
				t.Fatal(err)
			}
			queries, deletes, probes := 0, 0, 0
			presence := func(name, user string) (bool, error) {
				if name != record.TaskName || user != sid {
					t.Fatal("wrong identity")
				}
				probes++
				if scenario == "presence failed" || (scenario == "final query failed" && probes == 2) {
					return false, errors.New("unavailable")
				}
				if scenario == "absent" || (scenario == "reappeared" && probes == 1) {
					return false, nil
				}
				return deletes == 0 || scenario == "still present" || scenario == "reappeared", nil
			}
			query := func(name string) ([]byte, error) {
				queries++
				if scenario == "foreign" {
					return []byte(strings.Replace(expected, "LeastPrivilege", "HighestAvailable", 1)), nil
				}
				return []byte(expected), nil
			}
			runtime := func(name, user string) (string, error) {
				if scenario == "running" {
					return "SIQ_TASK_RUNTIME:4:1:0", nil
				}
				if scenario == "queued" {
					return "SIQ_TASK_RUNTIME:2:0:0", nil
				}
				return "SIQ_TASK_RUNTIME:3:0:1", nil
			}
			remove := func(name, user string, snapshot []byte) error {
				deletes++
				if name != record.TaskName || user != sid || string(snapshot) != expected || queries < 2 {
					t.Fatal("unverified delete")
				}
				if scenario == "delete failed" {
					return errors.New("failure")
				}
				return nil
			}
			err = unregisterOwnedWindowsTask(st, key, []byte(expected), sid, presence, query, runtime, remove)
			success := scenario == "idle" || scenario == "absent"
			if success && err != nil {
				t.Fatal(err)
			}
			if !success && err == nil {
				t.Fatal("unconfirmed deletion accepted")
			}
			wantDeletes := 0
			if scenario == "idle" || scenario == "delete failed" || scenario == "still present" || scenario == "final query failed" {
				wantDeletes = 1
			}
			if deletes != wantDeletes {
				t.Fatalf("deletes = %d, want %d", deletes, wantDeletes)
			}
			saved, err := st.VerifyWindowsTask(key, []byte(expected), sid)
			if err != nil || saved != record {
				t.Fatal("local source changed", err)
			}
		})
	}
}

func TestWindowsTaskUnregisterConfirmation(t *testing.T) {
	for _, args := range [][]string{nil, {"--confirm-unregister=false"}, {"--confirm-unregister", "extra"}} {
		if cmdWindowsTaskUnregister(args, &bytes.Buffer{}) == nil {
			t.Fatal("invalid confirmation accepted")
		}
	}
}
