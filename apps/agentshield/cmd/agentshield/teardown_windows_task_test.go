package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"siq-agent-security/apps/agentshield/internal/localcontrol"
	"testing"
	"time"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestWindowsTeardownRecovery(t *testing.T) {
	for _, scenario := range []string{"running", "drain failed", "idle", "absent", "query failed", "stop failed", "delete failed", "busy", "lifecycle busy"} {
		t.Run(scenario, func(t *testing.T) {
			st, err := state.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			w, err := state.AcquireWriter(st.Dir)
			if err != nil {
				t.Fatal(err)
			}
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
			if err = w.Release(); err != nil {
				t.Fatal(err)
			}
			var busy *state.Writer
			if scenario == "busy" || scenario == "lifecycle busy" {
				dir := st.Dir
				if scenario == "lifecycle busy" {
					dir = filepath.Join(dir, "service-control")
				}
				busy, err = state.AcquireWriter(dir)
				if err != nil {
					t.Fatal(err)
				}
				defer busy.Release()
			}
			var daemon *state.Writer
			running := scenario == "running" || scenario == "drain failed"
			if running {
				daemon, err = state.AcquireWriter(st.Dir)
				if err != nil {
					t.Fatal(err)
				}
				defer daemon.Release()
			}
			var accepted state.ServiceStopAcceptance
			present := scenario != "absent"
			deletes, requests := 0, 0
			retry := false
			presence := func(name, user string) (bool, error) {
				if name != record.TaskName || user != sid {
					t.Fatal("wrong identity")
				}
				if scenario == "query failed" {
					return false, errors.New("unavailable")
				}
				return present, nil
			}
			query := func(string) ([]byte, error) { return []byte(expected), nil }
			runtime := func(string, string) (string, error) {
				if scenario == "stop failed" || running {
					return "SIQ_TASK_RUNTIME:4:1:0", nil
				}
				return "SIQ_TASK_RUNTIME:3:0:0", nil
			}
			request := func() (state.ServiceStopAcceptance, error) {
				requests++
				if daemon == nil {
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
				accepted, err = st.RecordServiceStopAcceptance(daemon, key, signed, now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = st.RecordServiceStopResult(daemon, key, signed.BootID, scenario != "drain failed", now); err != nil {
					t.Fatal(err)
				}
				if err = daemon.Release(); err != nil {
					t.Fatal(err)
				}
				running = false
				return accepted, nil
			}
			remove := func(name, user string, snapshot []byte) error {
				deletes++
				if name != record.TaskName || user != sid || string(snapshot) != expected {
					t.Fatal("wrong deletion")
				}
				if scenario == "running" {
					result, err := st.ReadServiceStopResult(key, accepted.BootID)
					if err != nil || result.Status != "drained" || running {
						t.Fatal("deleted before confirmed drain", err)
					}
				}
				// The deletion may complete even if its response is lost.
				present = false
				if scenario == "delete failed" && !retry {
					return errors.New("response lost")
				}
				return nil
			}
			run := func() error {
				return teardownWindowsTask(st, key, []byte(expected), sid, presence, query, runtime, request, remove, 0)
			}
			err = run()
			success := scenario == "running" || scenario == "idle" || scenario == "absent"
			if (err == nil) != success {
				t.Fatal("unexpected result", err)
			}
			if scenario == "delete failed" {
				retry = true
				if err = run(); err != nil {
					t.Fatal("recovery failed", err)
				}
				success = true
			}
			if success {
				if err = run(); err != nil {
					t.Fatal("repeat failed", err)
				}
			}
			wantDeletes := 0
			if scenario == "running" || scenario == "idle" || scenario == "delete failed" {
				wantDeletes = 1
			}
			if deletes != wantDeletes {
				t.Fatalf("deletes=%d want=%d", deletes, wantDeletes)
			}
			wantRequests := 0
			if scenario == "running" || scenario == "drain failed" || scenario == "stop failed" {
				wantRequests = 1
			}
			if requests != wantRequests {
				t.Fatal("unexpected stop request")
			}
			saved, err := st.VerifyWindowsTask(key, []byte(expected), sid)
			if err != nil || saved != record {
				t.Fatal("source changed", err)
			}
			if busy != nil {
				if err = busy.Release(); err != nil {
					t.Fatal(err)
				}
			}
			for _, dir := range []string{st.Dir, filepath.Join(st.Dir, "service-control")} {
				check, err := state.AcquireWriter(dir)
				if err != nil {
					t.Fatal("lock leaked", err)
				}
				_ = check.Release()
			}
		})
	}
}
