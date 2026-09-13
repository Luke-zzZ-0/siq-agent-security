package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

func cmdWindowsTeardown() error {
	return withWindowsTaskIdentity(nil, func(st *state.Store, key *signing.Key, expected []byte, sid string) error {
		client := localClient()
		defer client.CloseIdleConnections()
		request := func() (state.ServiceStopAcceptance, error) {
			cfg, err := st.LoadConfig()
			if err != nil {
				return state.ServiceStopAcceptance{}, err
			}
			return requestSignedLocalStop(client, fmt.Sprintf("http://127.0.0.1:%d", cfg.Port), st, key)
		}
		return teardownWindowsTask(st, key, expected, sid, runWindowsTaskPresence, runWindowsTaskQuery, runWindowsTaskRuntime, request, runWindowsTaskDelete, 35*time.Second)
	})
}

func teardownWindowsTask(st *state.Store, key *signing.Key, expected []byte, sid string, presence func(string, string) (bool, error), query func(string) ([]byte, error), runtime func(string, string) (string, error), request func() (state.ServiceStopAcceptance, error), remove func(string, string, []byte) error, wait time.Duration) (resultErr error) {
	lifecycle, err := state.AcquireWriter(filepath.Join(st.Dir, "service-control"))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, lifecycle.Release()) }()
	if err := st.CheckServiceSwitchPending(); err != nil {
		return err
	}
	record, err := st.VerifyWindowsTask(key, expected, sid)
	if err != nil {
		return err
	}
	present, err := presence(record.TaskName, sid)
	if err != nil {
		return errors.New("teardown: Windows task presence unconfirmed")
	}
	if present {
		if err := stopOwnedWindowsTask(st, key, expected, sid, query, runtime, request, wait); err != nil {
			return err
		}
	}
	writer, err := state.AcquireWriter(st.Dir)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, writer.Release()) }()
	return unregisterOwnedWindowsTask(st, key, expected, sid, presence, query, runtime, remove)
}
