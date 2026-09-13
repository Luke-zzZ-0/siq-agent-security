package stateformat

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestV2StrictContract(t *testing.T) {
	raw, e := os.ReadFile("../../testdata/contracts/local-state-format-v2.json")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	var expected, actual any
	json.Unmarshal(raw, &expected)
	round, _ := json.Marshal(m)
	json.Unmarshal(round, &actual)
	if !reflect.DeepEqual(expected, actual) {
		t.Fatal("fixture drift")
	}
	for _, bad := range []string{
		strings.Replace(string(raw), `"min_writer": 2`, `"min_writer": 2,"min_writer":1`, 1),
		strings.Replace(string(raw), `"min_writer": 2`, `"MIN_WRITER": 2`, 1),
		strings.Replace(string(raw), `"min_writer": 2`, `"min_writer": null`, 1),
		strings.Replace(string(raw), `"min_writer": 2`, `"min_writer": 1`, 1),
		string(raw) + ` {}`, strings.Replace(string(raw), `"format_version": 2`, `"format_version": 1`, 1),
	} {
		if _, e := Decode([]byte(bad)); e == nil {
			t.Fatal("invalid marker accepted")
		}
	}
}
