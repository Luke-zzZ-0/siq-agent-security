package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os/user"
	"strings"
	"testing"
)

// Only creates an in-memory TaskDefinition. It never registers or runs a task.
func TestWindowsTaskNativeDefinitionRoundtrip(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := renderWindowsTask(`C:\SIQ 中文\agent.exe`, `C:\SIQ 中文\state folder`, strings.Repeat("a", 64), current.Uid)
	if err != nil {
		t.Fatal(err)
	}
	input, err := windowsTaskCreationInput(`\SIQ-Agent-Security-`+strings.Repeat("a", 64), current.Uid, []byte(expected))
	if err != nil {
		t.Fatal(err)
	}
	const script = `
$ErrorActionPreference = 'Stop'
try {
    [Console]::InputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false, $true)
    $request = ConvertFrom-Json -InputObject ([Console]::In.ReadToEnd())
    $service = New-Object -ComObject Schedule.Service
    $service.Connect()
    $definition = $service.NewTask(0)
    $definition.XmlText = $request.task_xml
    $result = $definition.XmlText
    # The result is now transported as UTF-8, not the COM Unicode BSTR.
    [Console]::Out.Write($result.Substring($result.IndexOf('?>') + 2))
} catch {
    [Console]::Error.Write('native task definition rejected')
    exit 1
}`
	actual, err := runWindowsTaskScript(script, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyWindowsTaskXML([]byte(actual), []byte(expected)); err != nil {
		t.Fatal(err)
	}
	// The old UTF-8 declaration on a Unicode BSTR must be rejected by the
	// actual native parser; this guards the transport conversion regression.
	oldInput, err := json.Marshal(map[string]string{"task_xml": expected})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runWindowsTaskScript(script, oldInput); err == nil {
		t.Fatal("native parser unexpectedly accepted mismatched BSTR encoding")
	}
}

func TestWindowsTaskNativeAbsentAndStrictTransport(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal(err)
	}
	present, err := runWindowsTaskPresence(`\SIQ-Agent-Security-`+hex.EncodeToString(id[:]), current.Uid)
	if err != nil || present {
		t.Fatalf("fresh task absence not confirmed: %v", err)
	}
	if _, err := runWindowsTaskPresence(`\SIQ-Agent-Security-`+hex.EncodeToString(id[:]), "S-1-5-21-0-0-0-1000"); err == nil {
		t.Fatal("wrong principal accepted as task absence")
	}
	if _, err := runWindowsTaskScript(`[Console]::Error.Write('failure'); [Console]::Out.Write('SIQ_TASK_ABSENT')`, nil); err == nil {
		t.Fatal("stderr accepted as a valid task result")
	}
}
