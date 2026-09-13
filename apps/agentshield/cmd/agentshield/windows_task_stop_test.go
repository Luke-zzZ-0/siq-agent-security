package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestWindowsTaskStop(t *testing.T) {
	for _, scenario := range []string{"running", "idle", "queued", "foreign", "request failed", "drain failed", "still running", "exit failed", "final drift", "writer busy"} {
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
			idle := scenario == "idle" || scenario == "writer busy" || scenario == "final drift"
			if idle && scenario != "writer busy" {
				if err = w.Release(); err != nil {
					t.Fatal(err)
				}
			}
			queries, requests := 0, 0
			query := func(name string) ([]byte, error) {
				if name != record.TaskName {
					t.Fatal("wrong task")
				}
				queries++
				if scenario == "foreign" || (scenario == "final drift" && queries >= 5) {
					return []byte(strings.Replace(expected, "LeastPrivilege", "HighestAvailable", 1)), nil
				}
				return []byte(expected), nil
			}
			runtime := func(name, user string) (string, error) {
				if name != record.TaskName || user != sid {
					t.Fatal("wrong runtime identity")
				}
				if scenario == "queued" {
					return "SIQ_TASK_RUNTIME:2:0:0", nil
				}
				if idle {
					return "SIQ_TASK_RUNTIME:3:0:1", nil
				} // Historical failure does not preclude confirming idle.
				if requests == 0 || scenario == "still running" {
					return "SIQ_TASK_RUNTIME:4:1:0", nil
				}
				if scenario == "exit failed" {
					return "SIQ_TASK_RUNTIME:3:0:1", nil
				}
				return "SIQ_TASK_RUNTIME:3:0:0", nil
			}
			request := func() (state.ServiceStopAcceptance, error) {
				requests++
				if scenario == "request failed" {
					return state.ServiceStopAcceptance{}, errors.New("unavailable")
				}
				directory, err := st.DirectoryID()
				if err != nil {
					t.Fatal(err)
				}
				now := time.Now()
				control, err := localcontrol.New(key, directory, func() time.Time { return now })
				if err != nil {
					t.Fatal(err)
				}
				challenge, err := control.Challenge()
				if err != nil {
					t.Fatal(err)
				}
				signed, err := localcontrol.SignStop(challenge, key, directory, now)
				if err != nil {
					t.Fatal(err)
				}
				accepted, err := st.RecordServiceStopAcceptance(w, key, signed, now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = st.RecordServiceStopResult(w, key, signed.BootID, scenario != "drain failed", now); err != nil {
					t.Fatal(err)
				}
				if err = w.Release(); err != nil {
					t.Fatal(err)
				}
				return accepted, nil
			}
			err = stopOwnedWindowsTask(st, key, []byte(expected), sid, query, runtime, request, 0)
			success := scenario == "running" || scenario == "idle"
			if success && err != nil {
				t.Fatal(err)
			}
			if !success && err == nil {
				t.Fatal("unconfirmed stop accepted")
			}
			wantRequests := 1
			if idle || scenario == "queued" || scenario == "foreign" {
				wantRequests = 0
			}
			if requests != wantRequests {
				t.Fatalf("requests = %d, want %d", requests, wantRequests)
			}
			saved, err := st.VerifyWindowsTask(key, []byte(expected), sid)
			if err != nil || saved != record {
				t.Fatal("task ownership changed", err)
			}
			if success || scenario == "final drift" {
				check, err := state.AcquireWriter(st.Dir)
				if err != nil {
					t.Fatal("verification lock leaked", err)
				}
				_ = check.Release()
			}
		})
	}
}

func TestWindowsTaskStopConfirmation(t *testing.T) {
	for _, args := range [][]string{nil, {"--confirm-stop=false"}, {"--confirm-stop", "extra"}} {
		if cmdWindowsTaskStop(args, &bytes.Buffer{}) == nil {
			t.Fatal("invalid confirmation accepted")
		}
	}
}
