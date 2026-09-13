package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

//go:embed windows_task_unregister.ps1
var windowsTaskUnregisterScript string

func cmdWindowsTaskUnregister(args []string, out io.Writer) error {
	if len(args) != 1 || args[0] != "--confirm-unregister" {
		return errors.New("task-unregister: --confirm-unregister required, no other arguments accepted")
	}
	err := withWindowsTaskIdentity(nil, func(st *state.Store, key *signing.Key, expected []byte, sid string) (resultErr error) {
		lifecycle, err := state.AcquireWriter(filepath.Join(st.Dir, "service-control"))
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, lifecycle.Release()) }()
		writer, err := state.AcquireWriter(st.Dir)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, writer.Release()) }()
		return unregisterOwnedWindowsTask(st, key, expected, sid, runWindowsTaskPresence, runWindowsTaskQuery, runWindowsTaskRuntime, runWindowsTaskDelete)
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, "当前实例任务已确认注销；本地配置、密钥和历史数据已保留。\n")
	return err
}

// Caller holds both lifecycle and primary writers until final absence is verified.
func unregisterOwnedWindowsTask(st *state.Store, key *signing.Key, expected []byte, sid string, presence func(string, string) (bool, error), query func(string) ([]byte, error), runtime func(string, string) (string, error), remove func(string, string, []byte) error) error {
	if err := st.CheckServiceSwitchPending(); err != nil {
		return err
	}
	record, err := st.VerifyWindowsTask(key, expected, sid)
	if err != nil {
		return err
	}
	present, err := presence(record.TaskName, sid)
	if err != nil {
		return errors.New("task-unregister: task presence is unconfirmed")
	}
	if present {
		var snapshot []byte
		capture := func(name string) ([]byte, error) {
			raw, err := query(name)
			if err != nil {
				return nil, err
			}
			snapshot, err = decodeWindowsTaskOutput(raw)
			return raw, err
		}
		current, err := readOwnedWindowsTaskRuntime(st, key, expected, sid, capture, runtime)
		if err != nil {
			return err
		}
		if current.State != "ready" {
			return errors.New("task-unregister: task is not idle; stop it first")
		}
		if _, err := st.VerifyWindowsTask(key, expected, sid); err != nil {
			return err
		}
		if err := remove(record.TaskName, sid, snapshot); err != nil {
			return errors.New("task-unregister: deletion unconfirmed; preserve state and inspect before retrying")
		}
	}
	present, err = presence(record.TaskName, sid)
	if err != nil || present {
		return errors.New("task-unregister: final absence not confirmed")
	}
	_, err = st.VerifyWindowsTask(key, expected, sid)
	return err
}

func runWindowsTaskDelete(name, sid string, snapshot []byte) error {
	input, err := json.Marshal(map[string]string{"task_name": name, "user_sid": sid, "task_xml": string(snapshot)})
	if err != nil {
		return err
	}
	stdout, err := runWindowsTaskScript(windowsTaskUnregisterScript, input)
	if err != nil || stdout != "SIQ_TASK_DELETED" {
		return errors.New("task-unregister: system deletion not confirmed")
	}
	return nil
}
