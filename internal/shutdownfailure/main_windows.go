//go:build windows && cgo

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func main() {
	output := flag.String("output", ".cache/shutdown-failure/current", "fixed owned evidence directory")
	flag.Parse()
	if err := run(*output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output string) (failure error) {
	cleanup, err := prepareOutput(output)
	if err != nil {
		return err
	}
	cleaned := false
	defer func() {
		if !cleaned {
			failure = errors.Join(failure, cleanup())
		}
	}()
	if err := resetEvidence(output); err != nil {
		return err
	}
	for key, value := range map[string]string{
		"GODESKTOP_TEST_SHUTDOWN_TIMEOUT": "1", "GODESKTOP_TEST_INPUT_ISOLATION": "1",
		"GODESKTOP_GPU_ADAPTER": "warp", "GODESKTOP_GPU_DEBUG": "0", "GODESKTOP_GPU_TRACE_STAGES": "0",
		"GODESKTOP_READBACK": "0", "GODESKTOP_TEST_DEVICE_REMOVAL": "0",
	} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	if err := os.Unsetenv("GODESKTOP_TEST_DRAWABLE_SCALE"); err != nil {
		return err
	}
	watchdog := time.AfterFunc(35*time.Second, func() {
		fmt.Fprintln(os.Stderr, "native shutdown failure 35s watchdog expired")
		os.Exit(2)
	})
	defer watchdog.Stop()
	r, err := runWindow()
	if err != nil {
		body, _ := json.MarshalIndent(struct {
			Error  string `json:"error"`
			Report report `json:"report"`
		}{err.Error(), r}, "", "  ")
		_ = os.WriteFile(filepath.Join(output, "failed.json"), body, 0600)
		return err
	}
	if err := cleanup(); err != nil {
		return err
	}
	cleaned = true
	r.Pass = true
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "current.json"), append(body, '\n'), 0600); err != nil {
		return err
	}
	fmt.Println("native shutdown failure contract passed: pending-real-frame, five-second Run error, completed/drop accounting, owned HWND closed")
	return nil
}

type pendingReceipt struct {
	window winprobe.Window
	stats  winprobe.RenderStats
	dpi    uint32
	path   string
	held   bool
	quitAt time.Time
	err    error
}

func runWindow() (report, error) {
	pid := uint32(os.Getpid())
	r := report{Schema: 1, PID: pid}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	title := fmt.Sprintf("owned native shutdown failure %d", pid)
	finished := make(chan pendingReceipt, 1)
	started := false
	var saved *ui.Context
	runErr := ui.Run(ui.WindowOptions{Title: title, Width: 320, Height: 200}, func(cx *ui.Context) *ui.Element {
		saved = cx
		if !started {
			started = true
			go func() {
				receipt := awaitPending(ctx, cx, title, pid)
				receipt.quitAt = time.Now()
				cx.Quit()
				finished <- receipt
			}()
		}
		return ui.Column().Background(ui.RGB(0x336699))
	})
	returnedAt := time.Now()
	cancel()
	if !started {
		return r, fmt.Errorf("native failure window never entered View: %v", runErr)
	}
	var pending pendingReceipt
	select {
	case pending = <-finished:
		r.WorkerJoined = true
	case <-time.After(3 * time.Second):
		return r, errors.New("owned native shutdown worker did not join within three seconds")
	}
	r.HWND, r.Pending = uint64(pending.window), pending.stats
	r.QuitToRunMS = returnedAt.Sub(pending.quitAt).Milliseconds()
	r.Closed = winprobe.RendererStats()
	r.OldRejected = saved != nil && !saved.Dispatch(func() {})
	if pending.window != 0 {
		alive, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("IsWindow").Call(uintptr(pending.window))
		r.WindowDestroyed = alive == 0
	}
	if runErr != nil {
		r.RunError = runErr.Error()
	}
	r.Presentation, r.DPI, r.ShutdownPending = pending.path, pending.dpi, pending.held
	if pending.err != nil {
		return r, pending.err
	}
	return r, validateReport(r, pid)
}

