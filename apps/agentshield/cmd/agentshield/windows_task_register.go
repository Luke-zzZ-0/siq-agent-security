package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

//go:embed windows_task_register.ps1
var windowsTaskRegisterScript string

func cmdWindowsTaskRegister(args []string, out io.Writer) error {
	if len(args) != 1 || args[0] != "--confirm-register" {
		return errors.New("task-register: --confirm-register required, no other arguments accepted")
	}
	return withPreparedWindowsTask(nil, func(st *state.Store, key *signing.Key, expected []byte, record state.WindowsTaskRecord) error {
		return registerOwnedWindowsTask(st, key, expected, record.UserSID, runWindowsTaskPresence, runWindowsTaskQuery, runWindowsTaskCreate, out)
	})
}

// The command owns both lifecycle and daemon writers throughout this operation.
func registerOwnedWindowsTask(st *state.Store, key *signing.Key, expected []byte, sid string, presence func(string, string) (bool, error), query func(string) ([]byte, error), create func(string, string, []byte) error, out io.Writer) error {
	if err := st.CheckServiceSwitchPending(); err != nil {
		return err
	}
	record, err := st.VerifyWindowsTask(key, expected, sid)
	if err != nil {
		return err
	}
	present, err := presence(record.TaskName, sid)
	if err != nil {
		return errors.New("task-register: task presence is unconfirmed")
	}
	if !present {
		if _, err := st.VerifyWindowsTask(key, expected, sid); err != nil {
			return err
		}
		if err := create(record.TaskName, sid, expected); err != nil {
			return errors.New("task-register: exclusive creation failed; preserve state and inspect before retrying")
		}
	}
	return queryOwnedWindowsTask(st, key, expected, sid, query, out)
}

func runWindowsTaskCreate(name, sid string, expected []byte) error {
	input, err := windowsTaskCreationInput(name, sid, expected)
	if err != nil {
		return err
	}
	stdout, err := runWindowsTaskScript(windowsTaskRegisterScript, input)
	if err != nil || stdout != "SIQ_TASK_CREATED" {
		return errors.New("task-register: system creation not confirmed")
	}
	return nil
}

func windowsTaskCreationInput(name, sid string, expected []byte) ([]byte, error) {
	// RegisterTask receives a Unicode BSTR rather than the signed UTF-8 bytes.
	// Preserve that signed source and remove only its exact transport declaration;
	// a UTF-8 declaration on a BSTR makes the native XML parser reject the task.
	const declaration = `<?xml version="1.0" encoding="UTF-8"?>`
	if !strings.HasPrefix(string(expected), declaration) {
		return nil, errors.New("task-register: unsupported source encoding")
	}
	return json.Marshal(map[string]string{"task_name": name, "user_sid": sid, "task_xml": strings.TrimPrefix(string(expected), declaration)})
}
