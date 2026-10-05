package extensions

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func archive(t *testing.T, manifest string, files map[string]string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "test.vsix")
	file, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(file)
	filesCopy := map[string]string{"extension/package.json": manifest}
	for k, v := range files {
		filesCopy[k] = v
	}
	for k, v := range filesCopy {
		w, err := z.Create(k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return name
}

const testManifest = `{"publisher":"test","name":"hello","version":"1.0.0","main":"main.cjs","contributes":{"commands":[{"command":"test.hello","title":"Hello"},{"command":"test.unsupported","title":"Unsupported"},{"command":"test.loop","title":"Loop"}]}}`

func TestInstallPaths(t *testing.T) {
	for _, name := range []string{"../escape", "../../escape", "C:/escape", "/escape", "a/../../escape", "a\\..\\escape", "NUL.txt", "a:stream", "trailing.", "trailing ", "COM1.js"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			_, err := Install(root, archive(t, testManifest, map[string]string{"extension/" + name: "oops"}))
			if err == nil {
				t.Fatal("unsafe path accepted")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 0 {
				t.Fatal("failed installation was left behind")
			}
		})
	}
	for _, name := range []string{"main.cjs", "node_modules/dependency/index.js", "./file.js", "icons/tool.png"} {
		if _, err := localPath(name); err != nil {
			t.Fatal(err)
		}
	}
}
func TestInstallVersions(t *testing.T) {
	root := t.TempDir()
	for _, v := range []string{"1.0.0", "1.10.0", "1.9.0"} {
		_, err := Install(root, archive(t, strings.Replace(testManifest, "1.0.0", v, 1), map[string]string{"extension/main.cjs": "exports.activate=()=>{};"}))
		if err != nil {
			t.Fatal(err)
		}
	}
	list, err := List(root)
	if err != nil || len(list) != 1 || list[0].Manifest.Version != "1.10.0" {
		t.Fatalf("list=%v err=%v", list, err)
	}
	_, err = Install(root, archive(t, testManifest, nil))
	if err == nil {
		t.Fatal("overwrote existing version")
	}
	if _, err = Install(t.TempDir(), archive(t, strings.Replace(testManifest, "test", "../test", 1), nil)); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if _, err = Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/MAIN.CJS": "x", "extension/main.cjs": "y"})); err == nil {
		t.Fatal("case-colliding files accepted")
	}
	if _, err = Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/huge": strings.Repeat("x", (16<<20)+1)})); err == nil {
		t.Fatal("oversized file accepted")
	}
}

func TestNodeHost(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("Node.js is required to test extension compatibility")
	}
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello workspace"), 0644); err != nil {
		t.Fatal(err)
	}
	script := `const v=require('vscode'); exports.activate=c=>{
console.log('logging must not corrupt protocol');
c.subscriptions.push(v.commands.registerCommand('test.hello',async name=>{
await v.window.showInformationMessage('Hello '+name);
const d=await v.workspace.openTextDocument(v.Uri.joinPath(v.workspace.workspaceFolders[0].uri,'README.md'));
await v.window.showTextDocument(d);const o=v.window.createOutputChannel('test');o.appendLine(d.getText());
await c.workspaceState.update('test',42);return {name,text:d.getText(),state:c.workspaceState.get('test'),uri:d.uri.toString()};
}));
c.subscriptions.push(v.commands.registerCommand('test.unsupported',()=>v.languages.registerCompletionItemProvider({},{})));
c.subscriptions.push(v.commands.registerCommand('test.loop',()=>{while(true){}}));};`
	e, err := Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/main.cjs": script}))
	if err != nil {
		t.Fatal(err)
	}
	h, err := Start(context.Background(), workspace, []Extension{e})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var commands []string
	if err = h.Call(ctx, "initialize", nil, &commands); err != nil || len(commands) != 3 {
		t.Fatalf("initialize %v %v", commands, err)
	}
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			var value struct {
				Name, Text, URI string
				State           int
			}
			if err := h.Call(ctx, "execute", map[string]any{"command": "test.hello", "args": []any{fmt.Sprint(i)}}, &value); err != nil {
				t.Error(err)
			}
			if value.Text != "hello workspace" || value.State != 42 || !strings.HasPrefix(value.URI, "file:") {
				t.Errorf("result %#v", value)
			}
		})
	}
	wg.Wait()
	if err = h.Call(ctx, "execute", map[string]string{"command": "test.unsupported"}, nil); err == nil || !strings.Contains(err.Error(), "Unsupported VS Code API") {
		t.Fatalf("missing explicit API error: %v", err)
	}
	if err = h.Call(ctx, "execute", map[string]string{"command": "missing"}, nil); err == nil {
		t.Fatal("missing command accepted")
	}
	counts := map[string]int{}
	for len(h.Events) > 0 {
		counts[(<-h.Events).Type]++
	}
	if counts["activated"] != 1 || counts["information"] != 4 || counts["open"] != 4 || counts["output"] != 4 {
		t.Fatalf("events %v", counts)
	}
	loopCtx, stop := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer stop()
	if err = h.Call(loopCtx, "execute", map[string]string{"command": "test.loop"}, nil); err != context.DeadlineExceeded {
		t.Fatalf("cancellation: %v", err)
	}
	h.Close()
	if err = h.Call(ctx, "initialize", nil, nil); err == nil {
		t.Fatal("closed host accepted call")
	}
}
func TestHostActivationError(t *testing.T) {
	var m Manifest
	_ = json.Unmarshal([]byte(testManifest), &m)
	h, err := Start(context.Background(), t.TempDir(), []Extension{{Manifest: m, Path: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = h.Call(ctx, "execute", map[string]string{"command": "test.hello"}, nil); err == nil || !strings.Contains(err.Error(), "Cannot find module") {
		t.Fatalf("activation error %v", err)
	}
}