func awaitPending(ctx context.Context, cx *ui.Context, title string, pid uint32) pendingReceipt {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		window, err := winprobe.Find(title, pid)
		if err == nil {
			stats := winprobe.RendererStats()
			if stats.Submitted >= 5 && stats.Completed < stats.Submitted && stats.InFlight > 0 {
				held, err := actualShutdownPending(window, pid)
				if err != nil {
					return pendingReceipt{window: window, stats: stats, err: err}
				}
				if !held {
					select {
					case <-ctx.Done():
						return pendingReceipt{window: window, stats: stats, err: errors.New("pending fifth frame lacks its actual diagnostic HoldQueue cause property")}
					case <-ticker.C:
					}
					continue
				}
				path, err := winprobe.NativePresentation(window, pid)
				if err == nil {
					err = winprobe.ValidateWindowsPresentation(stats, path)
				}
				if err == nil && path != "committed-dib" {
					err = errors.New("shutdown timeout requires the actual committed-dib window")
				}
				return pendingReceipt{window: window, stats: stats, dpi: window.DPI(), path: path, held: held, err: err}
			}
			if stats.Submitted < 5 {
				cx.Invalidate()
			}
		}
		select {
		case <-ctx.Done():
			return pendingReceipt{err: fmt.Errorf("no fifth actual pending native frame before shutdown: %w", ctx.Err())}
		case <-ticker.C:
		}
	}
}

func actualShutdownPending(window winprobe.Window, pid uint32) (bool, error) {
	if window == 0 || pid == 0 || pid != uint32(os.Getpid()) {
		return false, errors.New("shutdown pending property requires this exact owned nonzero HWND/PID")
	}
	user := syscall.NewLazyDLL("user32.dll")
	owner := func() uint32 {
		var actual uint32
		user.NewProc("GetWindowThreadProcessId").Call(uintptr(window), uintptr(unsafe.Pointer(&actual)))
		return actual
	}
	if owner() != pid {
		return false, errors.New("shutdown pending HWND belongs to another/closed process")
	}
	name, err := syscall.UTF16PtrFromString("godesktop.shutdown-pending")
	if err != nil {
		return false, err
	}
	value, _, _ := user.NewProc("GetPropW").Call(uintptr(window), uintptr(unsafe.Pointer(name)))
	if owner() != pid || value > 1 {
		return false, errors.New("shutdown pending HWND changed owner or has an unknown native property")
	}
	return value == 1, nil
}

func resetEvidence(output string) error {
	for _, name := range []string{"current.json", "failed.json"} {
		if err := os.Remove(filepath.Join(output, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func prepareOutput(output string) (func() error, error) {
	cache, err := filepath.Abs(filepath.Join(".cache", "shutdown-failure"))
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(output)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(cache, root)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, errors.New("shutdown failure output must remain in its owned workspace cache")
	}
	for cursor := root; ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
			return nil, errors.New("shutdown failure output ancestor is not an ordinary directory")
		}
		if cursor == filepath.Dir(cursor) {
			break
		}
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	// Retire the previous fixed receipt even if a retained private scratch
	// directory prevents this invocation from reaching native startup.
	if err := resetEvidence(root); err != nil {
		return nil, err
	}
	temporary := filepath.Join(cache, "owned-temp-"+filepath.Base(root))
	if err := os.Mkdir(temporary, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(temporary)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("shutdown failure scratch is not an ordinary directory")
	}
	entries, err := os.ReadDir(temporary)
	if err != nil || len(entries) != 0 {
		return nil, errors.New("shutdown failure scratch is not empty")
	}
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		if err := os.Setenv(key, temporary); err != nil {
			return nil, err
		}
	}
	return func() error {
		entries, err := os.ReadDir(temporary)
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return errors.New("shutdown failure scratch retained files after owned closure")
		}
		return os.Remove(temporary)
	}, nil
}
