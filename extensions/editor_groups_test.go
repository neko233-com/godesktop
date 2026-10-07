package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/editor"
)

func TestNativeEditorIdentityVisibilityEventsAndStaleSnapshots(t *testing.T) {
	workspace := t.TempDir()
	file, hidden := filepath.Join(workspace, "main.go"), filepath.Join(workspace, "hidden.go")
	for _, name := range []string{file, hidden} {
		if err := os.WriteFile(name, []byte("disk"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `const v=require('vscode');let held;const counts={active:0,visible:0,column:0,ranges:0,selection:0,close:0};exports.activate=c=>{
 for(const [name,event] of Object.entries({active:v.window.onDidChangeActiveTextEditor,visible:v.window.onDidChangeVisibleTextEditors,column:v.window.onDidChangeTextEditorViewColumn,ranges:v.window.onDidChangeTextEditorVisibleRanges,selection:v.window.onDidChangeTextEditorSelection,close:v.workspace.onDidCloseTextDocument}))c.subscriptions.push(event(()=>counts[name]++));
 c.subscriptions.push(v.commands.registerCommand('test.hello',()=>{held=v.window.visibleTextEditors[1];return snapshot();}));
 c.subscriptions.push(v.commands.registerCommand('test.snapshot',snapshot));
 c.subscriptions.push(v.commands.registerCommand('test.editHeld',()=>held.edit(e=>e.insert(new v.Position(0,0),'stale'))));
 function snapshot(){const views=v.window.visibleTextEditors;return {count:views.length,documents:v.workspace.textDocuments.length,active:views.indexOf(v.window.activeTextEditor),sameDocument:views.length===2&&views[0].document===views[1].document,heldSame:views.includes(held),heldColumn:held?.viewColumn,columns:views.map(e=>e.viewColumn),selections:views.map(e=>e.selection.active),text:views.map(e=>e.document.getText()),counts:{...counts}};}
 };`
	e, err := Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/main.cjs": script}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := Start(ctx, workspace, []Extension{e})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	p := editor.Position{Line: 0, Character: 2}
	s := editor.Selection{Anchor: p, Active: p}
	view := func(id, path string, column int) map[string]any {
		return map[string]any{"id": id, "path": path, "viewColumn": column, "selection": s, "visibleRanges": []editor.Range{{Start: editor.Position{}, End: p}}}
	}
	layout := func(generation int, active string, views ...map[string]any) map[string]any {
		return map[string]any{"generation": generation, "active": active, "editors": views}
	}
	state := layout(1, "a", view("a", file, 1), view("b", file, 2))
	doc := func(path, text string) map[string]any {
		instance := "1"
		if path == hidden {
			instance = "2"
		}
		return map[string]any{"path": path, "text": text, "version": 1, "instance": instance}
	}
	if err = h.Call(ctx, "initialize", map[string]any{"documents": []any{doc(file, "😀α"), doc(hidden, "hidden")}, "editors": state, "clientCapabilities": map[string]bool{"editorGroups": true}}, nil); err != nil {
		t.Fatal(err)
	}
	type result struct {
		Count, Documents, Active int
		SameDocument, HeldSame   bool
		HeldColumn               *int
		Columns                  []int
		Selections               []editor.Position
		Text                     []string
		Counts                   map[string]int
	}
	snapshot := func(command string) result {
		t.Helper()
		var r result
		if err := h.Call(ctx, "execute", map[string]string{"command": command}, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	r := snapshot("test.hello")
	if r.Count != 2 || r.Documents != 2 || r.Active != 0 || !r.SameDocument || !r.HeldSame || r.Selections[1] != p {
		t.Fatalf("distinct native views: %+v", r)
	}
	if err = h.Call(ctx, "syncEditors", layout(2, "a", view("a", file, 1), view("b", file, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if r = snapshot("test.snapshot"); r.Counts["active"] != 0 || r.Counts["visible"] != 0 || r.Counts["selection"] != 0 {
		t.Fatalf("duplicate events: %+v", r)
	}
	if err = h.Call(ctx, "syncEditors", layout(1, "b", view("b", file, 1)), nil); err != nil {
		t.Fatal(err)
	}
	if r = snapshot("test.snapshot"); r.Count != 2 || r.Active != 0 {
		t.Fatal("stale layout overwrote views", r)
	}
	if err = h.Call(ctx, "syncEditors", layout(3, "b", view("b", file, 1)), nil); err != nil {
		t.Fatal(err)
	}
	if r = snapshot("test.snapshot"); r.Count != 1 || !r.HeldSame || r.HeldColumn == nil || *r.HeldColumn != 1 || r.Counts["column"] != 1 || r.Counts["visible"] != 1 || r.Counts["active"] != 1 {
		t.Fatalf("stable renumbered view: %+v", r)
	}
	if err = h.Call(ctx, "syncEditors", layout(4, "c", view("c", hidden, 1)), nil); err != nil {
		t.Fatal(err)
	}
	var applied bool
	if err = h.Call(ctx, "execute", map[string]string{"command": "test.editHeld"}, &applied); err != nil || applied {
		t.Fatal("disposed editor accepted edit", err)
	}
	if err = h.Call(ctx, "syncEditors", layout(5, "d", view("c", hidden, 1), view("d", file, 2)), nil); err != nil {
		t.Fatal(err)
	}
	if r = snapshot("test.snapshot"); r.Count != 2 || r.HeldSame || r.HeldColumn != nil {
		t.Fatalf("hidden editor identity revived: %+v", r)
	}
	if err = h.Call(ctx, "syncDocument", map[string]any{"kind": "close", "document": map[string]any{"path": hidden}, "editors": layout(6, "d", view("d", file, 1))}, nil); err != nil {
		t.Fatal(err)
	}
	if r = snapshot("test.snapshot"); r.Count != 1 || r.Documents != 1 || r.Counts["close"] != 1 || r.Active != 0 {
		t.Fatalf("resource/view close differ: %+v", r)
	}
	// Malformed whole snapshots cannot partially remove a live editor.
	if err = h.Call(ctx, "syncEditors", layout(7, "d", view("d", file, 1), view("d", file, 2)), nil); err == nil {
		t.Fatal("duplicate editor identity accepted")
	}
	if r = snapshot("test.snapshot"); r.Count != 1 || r.Columns[0] != 1 {
		t.Fatal("invalid batch mutated live editors", r)
	}
	bad := view("d", file, 1)
	bad["selection"] = editor.Selection{Anchor: editor.Position{Line: 100}, Active: editor.Position{Line: 100}}
	if err = h.Call(ctx, "syncEditors", layout(7, "d", bad), nil); err == nil {
		t.Fatal("invalid coordinates accepted")
	}
	if r = snapshot("test.snapshot"); r.Count != 1 || r.Text[0] != "😀α" {
		t.Fatal("invalid coordinates partially disposed live editor", r)
	}
	fresh := doc(file, "NEW")
	fresh["instance"] = "3"
	if err = h.Call(ctx, "syncDocument", map[string]any{"kind": "open", "document": fresh, "editors": layout(8, "e", view("e", file, 1))}, nil); err != nil {
		t.Fatal(err)
	}
	if err = h.Call(ctx, "syncDocument", map[string]any{"kind": "close", "document": map[string]any{"path": file, "instance": "1"}, "editors": layout(9, "", []map[string]any{}...)}, nil); err != nil {
		t.Fatal(err)
	}
	if err = h.Call(ctx, "syncDocument", map[string]any{"kind": "open", "document": doc(file, "OLD"), "editors": layout(10, "f", view("f", file, 1))}, nil); err != nil {
		t.Fatal(err)
	}
	if r = snapshot("test.snapshot"); r.Count != 1 || r.Documents != 1 || r.Text[0] != "NEW" || r.Counts["close"] != 2 {
		t.Fatal("old resource notification replaced/closed reopened native instance", r)
	}
}

func TestHostRuntimeFilesRemoveWindowsCommandLimitAndBoundFallbackReads(t *testing.T) {
	workspace := t.TempDir()
	huge := filepath.Join(workspace, "huge.txt")
	f, err := os.Create(huge)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	f.Close()
	script := `const v=require('vscode');exports.activate=c=>c.subscriptions.push(v.commands.registerCommand('test.hello',async()=>{try{await v.workspace.openTextDocument(v.Uri.joinPath(v.workspace.workspaceFolders[0].uri,'huge.txt'));return false;}catch(e){return e.message.includes('2 MiB');}}));`
	e, err := Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/main.cjs": script}))
	if err != nil {
		t.Fatal(err)
	}
	many := make([]Extension, 512)
	for i := range many {
		many[i] = e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := Start(ctx, workspace, many)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Close() })
	data, err := os.ReadFile(filepath.Join(h.runtimeRoot, "config.json"))
	if err != nil || len(data) < 32768 {
		t.Fatal("configuration did not exceed Windows CLI limit", len(data), err)
	}
	if err = h.Call(ctx, "initialize", nil, nil); err != nil {
		t.Fatal(err)
	}
	var bounded bool
	if err = h.Call(ctx, "execute", map[string]string{"command": "test.hello"}, &bounded); err != nil || !bounded {
		t.Fatal("GiB fallback read was not rejected before allocation", err)
	}
	if err = h.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(h.runtimeRoot); !os.IsNotExist(err) {
		t.Fatal("owned runtime files leaked", err)
	}
	// Runtime configuration is plain JSON, with the expected workspace and no
	// command-line escaping substitution of newlines or extension metadata.
	var config struct {
		Workspace  string
		Extensions []Extension
	}
	if json.Unmarshal(data, &config) != nil || config.Workspace != workspace || len(config.Extensions) != len(many) || !strings.Contains(string(data), e.Manifest.Name) {
		t.Fatal("runtime configuration changed")
	}
}

func TestConcurrentVSIXOpeningReceiptsAreInvocationLocal(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	script := `const v=require('vscode');exports.activate=c=>c.subscriptions.push(v.commands.registerCommand('test.hello',async()=>{const d=await v.workspace.openTextDocument(v.Uri.joinPath(v.workspace.workspaceFolders[0].uri,'main.go'));await new Promise(resolve=>setTimeout(resolve,75));await v.window.showTextDocument(d);return true;}));`
	e, err := Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/main.cjs": script}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := Start(ctx, workspace, []Extension{e})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	doc := map[string]any{"path": file, "text": "hello", "version": 1, "instance": "1"}
	layout := map[string]any{"generation": 1, "active": "a", "editors": []any{map[string]any{"id": "a", "path": file, "viewColumn": 1, "selection": editor.Selection{}, "visibleRanges": []editor.Range{{End: editor.Position{Character: 5}}}}}}
	var sequence atomic.Int32
	receipts := make(chan int, 2)
	h.Register("workspace/openTextDocument", func(context.Context, json.RawMessage) (any, error) {
		token := sequence.Add(1)
		return map[string]any{"document": doc, "editors": layout, "focusReceipt": map[string]any{"sequence": token}}, nil
	})
	h.Register("window/showTextDocument", func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			FocusReceipt struct {
				Sequence int `json:"sequence"`
			} `json:"focusReceipt"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		receipts <- request.FocusReceipt.Sequence
		return map[string]any{"document": doc, "editors": layout, "editorId": "a"}, nil
	})
	if err = h.Call(ctx, "initialize", map[string]any{"documents": []any{doc}, "editors": layout, "clientCapabilities": map[string]bool{"editorGroups": true, "showDocument": true, "openDocument": true}}, nil); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			var shown bool
			if err := h.Call(ctx, "execute", map[string]string{"command": "test.hello"}, &shown); err != nil || !shown {
				t.Errorf("concurrent real VSIX command: shown=%t err=%v", shown, err)
			}
		})
	}
	workers.Wait()
	got := []int{}
	for len(got) < 2 {
		select {
		case token := <-receipts:
			got = append(got, token)
		case <-ctx.Done():
			t.Fatal("missing native opening receipt", ctx.Err())
		}
	}
	sort.Ints(got)
	if got[0] != 1 || got[1] != 2 {
		t.Fatal("shared TextDocument mixed two invocation receipts", got)
	}
}
