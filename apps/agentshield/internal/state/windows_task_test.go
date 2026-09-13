package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"siq-agent-security/apps/agentshield/internal/signing"
	"strings"
	"testing"
)

func windowsTaskFixture(t *testing.T) (*Store, *Writer, *signing.Key, []byte) {
	t.Helper()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := AcquireWriter(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Release() })
	if _, err := st.Initialize(w, 0); err != nil {
		t.Fatal(err)
	}
	key, _ := signing.FromSeed(bytes.Repeat([]byte{7}, 32))
	taskXML, err := os.ReadFile("../../testdata/contracts/windows-task.sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	return st, w, key, taskXML
}
func TestWindowsTaskOwnershipAndRecovery(t *testing.T) {
	st, w, key, taskXML := windowsTaskFixture(t)
	if _, err := st.PrepareWindowsTask(nil, key, taskXML, "S-1-5-21-100-200-300-1001"); err == nil {
		t.Fatal("missing writer accepted")
	}
	record, err := st.PrepareWindowsTask(w, key, taskXML, "S-1-5-21-100-200-300-1001")
	if err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{"S-1-5-21-100-200-300-1002", "S-1-5-18", "invalid"} {
		if _, err := st.VerifyWindowsTask(key, taskXML, sid); err == nil {
			t.Fatal("foreign user verified")
		}
		if _, err := st.PrepareWindowsTask(w, key, taskXML, sid); err == nil {
			t.Fatal("foreign user adopted")
		}
	}
	path := filepath.Join(st.Dir, strings.TrimPrefix(record.TaskName, `\`)+".xml")
	if _, err := st.PrepareWindowsTask(w, key, taskXML, "S-1-5-21-100-200-300-1001"); err != nil {
		t.Fatal("repeat preparation", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := st.VerifyWindowsTask(key, taskXML, "S-1-5-21-100-200-300-1001"); err == nil {
		t.Fatal("missing configuration verified")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("read verification repaired file")
	}
	if _, err := st.PrepareWindowsTask(w, key, taskXML, "S-1-5-21-100-200-300-1001"); err != nil {
		t.Fatal("recover missing configuration", err)
	}
	if err := os.WriteFile(path, []byte("user changes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PrepareWindowsTask(w, key, taskXML, "S-1-5-21-100-200-300-1001"); err == nil {
		t.Fatal("drift overwritten")
	}
	if raw, _ := os.ReadFile(path); string(raw) != "user changes" {
		t.Fatal("user data changed")
	}
	if _, err := st.VerifyWindowsTask(key, taskXML, "S-1-5-21-100-200-300-1001"); err == nil {
		t.Fatal("drift verified")
	}
	if _, err := st.VerifyUserService(key, taskXML); err == nil {
		t.Fatal("macOS record accepted as Linux ownership")
	}
}
func TestWindowsTaskRejectsUnownedAndClonedRecords(t *testing.T) {
	st, w, key, taskXML := windowsTaskFixture(t)
	expected, err := st.expectedWindowsTask(key, taskXML, "S-1-5-21-100-200-300-1001")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(st.Dir, strings.TrimPrefix(expected.TaskName, `\`)+".xml")
	if err := os.WriteFile(path, taskXML, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PrepareWindowsTask(w, key, taskXML, "S-1-5-21-100-200-300-1001"); err == nil {
		t.Fatal("unowned configuration adopted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	record, err := st.PrepareWindowsTask(w, key, taskXML, "S-1-5-21-100-200-300-1001")
	if err != nil {
		t.Fatal(err)
	}
	clone, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"local-instance.json", "windows-task.json", strings.TrimPrefix(record.TaskName, `\`) + ".xml"} {
		raw, err := os.ReadFile(filepath.Join(st.Dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(clone.Dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := clone.VerifyWindowsTask(key, taskXML, "S-1-5-21-100-200-300-1001"); err == nil {
		t.Fatal("cloned directory accepted")
	}
	record.XMLSHA256 = strings.Repeat("0", 64)
	raw, _ := json.Marshal(record)
	if err := os.WriteFile(filepath.Join(st.Dir, "windows-task.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.VerifyWindowsTask(key, taskXML, "S-1-5-21-100-200-300-1001"); err == nil {
		t.Fatal("tampered signature accepted")
	}
}
func TestWindowsTaskRecordContract(t *testing.T) {
	st, w, key, taskXML := windowsTaskFixture(t)
	r, err := st.PrepareWindowsTask(w, key, taskXML, "S-1-5-21-100-200-300-1001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.VerifyWindowsTask(key, taskXML, "S-1-5-21-100-200-300-1001"); err != nil {
		t.Fatal(err)
	}
	r.InstanceID = strings.Repeat("a", 64)
	r.DirectoryID = strings.Repeat("b", 64)
	r.TaskName = "\\SIQ-Agent-Security-" + r.InstanceID
	r.Signature, err = key.SignCanonical(r.unsigned())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	path := "../../testdata/contracts/local-windows-task-record.json"
	if os.Getenv("SIQ_UPDATE_WINDOWS_TASK_RECORD_FIXTURE") == "1" {
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, expected) {
		t.Fatal("contract fixture drift")
	}
}
