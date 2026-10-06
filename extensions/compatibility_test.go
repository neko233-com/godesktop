package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/editor"
)

func TestVSIXLiveDocumentEditsProvidersAndPersistentState(t *testing.T) {
	workspace, storage := t.TempDir(), t.TempDir()
	file := filepath.Join(workspace, "main.go")
	if err := os.WriteFile(file, []byte("disk version"), 0644); err != nil {
		t.Fatal(err)
	}
	script := `const v=require('vscode');exports.activate=c=>{
 let changes=0; c.subscriptions.push(v.workspace.onDidChangeTextDocument(e=>{if(e.document===v.window.activeTextEditor.document)changes++;}));
 c.subscriptions.push(v.languages.registerCompletionItemProvider('go',{provideCompletionItems(d,p,token){return [new v.CompletionItem('version-'+d.version+':'+d.getText(new v.Range(0,0,0,3)),v.CompletionItemKind.Function)];}}));
 c.subscriptions.push(v.languages.registerHoverProvider('go',{provideHover(d,p){return new v.Hover(new v.MarkdownString('live '+d.version),new v.Range(p,p));}}));
 c.subscriptions.push(v.commands.registerCommand('test.hello',async()=>{
  const e=v.window.activeTextEditor,d=e.document;
  if(d.getText()!=='你😀\r\nsecond')throw Error('Editor did not expose unsaved document');
  if(d.offsetAt(new v.Position(1,0))!==5 || d.positionAt(5).line!==1)throw Error('UTF-16 CRLF coordinates');
  const old=await c.globalState.get('count',0);await c.globalState.update('count',old+1);
  const applied=await e.edit(edit=>edit.replace(new v.Range(0,1,0,3),'界'));
  return {applied,text:d.getText(),version:d.version,changes,count:old+1,range:d.lineAt(0).range,isDirty:d.isDirty};
 }));
 c.subscriptions.push(v.commands.registerCommand('test.snapshot',()=>({version:v.window.activeTextEditor.document.version,text:v.window.activeTextEditor.document.getText(),changes})));};`
	e, err := Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/main.cjs": script}))
	if err != nil {
		t.Fatal(err)
	}
	for iteration := 1; iteration <= 2; iteration++ {
		buffer, _ := editor.New("你😀\r\nsecond")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		h, err := Start(ctx, workspace, []Extension{e})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		t.Cleanup(func() { h.Close(); cancel() })
		h.Register("workspace/applyEdit", func(_ context.Context, raw json.RawMessage) (any, error) {
			var request struct {
				Documents []struct {
					Path    string        `json:"path"`
					Version int           `json:"version"`
					Edits   []editor.Edit `json:"edits"`
				} `json:"documents"`
			}
			if err := json.Unmarshal(raw, &request); err != nil {
				return nil, err
			}
			if len(request.Documents) != 1 || request.Documents[0].Version != buffer.Version() {
				t.Error("missing optimistic version")
				return map[string]bool{"applied": false}, nil
			}
			change, err := buffer.Apply(request.Documents[0].Edits, nil)
			if err != nil {
				return nil, err
			}
			return map[string]any{"applied": true, "documents": []any{map[string]any{"path": file, "text": buffer.Text(), "version": buffer.Version(), "dirty": buffer.Dirty(), "changes": change.Changes}}}, nil
		})
		initial := map[string]any{"path": file, "text": buffer.Text(), "version": buffer.Version(), "languageId": "go", "dirty": true}
		if err = h.Call(ctx, "initialize", map[string]any{"documents": []any{initial}, "active": initial, "storageRoot": storage}, nil); err != nil {
			h.Close()
			cancel()
			t.Fatal(err)
		}
		var result struct {
			Applied, IsDirty        bool
			Text                    string
			Version, Changes, Count int
		}
		if err = h.Call(ctx, "execute", map[string]string{"command": "test.hello"}, &result); err != nil {
			h.Close()
			cancel()
			t.Fatal(err)
		}
		if !result.Applied || !result.IsDirty || result.Text != "你界\r\nsecond" || result.Version != 2 || result.Changes != 1 || result.Count != iteration {
			t.Fatalf("live edit: %+v", result)
		}
		var items []struct {
			Label string
			Kind  int
		}
		if err = h.Call(ctx, "provideCompletionItems", map[string]any{"uri": fileURI(file), "position": editor.Position{Line: 0, Character: 2}}, &items); err != nil || len(items) != 1 || items[0].Label != "version-2:你界" {
			t.Fatalf("provider: %+v %v", items, err)
		}
		var hover []json.RawMessage
		if err = h.Call(ctx, "provideHover", map[string]any{"uri": fileURI(file), "position": editor.Position{}}, &hover); err != nil || len(hover) != 1 || !strings.Contains(string(hover[0]), "live 2") {
			t.Fatalf("hover %s %v", hover, err)
		}
		_ = buffer.SetSelection(editor.Selection{Anchor: editor.Position{Line: 1, Character: 6}, Active: editor.Position{Line: 1, Character: 6}})
		change, _ := buffer.ReplaceSelection("😀")
		if err = h.Call(ctx, "syncDocument", map[string]any{"kind": "change", "document": map[string]any{"path": file, "version": buffer.Version(), "dirty": true}, "changes": change.Changes}, nil); err != nil {
			t.Fatal(err)
		}
		var snapshot struct {
			Version, Changes int
			Text             string
		}
		if err = h.Call(ctx, "execute", map[string]string{"command": "test.snapshot"}, &snapshot); err != nil || snapshot.Version != 3 || snapshot.Changes != 2 || snapshot.Text != "你界\r\nsecond😀" {
			t.Fatalf("incremental native sync: %+v %v", snapshot, err)
		}
		if data, _ := os.ReadFile(file); string(data) != "disk version" {
			t.Fatal("extension edit wrote directly to disk")
		}
		h.Close()
		cancel()
	}
}

func fileURI(file string) string {
	file = filepath.ToSlash(file)
	if !strings.HasPrefix(file, "/") {
		file = "/" + file
	}
	return "file://" + file
}
