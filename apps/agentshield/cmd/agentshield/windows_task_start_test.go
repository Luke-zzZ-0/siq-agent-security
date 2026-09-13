package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func TestWindowsTaskStartConfirmationAndTemplates(t *testing.T) {
	for _, args := range [][]string{nil, {"--confirm-start=false"}, {"--confirm-start", "extra"}} {
		if cmdWindowsTaskStart(args, &bytes.Buffer{}) == nil {
			t.Fatal("missing confirmation accepted")
		}
	}
	for _, path := range []string{`C:\SIQ\$(Arg0).exe`, `C:\$(Arg1)\state`} {
		if windowsTaskPathValid(path) {
			t.Fatal("task action template accepted")
		}
	}
}

func TestWindowsTaskStartAndDirectoryHealth(t *testing.T) {
	for _, scenario := range []string{"start", "reuse", "writer busy", "query failed", "foreign task", "start failed", "health timeout", "final drift"} {
		t.Run(scenario, func(t *testing.T) {
			st, err := state.Open(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			w, err := state.AcquireWriter(st.Dir)
			if err != nil {
				t.Fatal(err)
			}
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
			if err := w.Release(); err != nil {
				t.Fatal(err)
			}
			if scenario == "writer busy" {
				busy, err := state.AcquireWriter(st.Dir)
				if err != nil {
					t.Fatal(err)
				}
				defer busy.Release()
			}
			starts, queries, healthCalls := 0, 0, 0
			query := func(name string) ([]byte, error) {
				queries++
				if scenario == "query failed" {
					return nil, errors.New("failed")
				}
				if scenario == "foreign task" || (scenario == "final drift" && healthCalls >= 2) {
					return []byte(strings.Replace(expected, "LeastPrivilege", "HighestAvailable", 1)), nil
				}
				return []byte(expected), nil
			}
			start := func(name, user string) error {
				starts++
				if name != `\SIQ-Agent-Security-`+instance.InstanceID || user != sid {
					t.Fatal("wrong instance")
				}
				writer, err := state.AcquireWriter(st.Dir)
				if err != nil {
					t.Fatal("writer held during task start", err)
				}
				if err := writer.Release(); err != nil {
					t.Fatal(err)
				}
				if scenario == "start failed" {
					return errors.New("failed")
				}
				return nil
			}
			health := func() error {
				healthCalls++
				if scenario == "reuse" || (starts > 0 && scenario != "health timeout") {
					return nil
				}
				return errors.New("not healthy")
			}
			err = startOwnedWindowsTask(st, key, []byte(expected), sid, query, start, health, 0)
			if (err == nil) != (scenario == "start" || scenario == "reuse") {
				t.Fatalf("result: %v", err)
			}
			wantStarts := 1
			if scenario == "reuse" || scenario == "writer busy" || scenario == "query failed" || scenario == "foreign task" {
				wantStarts = 0
			}
			if starts != wantStarts {
				t.Fatalf("start calls: %d", starts)
			}
			if queries == 0 {
				t.Fatal("configuration not checked")
			}
			if _, err := st.VerifyWindowsTask(key, []byte(expected), sid); err != nil {
				t.Fatal("recovery state changed", err)
			}
		})
	}
}
