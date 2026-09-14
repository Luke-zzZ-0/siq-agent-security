package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestWindowsTaskReadbackConfiguration(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/contracts/windows-task.sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for name, actual := range map[string]string{
		"identical":       source,
		"formatting":      strings.ReplaceAll(source, "\n  ", "\r\n\t"),
		"comments":        strings.Replace(source, "<Settings>", "<!-- readback --><Settings>", 1),
		"attribute order": strings.Replace(source, `version="1.3" xmlns="`+windowsTaskNamespace+`"`, `xmlns="`+windowsTaskNamespace+`" version="1.3"`, 1),
		"setting order":   strings.Replace(source, "<Enabled>true</Enabled><Hidden>false</Hidden>", "<Hidden>false</Hidden><Enabled>true</Enabled>", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := verifyWindowsTaskXML([]byte(actual), raw); err != nil {
				t.Fatal(err)
			}
		})
	}
	for name, actual := range map[string]string{
		"elevation":           strings.Replace(source, "LeastPrivilege", "HighestAvailable", 1),
		"other user":          strings.Replace(source, "1001</UserId>", "1002</UserId>", 1),
		"logon type":          strings.Replace(source, "InteractiveToken", "ServiceAccount", 1),
		"force stop":          strings.Replace(source, "<AllowHardTerminate>false", "<AllowHardTerminate>true", 1),
		"extra action":        strings.Replace(source, "</Actions>", "<Exec><Command>evil.exe</Command></Exec></Actions>", 1),
		"trigger":             strings.Replace(source, "</Task>", "<Triggers><LogonTrigger/></Triggers></Task>", 1),
		"unknown setting":     strings.Replace(source, "</Settings>", "<Unknown>true</Unknown></Settings>", 1),
		"missing setting":     strings.Replace(source, "<Hidden>false</Hidden>", "", 1),
		"duplicate setting":   strings.Replace(source, "</Settings>", "<Hidden>false</Hidden></Settings>", 1),
		"duplicate attribute": strings.Replace(source, `version="1.3"`, `version="1.3" version="1.3"`, 1),
		"unknown attribute":   strings.Replace(source, `version="1.3"`, `version="1.3" other="value"`, 1),
		"namespace":           strings.ReplaceAll(source, windowsTaskNamespace, "urn:foreign"),
		"leaf whitespace":     strings.Replace(source, "<Command>", "<Command> ", 1),
		"argument change":     strings.Replace(source, "serve --state-dir", "serve --port 9999 --state-dir", 1),
		"mixed content":       strings.Replace(source, "<Settings>", "<Settings>ignored?", 1),
		"DTD":                 strings.Replace(source, "<Task ", "<!DOCTYPE Task><Task ", 1),
		"instruction":         source + "<?other value?>",
		"second root":         source + source,
		"truncated":           strings.TrimSuffix(strings.TrimSpace(source), "</Task>"),
		"trailing data":       source + "bad",
		"size":                source + strings.Repeat(" ", 64*1024),
	} {
		t.Run(name, func(t *testing.T) {
			if actual == source {
				t.Fatal("negative fixture did not change")
			}
			if err := verifyWindowsTaskXML([]byte(actual), raw); err == nil {
				t.Fatal("accepted changed or invalid configuration")
			}
		})
	}
}

func TestWindowsTaskXMLBudgets(t *testing.T) {
	wrap := func(body string) []byte {
		return []byte(`<Task xmlns="` + windowsTaskNamespace + `">` + body + `</Task>`)
	}
	// Root plus nested elements: exactly 16 succeeds, 17 fails.
	for _, depth := range []int{16, 17} {
		_, err := parseWindowsTaskXML(wrap(strings.Repeat("<N>", depth-1) + strings.Repeat("</N>", depth-1)))
		if (err == nil) != (depth == 16) {
			t.Fatalf("depth %d: %v", depth, err)
		}
	}
	base := wrap("")
	for _, size := range []int{64 * 1024, 64*1024 + 1} {
		_, err := parseWindowsTaskXML(append(base, []byte(strings.Repeat(" ", size-len(base)))...))
		if (err == nil) != (size == 64*1024) {
			t.Fatalf("size %d: %v", size, err)
		}
	}
	for _, count := range []int{256, 257} {
		var children strings.Builder
		for i := 1; i < count; i++ {
			fmt.Fprintf(&children, "<N%d/>", i)
		}
		_, err := parseWindowsTaskXML(wrap(children.String()))
		if (err == nil) != (count == 256) {
			t.Fatalf("elements %d: %v", count, err)
		}
	}
}

