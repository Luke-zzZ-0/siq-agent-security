package main

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"
)

func TestWindowsTaskFixtureAndBoundaries(t *testing.T) {
	binary := `C:\Program Files\SIQ & tools\siq.exe`
	dir := `C:\Users\example\SIQ 中文`
	id := strings.Repeat("a", 64)
	sid := "S-1-5-21-100-200-300-1001"
	raw, err := renderWindowsTask(binary, dir, id, sid)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/contracts/windows-task.sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(want) {
		t.Fatal("shared XML fixture mismatch")
	}
	var doc struct {
		XMLName   xml.Name
		Principal struct {
			UserID string `xml:"UserId"`
			Logon  string `xml:"LogonType"`
			Level  string `xml:"RunLevel"`
		} `xml:"Principals>Principal"`
		Exec struct {
			Command   string
			Arguments string
		} `xml:"Actions>Exec"`
	}
	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.XMLName.Space != "http://schemas.microsoft.com/windows/2004/02/mit/task" || doc.Principal.UserID != sid || doc.Principal.Level != "LeastPrivilege" || doc.Principal.Logon != "InteractiveToken" {
		t.Fatal("wrong principal")
	}
	if doc.Exec.Command != binary || doc.Exec.Arguments != `serve --state-dir "`+dir+`"` {
		t.Fatal("wrong invocation")
	}
	for _, path := range []string{`relative`, `C:relative`, `\\host\share\file`, `\\?\C:\file`, `C:\`, `C:\foo\..\bar`, `C:\foo.`, `C:\foo `, `C:\foo\`, `C:\%HOME%\file`, `C:\foo:stream`, `C:\a"b`, `C:\a<b`, `C:\a|b`, `C:\a?b`, "C:\\foo\nbar", "C:\\bad\xff", `C:\` + strings.Repeat("a", 258)} {
		if _, err := renderWindowsTask(path, dir, id, sid); err == nil {
			t.Fatal("accepted invalid path", path)
		}
	}
	if !windowsTaskPathValid(`C:\` + strings.Repeat("a", 257)) {
		t.Fatal("260-unit path rejected")
	}
	for _, bad := range []string{"", "administrator", "S-1-5-18", "S-1-5-19", "S-1-5-20", "S-1-05-21", "S-1-5-4294967296", "S-1-281474976710656-1", "S-1-5-+1", "S-1-5-1" + strings.Repeat("-1", 15)} {
		if _, err := renderWindowsTask(binary, dir, id, bad); err == nil {
			t.Fatal("invalid principal accepted", bad)
		}
	}
	for _, bad := range []string{"", strings.Repeat("A", 64), strings.Repeat("g", 64), strings.Repeat("a", 63)} {
		if _, err := renderWindowsTask(binary, dir, bad, sid); err == nil {
			t.Fatal("invalid instance accepted")
		}
	}
}
