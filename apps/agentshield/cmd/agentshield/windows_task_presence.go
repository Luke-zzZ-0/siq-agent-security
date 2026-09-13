package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"time"
	"unicode/utf16"

	"siq-agent-security/apps/agentshield/internal/signing"
	"siq-agent-security/apps/agentshield/internal/state"
)

//go:embed windows_task_presence.ps1
var windowsTaskPresenceScript string

func inspectOwnedWindowsTask(st *state.Store, key *signing.Key, expected []byte, sid string, presence func(string, string) (bool, error), query func(string) ([]byte, error), out io.Writer) error {
	record, err := st.VerifyWindowsTask(key, expected, sid)
	if err != nil {
		return err
	}
	present, err := presence(record.TaskName, record.UserSID)
	if err != nil {
		return errors.New("task-presence: task presence is unconfirmed")
	}
	result := "absent\n"
	if present {
		if err := queryOwnedWindowsTask(st, key, expected, sid, query, io.Discard); err != nil {
			return err
		}
		result = "present\n"
	}
	if _, err := st.VerifyWindowsTask(key, expected, sid); err != nil {
		return err
	}
	_, err = io.WriteString(out, result)
	return err
}

func runWindowsTaskPresence(name, sid string) (bool, error) {
	input, err := json.Marshal(map[string]string{"task_name": name, "user_sid": sid})
	if err != nil {
		return false, err
	}
	stdout, err := runWindowsTaskScript(windowsTaskPresenceScript, input)
	return windowsTaskPresenceResult(stdout, "", err)
}

func runWindowsTaskScript(script string, input []byte) (string, error) {
	taskExe, err := windowsTaskExecutable()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	executable := filepath.Join(filepath.Dir(taskExe), "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.CommandContext(ctx, executable, "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodeWindowsPowerShell(script))
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr serviceOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second
	err = cmd.Run()
	if err != nil || stderr.Len() != 0 {
		return "", errors.New("task: system script failed or timed out")
	}
	return stdout.String(), nil
}

func windowsTaskPresenceResult(stdout, stderr string, err error) (bool, error) {
	if err != nil || stderr != "" {
		return false, errors.New("task-presence: system query failed")
	}
	switch stdout {
	case "SIQ_TASK_PRESENT":
		return true, nil
	case "SIQ_TASK_ABSENT":
		return false, nil
	default:
		return false, errors.New("task-presence: invalid system response")
	}
}

func encodeWindowsPowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(units))
	for i, u := range units {
		b[2*i], b[2*i+1] = byte(u), byte(u>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}