func TestWindowsTaskNativeReadbackDefaults(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/contracts/windows-task.sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	actual := strings.Replace(source, "</RegistrationInfo>", "</RegistrationInfo><Triggers />", 1)
	actual = strings.Replace(actual, "</Settings>", "<DisallowStartOnRemoteAppSession>false</DisallowStartOnRemoteAppSession></Settings>", 1)
	if err := verifyWindowsTaskXML([]byte(actual), raw); err != nil {
		t.Fatal("native explicit defaults rejected", err)
	}
	for name, changed := range map[string]string{
		"login trigger":                strings.Replace(actual, "<Triggers />", "<Triggers><LogonTrigger/></Triggers>", 1),
		"trigger attribute":            strings.Replace(actual, "<Triggers />", `<Triggers enabled="true" />`, 1),
		"trigger text":                 strings.Replace(actual, "<Triggers />", "<Triggers>unknown</Triggers>", 1),
		"remote app changed":           strings.Replace(actual, "<DisallowStartOnRemoteAppSession>false", "<DisallowStartOnRemoteAppSession>true", 1),
		"unified engine changed":       strings.Replace(actual, "<UseUnifiedSchedulingEngine>true", "<UseUnifiedSchedulingEngine>false", 1),
		"setting attribute":            strings.Replace(actual, "<UseUnifiedSchedulingEngine>", `<UseUnifiedSchedulingEngine other="false">`, 1),
		"setting child":                strings.Replace(actual, "<UseUnifiedSchedulingEngine>true", "<UseUnifiedSchedulingEngine><Unknown/>", 1),
		"unknown default":              strings.Replace(actual, "</Settings>", "<Unknown>false</Unknown></Settings>", 1),
		"duplicate default":            strings.Replace(actual, "</Settings>", "<UseUnifiedSchedulingEngine>false</UseUnifiedSchedulingEngine></Settings>", 1),
		"elevation alongside defaults": strings.Replace(actual, "LeastPrivilege", "HighestAvailable", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if verifyWindowsTaskXML([]byte(changed), raw) == nil {
				t.Fatal("changed native configuration accepted")
			}
		})
	}
	// An explicit setting in the source must still be present in the readback.
	if verifyWindowsTaskXML(raw, []byte(actual)) == nil {
		t.Fatal("missing explicitly signed settings accepted")
	}
}

func TestWindowsTaskXMLRejectsAmbiguousNamespaceAndEncoding(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/contracts/windows-task.sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for name, actual := range map[string]string{
		"foreign leaf":                         strings.Replace(source, "<Command>", `<Command xmlns="urn:foreign">`, 1),
		"unbound prefix":                       strings.Replace(source, "<Command>", "<q:Command>", 1),
		"namespaced extra attribute":           strings.Replace(source, "<Actions ", `<Actions xmlns:q="urn:foreign" q:Context="LocalUser" `, 1),
		"duplicate namespace":                  strings.Replace(source, `<Task `, `<Task xmlns="`+windowsTaskNamespace+`" `, 1),
		"invalid UTF8":                         strings.Replace(source, "<Command>", "<Command>\xff", 1),
		"UTF16 declaration without conversion": strings.Replace(source, "UTF-8", "UTF-16", 1),
		"second declaration":                   source + `<?xml version="1.0"?>`,
	} {
		t.Run(name, func(t *testing.T) {
			if actual == source || verifyWindowsTaskXML([]byte(actual), raw) == nil {
				t.Fatal("invalid configuration was not rejected")
			}
		})
	}
}
