package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"os/user"

	"siq-agent-security/apps/agentshield/internal/state"
)

//go:embed windows_task_preflight.ps1
var windowsTaskPreflightScript string

func nativeWindowsSetup() userSetupActions {
	actions := nativeLocalSetup()
	actions.preflight = preflightWindowsTask
	actions.register = func() error { return cmdWindowsTaskRegister([]string{"--confirm-register"}, io.Discard) }
	actions.start = func() error { return cmdWindowsTaskStart([]string{"--confirm-start"}, io.Discard) }
	return actions
}

func preflightWindowsTask() error {
	current, err := user.Current()
	if err != nil || !state.WindowsUserSIDValid(current.Uid) {
		return errors.New("task: current Windows user not confirmed")
	}
	input, err := json.Marshal(map[string]string{"user_sid": current.Uid})
	if err != nil {
		return err
	}
	output, err := runWindowsTaskScript(windowsTaskPreflightScript, input)
	if err != nil || output != "SIQ_TASK_MANAGER_READY" {
		return errors.New("task: current user task scheduler unavailable")
	}
	return nil
}

func setupWindowsTask(port int, explicit, openUI bool, out io.Writer, actions userSetupActions) error {
	return setupUserService(port, explicit, openUI, out, actions, "Windows 当前用户任务服务", "task-runtime/task-query")
}
