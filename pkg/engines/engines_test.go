package engines

import (
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/startracex/wangpkg/pkg/constants"
)

func TestInfoCheck(t *testing.T) {
	info := &Info{Name: "node", Version: semver.MustParse("20.5.0")}
	if err := info.Check(">=18"); err != nil {
		t.Errorf("matching constraint returned error: %v", err)
	}
	if err := info.Check(">=22"); err == nil {
		t.Error("non-matching constraint should return error")
	}
	if err := info.Check("not-a-range"); err == nil {
		t.Error("invalid constraint should return error")
	}

	wantErr := errors.New("probe failed")
	errInfo := &Info{Err: wantErr}
	if err := errInfo.Check(">=1"); !errors.Is(err, wantErr) {
		t.Errorf("info with Err should return it, got %v", err)
	}
}

func TestEnginesCheck(t *testing.T) {
	// Self app engine is available without spawning a process.
	ok := Engines{constants.AppName: constants.Version}
	errs, err := ok.Check()
	if err != nil || len(errs) != 0 {
		t.Errorf("self engine check failed: errs=%v err=%v", errs, err)
	}

	bad := Engines{"does-not-exist": "1.0.0"}
	if _, err := bad.Check(); err == nil {
		t.Error("unknown engine name should produce an error")
	}
}

func TestOnFailError(t *testing.T) {
	base := errors.New("boom")
	if got := onFailError("", base); !errors.Is(got, base) {
		t.Error("empty onFail should default to error and return err")
	}
	if got := onFailError("error", base); !errors.Is(got, base) {
		t.Error("error should return err")
	}
	if got := onFailError("warn", base); got != nil {
		t.Error("warn should return nil")
	}
	if got := onFailError("ignore", base); got != nil {
		t.Error("ignore should return nil")
	}
	if got := onFailError("bogus", base); got == nil {
		t.Error("unknown onFail should return an error")
	}
}

func TestCheckIncludes(t *testing.T) {
	// No requirements: nothing to check.
	if err := checkIncludes(nil, "x64", &[]error{}); err != nil {
		t.Errorf("empty requirements: %v", err)
	}

	// Current value matches.
	var errs []error
	match := Flex[DevEnginesElement]{{Name: "x64"}}
	if err := checkIncludes(match, "x64", &errs); err != nil || len(errs) != 0 {
		t.Errorf("matching include: err=%v errs=%v", err, errs)
	}

	// Mismatch with error policy: hard failure.
	var hardErrs []error
	hard := Flex[DevEnginesElement]{{Name: "arm64", OnFail: "error"}}
	if err := checkIncludes(hard, "x64", &hardErrs); err == nil {
		t.Error("mismatch with error policy should fail")
	}

	// Mismatch with ignore policy: no error and no recorded warnings.
	var ignoreErrs []error
	ignore := Flex[DevEnginesElement]{{Name: "arm64", OnFail: "ignore"}}
	if err := checkIncludes(ignore, "x64", &ignoreErrs); err != nil || len(ignoreErrs) != 0 {
		t.Errorf("ignore policy: err=%v errs=%v", err, ignoreErrs)
	}

	// Mismatch with warn policy: no hard error but a warning recorded.
	var warnErrs []error
	warn := Flex[DevEnginesElement]{{Name: "arm64", OnFail: "warn"}}
	if err := checkIncludes(warn, "x64", &warnErrs); err != nil || len(warnErrs) != 1 {
		t.Errorf("warn policy: err=%v errs=%v", err, warnErrs)
	}
}

func TestFlexUnmarshal(t *testing.T) {
	var arr Flex[string]
	if err := json.Unmarshal([]byte(`["a","b"]`), &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) != 2 || arr[0] != "a" || arr[1] != "b" {
		t.Errorf("array form = %v", arr)
	}

	var single Flex[string]
	if err := json.Unmarshal([]byte(`"a"`), &single); err != nil {
		t.Fatal(err)
	}
	if len(single) != 1 || single[0] != "a" {
		t.Errorf("single form = %v", single)
	}
}

func TestRecordUnmarshal(t *testing.T) {
	var rec Record[string]
	if err := json.Unmarshal([]byte(`{"a":"1"}`), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["a"] != "1" {
		t.Errorf("record = %v", rec)
	}

	// Non-object input clears the record.
	rec = Record[string]{"a": "1"}
	if err := json.Unmarshal([]byte(`[1,2]`), &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec) != 0 {
		t.Errorf("record after non-object = %v, want empty", rec)
	}
}
