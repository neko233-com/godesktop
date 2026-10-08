// Command dx12timeout validates the real production WARP timeout/cancellation
// path in one owned child process. It does not create a window or public UI API.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"time"
)

const maxReportBytes = 16 << 10

type report struct {
	Schema         int    `json:"schema"`
	PID            int    `json:"pid"`
	Software       bool   `json:"software"`
	Debug          bool   `json:"debug"`
	InitialDrain   bool   `json:"initialDrain"`
	Held           bool   `json:"held"`
	Submitted      uint64 `json:"actualSubmitted"`
	Completed      uint64 `json:"actualCompleted"`
	FirstDrain     bool   `json:"firstDrain"`
	DrainMS        uint64 `json:"drainMS"`
	FenceBefore    uint64 `json:"fenceBefore"`
	SubmittedFence uint64 `json:"submittedFence"`
	FenceAfter     uint64 `json:"fenceAfter"`
	RemovedReason  string `json:"removedReason"`
	TerminalDrain  bool   `json:"terminalDrain"`
	TerminalWait   bool   `json:"terminalWait"`
	TerminalSignal bool   `json:"terminalSignal"`
	TerminalSubmit bool   `json:"terminalSubmit"`
	FirstPreserved bool   `json:"firstErrorPreserved"`
	TerminalMS     uint64 `json:"terminalMS"`
	Retired        bool   `json:"retired"`
	RetireMS       uint64 `json:"retireMS"`
	DestructMS     uint64 `json:"destructMS"`
	FirstError     string `json:"firstError"`
	Failure        string `json:"failure"`
	Pass           bool   `json:"pass"`
}

type boundedOutput struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) > b.limit-b.Len() {
		b.overflow = true
		p = p[:b.limit-b.Len()]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}

// Validate the exact producer object before decoding typed values. A struct
// decoder alone accepts missing/null zero-valued fields and duplicate keys,
// which would erase evidence that every native check actually reported.
func validateReportFields(data []byte) error {
	schema := reflect.TypeOf(report{})
	required := make(map[string]struct{}, schema.NumField())
	for index := range schema.NumField() {
		required[schema.Field(index).Tag.Get("json")] = struct{}{}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	opening, err := d.Token()
	if err != nil {
		return err
	}
	if opening != json.Delim('{') {
		return errors.New("timeout report must be a JSON object")
	}
	seen := make(map[string]struct{}, len(required))
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return err
		}
		name, ok := key.(string)
		if !ok {
			return errors.New("timeout report field name is not a string")
		}
		if _, ok := required[name]; !ok {
			return fmt.Errorf("unknown timeout report field %q", name)
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate timeout report field %q", name)
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("null timeout report field %q", name)
		}
		seen[name] = struct{}{}
	}
	closing, err := d.Token()
	if err != nil {
		return err
	}
	if closing != json.Delim('}') {
		return errors.New("timeout report object did not close")
	}
	for index := range schema.NumField() {
		name := schema.Field(index).Tag.Get("json")
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("missing timeout report field %q", name)
		}
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return errors.New("timeout report has trailing JSON")
	}
	return nil
}

func decodeReport(data []byte, pid int) (report, error) {
	var r report
	if pid <= 0 || len(data) == 0 || len(data) > maxReportBytes {
		return r, errors.New("invalid owned timeout report bounds/PID")
	}
	if err := validateReportFields(data); err != nil {
		return r, err
	}
	// Typed decoding additionally rejects numeric overflow/fractions/strings,
	// wrong scalar types and objects/arrays for the producer's scalar fields.
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	reason, err := strconv.ParseUint(r.RemovedReason, 0, 32)
	if err != nil || reason&0x80000000 == 0 || r.Schema != 1 || r.PID != pid || !r.Software || r.Debug || !r.Pass || r.Failure != "" ||
		!r.InitialDrain || !r.Held || r.Submitted != 1 || r.Completed != 0 || r.FirstDrain || r.DrainMS < 4500 || r.DrainMS > 7000 ||
		r.SubmittedFence == 0 || r.FenceBefore >= r.SubmittedFence || r.FenceAfter != math.MaxUint64 ||
		r.TerminalDrain || r.TerminalWait || r.TerminalSignal || r.TerminalSubmit || !r.FirstPreserved ||
		r.FirstError != "GPU fence did not complete within five seconds" || r.TerminalMS >= 1000 || !r.Retired || r.RetireMS >= 1000 || r.DestructMS >= 1000 {
		return r, errors.New("real owned timeout/cancellation report did not satisfy native gates")
	}
	return r, nil
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "-child" {
		data, err := nativeProbe()
		if len(data) > 0 {
			_, _ = os.Stdout.Write(append(data, '\n'))
		}
		return err
	}
	if len(os.Args) != 1 {
		return errors.New("unexpected timeout acceptance arguments")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-child")
	cmd.WaitDelay = 5 * time.Second
	stdout := &boundedOutput{limit: maxReportBytes}
	stderr := &boundedOutput{limit: 128 << 10}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("owned native timeout child failed: %w; deadline=%v; stdout=%s; stderr=%s", err, ctx.Err(), stdout.Bytes(), stderr.Bytes())
	}
	if stdout.overflow || stderr.overflow {
		return errors.New("owned native timeout output exceeded bounds")
	}
	if _, err = decodeReport(stdout.Bytes(), cmd.Process.Pid); err != nil {
		return fmt.Errorf("%w; raw=%s", err, stdout.Bytes())
	}
	_, err = os.Stdout.Write(stdout.Bytes())
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
