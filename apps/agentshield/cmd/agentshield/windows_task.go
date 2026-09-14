package main

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os/user"
	"runtime"
	"siq-agent-security/apps/agentshield/internal/state"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func windowsTaskPathValid(value string) bool {
	if strings.Contains(value, "$(") || !utf8.ValidString(value) || len(utf16.Encode([]rune(value))) > 260 || len(value) < 4 || value[1:3] != `:\` || !((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) {
		return false
	}
	for _, component := range strings.Split(value[3:], `\`) {
		if component == "" || component == "." || component == ".." || strings.HasSuffix(component, " ") || strings.HasSuffix(component, ".") || strings.ContainsAny(component, `/:"<>|?*%`) {
			return false
		}
		for _, r := range component {
			if unicode.IsControl(r) || r == 0xfffe || r == 0xffff {
				return false
			}
		}
	}
	return true
}

func renderWindowsTask(binary, directory, instanceID, sid string) (string, error) {
	id, err := hex.DecodeString(instanceID)
	if err != nil || len(id) != 32 || hex.EncodeToString(id) != instanceID || !state.WindowsUserSIDValid(sid) {
		return "", errors.New("task: invalid instance or user identity")
	}
	if !windowsTaskPathValid(binary) || !windowsTaskPathValid(directory) {
		return "", errors.New("task: canonical local Windows paths without environment expansion required")
	}
	escape := func(value string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(value))
		return b.String()
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.3" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><URI>\SIQ-Agent-Security-%s</URI></RegistrationInfo>
  <Principals><Principal id="LocalUser"><UserId>%s</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>false</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings><StopOnIdleEnd>false</StopOnIdleEnd><RestartOnIdle>false</RestartOnIdle></IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled><Hidden>false</Hidden><RunOnlyIfIdle>false</RunOnlyIfIdle>
    <UseUnifiedSchedulingEngine>true</UseUnifiedSchedulingEngine>
    <WakeToRun>false</WakeToRun><ExecutionTimeLimit>PT0S</ExecutionTimeLimit><Priority>7</Priority>
  </Settings>
  <Actions Context="LocalUser"><Exec><Command>%s</Command><Arguments>%s</Arguments></Exec></Actions>
</Task>
`, instanceID, escape(sid), escape(binary), escape(`serve --state-dir "`+directory+`"`)), nil
}

func cmdTaskXML(args []string, out io.Writer) error {
	if len(args) != 0 || runtime.GOOS != "windows" {
		return errors.New("task-xml: Windows-only read-only export, no arguments expected")
	}
	dir, binary, err := currentServicePaths()
	if err != nil {
		return err
	}
	instance, err := (&state.Store{Dir: dir}).ReadLocalInstance()
	if err != nil {
		return err
	}
	current, err := user.Current()
	if err != nil {
		return errors.New("task-xml: current Windows user identity unavailable")
	}
	raw, err := renderWindowsTask(binary, dir, instance.InstanceID, current.Uid)
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, raw)
	return err
}
