package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
)

// Synthetic decoder input tests the receipt boundary only. It neither invokes
// nativeProbe nor constitutes evidence of actual GPU timeout/cancellation.
func receiptForDecoder(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(report{Schema: 1, PID: 42, Software: true, InitialDrain: true, Held: true, Submitted: 1, DrainMS: 5000, FenceBefore: 2, SubmittedFence: 3, FenceAfter: math.MaxUint64, RemovedReason: "0x887a0005", FirstPreserved: true, Retired: true, FirstError: "GPU fence did not complete within five seconds", Pass: true})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReceiptRejectsReplayedPIDAndMalformedOutput(t *testing.T) {
	data := receiptForDecoder(t)
	if _, err := decodeReport(data, 42); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		data []byte
		pid  int
	}{
		{"copiedPID", data, 43},
		{"missingOwner", data, 0},
		{"truncated", data[:len(data)-1], 42},
		{"trailingReceipt", append(append([]byte(nil), data...), data...), 42},
		{"unknownField", append([]byte(`{"unexpected":true,`), data[1:]...), 42},
		{"oversized", bytes.Repeat([]byte(" "), maxReportBytes+1), 42},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeReport(tt.data, tt.pid); err == nil {
				t.Fatal("invalid receipt admitted")
			}
		})
	}
}

func TestBoundedChildOutputKeepsDrainingAfterLimit(t *testing.T) {
	output := &boundedOutput{limit: 4}
	if n, err := output.Write([]byte("abc")); err != nil || n != 3 {
		t.Fatalf("first write %d %v", n, err)
	}
	if n, err := output.Write([]byte("defgh")); err != nil || n != 5 {
		t.Fatalf("overflow write %d %v", n, err)
	}
	if n, err := output.Write([]byte("more")); err != nil || n != 4 {
		t.Fatalf("continued drain %d %v", n, err)
	}
	if output.String() != "abcd" || !output.overflow {
		t.Fatalf("unbounded or unmarked output: %q %v", output.String(), output.overflow)
	}
}

// These retained bytes came from the real Device fixture's owned child227032
// on 2026-10-08. Replaying them verifies only this consumer, never a new GPU run.
func retainedNativeReceipt(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/timeout-native-20261008.json")
	if err != nil {
		t.Fatal(err)
	}
	if digest := fmt.Sprintf("%x", sha256.Sum256(data)); digest != "9250bc32d5d9e4540c8786a8c606afa1d22b768d564fe76b1a4ce461e238b3f2" {
		t.Fatalf("retained actual receipt bytes changed: %s", digest)
	}
	return data
}

func TestReceiptReplaysRetainedNativeScalars(t *testing.T) {
	r, err := decodeReport(retainedNativeReceipt(t), 227032)
	if err != nil {
		t.Fatal(err)
	}
	if r.FenceAfter != math.MaxUint64 || r.DrainMS != 5000 || r.TerminalMS != 0 || r.DestructMS != 15 {
		t.Fatalf("actual native scalar values changed during replay: %+v", r)
	}
}

func TestReceiptRequiresEveryProducerFieldOnceAndNonNull(t *testing.T) {
	data := retainedNativeReceipt(t)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, mutation := range []string{"missing", "null", "duplicate"} {
			t.Run(name+"/"+mutation, func(t *testing.T) {
				changed := make(map[string]json.RawMessage, len(fields))
				for key, value := range fields {
					changed[key] = value
				}
				switch mutation {
				case "missing":
					delete(changed, name)
				case "null":
					changed[name] = json.RawMessage("null")
				}
				encoded, err := json.Marshal(changed)
				if err != nil {
					t.Fatal(err)
				}
				if mutation == "duplicate" {
					encoded = []byte(fmt.Sprintf("{%q:%s,%s", name, fields[name], encoded[1:]))
				}
				if _, err := decodeReport(encoded, 227032); err == nil || !strings.Contains(err.Error(), mutation+" timeout report field") {
					t.Fatalf("producer %s field %s admitted or rejected for wrong reason: %v", mutation, name, err)
				}
			})
		}
	}
}

func TestReceiptRejectsFenceAndRemovedReasonTypeCorruption(t *testing.T) {
	data := retainedNativeReceipt(t)
	cases := []struct {
		name, from, to string
	}{
		{"negativeFence", `"fenceAfter":18446744073709551615`, `"fenceAfter":-1`},
		{"overflowFence", `"fenceAfter":18446744073709551615`, `"fenceAfter":18446744073709551616`},
		{"fractionalFence", `"fenceAfter":18446744073709551615`, `"fenceAfter":18446744073709551615.5`},
		{"stringFence", `"fenceAfter":18446744073709551615`, `"fenceAfter":"18446744073709551615"`},
		{"numberBoolean", `"terminalWait":false`, `"terminalWait":0`},
		{"stringBoolean", `"terminalWait":false`, `"terminalWait":"false"`},
		{"booleanFailure", `"failure":""`, `"failure":false`},
		{"caseAlias", `"pid":227032`, `"PID":227032`},
		{"malformedReason", `"removedReason":"0x887a0005"`, `"removedReason":"0xnot-an-hresult"`},
		{"successfulReason", `"removedReason":"0x887a0005"`, `"removedReason":"0x00000000"`},
		{"overflowReason", `"removedReason":"0x887a0005"`, `"removedReason":"0x1887a0005"`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if bytes.Count(data, []byte(tt.from)) != 1 {
				t.Fatal("retained receipt mutation is not exact")
			}
			changed := bytes.Replace(data, []byte(tt.from), []byte(tt.to), 1)
			if _, err := decodeReport(changed, 227032); err == nil {
				t.Fatal("corrupt producer scalar admitted")
			}
		})
	}
}
