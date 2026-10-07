package extensions

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestTerminalSnapshotAtomicityFailureAndBoundedQueue(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required for terminal protocol acceptance", err)
	}
	root := t.TempDir()
	module := filepath.Join(root, "terminals.cjs")
	if err := os.WriteFile(module, []byte(terminalSource), 0600); err != nil {
		t.Fatal(err)
	}
	script := `const assert=require('node:assert/strict'),factory=require(process.argv[1]);
class E{constructor(){this.listeners=[];this.event=fn=>{this.listeners.push(fn);return{dispose:()=>{}}};}fire(v){for(const f of this.listeners)f(v);}}
class U{}
let response,errors=[],operations=[];const t=factory({native:async(method,args)=>{operations.push(args.kind||method);if(method==='window/createTerminal'&&response instanceof Error)throw response;return response;},strict:(_,value)=>value,EventEmitter:E,Uri:U,enabled:()=>true,report:e=>errors.push(e.message)});
const record=(id)=>({id,name:id,isInteractedWith:false,processId:37,creationOptions:{}});
let opened=0,closed=0;t.events.open.event(()=>opened++);t.events.close.event(()=>closed++);
t.sync({generation:1,terminals:[record('native:1')],closed:[],activeId:'native:1'});const original=t.active;
assert.throws(()=>t.sync({generation:2,terminals:[record('native:1'),record('native:2'),{...record('bad'),processId:0}],closed:[]}));
assert.equal(t.terminals.length,1);assert.equal(t.active,original);assert.equal(opened,1);
t.sync({generation:2,terminals:[record('native:1'),record('native:2')],closed:[],activeId:'native:2'});assert.equal(opened,2);
assert.throws(()=>t.sync({generation:3,terminals:[record('native:1')],closed:[record('native:1')]}));assert.equal(t.terminals.length,2);
(async()=>{
response=new Error('owned startup rejected');const failed=t.create('failure');assert.equal(await failed.processId,undefined);assert.equal(failed.exitStatus.reason,0);assert.equal(closed,1);assert.equal(t.terminals.length,2);assert.throws(()=>failed.sendText('late'));
response={generation:3,terminals:[record('native:1'),record('native:2')],closed:[],activeId:'native:2'};
for(let i=0;i<128;i++)original.hide();assert.throws(()=>original.hide(),/queue limit/);
await new Promise(resolve=>setImmediate(resolve));assert.equal(operations.filter(x=>x==='hide').length,128);original.hide();await new Promise(resolve=>setImmediate(resolve));
for(let i=0;i<16;i++)original.sendText('x'.repeat(65534),false);assert.throws(()=>original.sendText('x'.repeat(65534),false),/queue limit/);
await new Promise(resolve=>setImmediate(resolve));assert.equal(operations.filter(x=>x==='sendText').length,16);
assert.throws(()=>original.sendText('x'.repeat(65535)),/Invalid terminal input/);assert.equal(errors.length,1);
})().catch(error=>{console.error(error);process.exitCode=1;});`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, node, "-e", script, module).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
}

func TestVSIXTerminalIdentityOrderedActionsAndClosure(t *testing.T) {
	script := `const v=require('vscode');let held, original, counts={open:0,close:0,active:0,state:0}, reason;
exports.activate=c=>{
 for(const [key,event] of [['open',v.window.onDidOpenTerminal],['close',v.window.onDidCloseTerminal],['active',v.window.onDidChangeActiveTerminal],['state',v.window.onDidChangeTerminalState]])c.subscriptions.push(event(t=>{counts[key]++;if(key==='close')reason=t.exitStatus.reason;}));
 c.subscriptions.push(v.commands.registerCommand('test.hello',async()=>{
  original={name:'测试😀',cwd:v.Uri.file(process.cwd()),env:{CHANGE:'old',REMOVE:null},shellArgs:['-i']};held=v.window.createTerminal(original);original.env.CHANGE='new';
  const same=held.processId===held.processId,pid=await held.processId;
  const shown=new Promise(resolve=>{const d=v.window.onDidChangeActiveTerminal(t=>{if(t===held){d.dispose();resolve();}});});
  held.sendText('first 世界',false);held.sendText('second 😀');held.hide();held.show(true);await shown;
  return {pid,same,options:held.creationOptions,unsupported:(()=>{try{v.window.createTerminal({pty:{}});return false;}catch{return true;}})()};
 }));
 c.subscriptions.push(v.commands.registerCommand('test.snapshot',()=>({count:v.window.terminals.length,same:v.window.terminals.includes(held),active:v.window.activeTerminal===held,interacted:held.state.isInteractedWith,counts,reason})));
 c.subscriptions.push(v.commands.registerCommand('test.dispose',async()=>{const closed=new Promise(resolve=>{const d=v.window.onDidCloseTerminal(t=>{if(t===held){d.dispose();resolve();}});});held.dispose();held.dispose();let rejected=false;try{held.sendText('late');}catch{rejected=true;}await closed;return rejected;}));
};`
	e, err := Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/main.cjs": script}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h, err := Start(ctx, t.TempDir(), []Extension{e})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	var mu sync.Mutex
	var generation = 1
	user := map[string]any{"id": "native:1", "name": "User", "processId": 888, "isInteractedWith": false, "creationOptions": map[string]any{}}
	records := []map[string]any{user}
	closed := []map[string]any{}
	active := "native:1"
	snapshot := func() any {
		return map[string]any{"generation": generation, "terminals": records, "closed": closed, "activeId": active}
	}
	var actions []string
	changed := make(chan struct{}, 8)
	h.Register("window/createTerminal", func(_ context.Context, raw json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		var request struct {
			ID      string         `json:"id"`
			Options map[string]any `json:"options"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		env := request.Options["env"].(map[string]any)
		if env["CHANGE"] != "old" || env["REMOVE"] != nil || request.Options["cwd"] == nil {
			t.Error("creation options were mutated", request.Options)
		}
		generation++
		records = append(records, map[string]any{"id": request.ID, "name": "测试😀", "processId": 777, "isInteractedWith": false, "creationOptions": request.Options})
		return snapshot(), nil
	})
	h.Register("window/terminalAction", func(_ context.Context, raw json.RawMessage) (any, error) {
		mu.Lock()
		defer mu.Unlock()
		var request struct {
			ID, Kind, Text               string
			ShouldExecute, PreserveFocus bool
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		actions = append(actions, request.Kind)
		generation++
		switch request.Kind {
		case "sendText":
			if (request.Text == "first 世界" && request.ShouldExecute) || (request.Text == "second 😀" && !request.ShouldExecute) {
				t.Error("execution default/order changed", request)
			}
			records[1]["isInteractedWith"] = true
		case "show":
			if !request.PreserveFocus {
				t.Error("preserveFocus lost")
			}
			active = request.ID
		case "hide":
		case "dispose":
			last := records[1]
			last["exitStatus"] = map[string]any{"reason": 4}
			closed = append(closed, last)
			records = records[:1]
			active = "native:1"
		default:
			t.Error("unknown operation", request.Kind)
		}
		result := snapshot()
		// Marshal under the same lock; later actions must not mutate an in-flight response.
		copy, err := json.Marshal(result)
		changed <- struct{}{}
		return json.RawMessage(copy), err
	})
	if err = h.Call(ctx, "initialize", map[string]any{"clientCapabilities": map[string]bool{"terminals": true}, "terminals": snapshot()}, nil); err != nil {
		t.Fatal(err)
	}
	var initial struct {
		PID               int
		Same, Unsupported bool
		Options           map[string]any
	}
	if err = h.Call(ctx, "execute", map[string]string{"command": "test.hello"}, &initial); err != nil {
		t.Fatal(err)
	}
	if initial.PID != 777 || !initial.Same || !initial.Unsupported {
		t.Fatal(initial)
	}
	for range 4 {
		select {
		case <-changed:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	var current struct {
		Count                    int
		Same, Active, Interacted bool
		Counts                   map[string]int
		Reason                   int
	}
	read := func() {
		t.Helper()
		if err = h.Call(ctx, "execute", map[string]string{"command": "test.snapshot"}, &current); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if current.Count != 2 || !current.Same || !current.Active || !current.Interacted || current.Counts["open"] != 1 || current.Counts["state"] != 1 {
		t.Fatal(current)
	}
	mu.Lock()
	order := append([]string(nil), actions...)
	mu.Unlock()
	if !reflect.DeepEqual(order, []string{"sendText", "sendText", "hide", "show"}) {
		t.Fatal(order)
	}
	if err = h.Call(ctx, "syncTerminals", map[string]any{"generation": 0, "terminals": []any{}, "closed": []any{}}, nil); err != nil {
		t.Fatal(err)
	}
	read()
	if current.Count != 2 {
		t.Fatal("stale snapshot removed terminal", current)
	}
	var rejected bool
	if err = h.Call(ctx, "execute", map[string]string{"command": "test.dispose"}, &rejected); err != nil || !rejected {
		t.Fatal(err, rejected)
	}
	select {
	case <-changed:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	read()
	if current.Count != 1 || current.Same || current.Active || current.Counts["close"] != 1 || current.Reason != 4 {
		t.Fatal(current)
	}
	mu.Lock()
	order = append([]string(nil), actions...)
	mu.Unlock()
	if len(order) != 5 || order[4] != "dispose" {
		t.Fatal("duplicate disposal", order)
	}
}

func TestTerminalRuntimeModuleIsOwnedAndLegacyCapabilityFailsClearly(t *testing.T) {
	e, err := Install(t.TempDir(), archive(t, testManifest, map[string]string{"extension/main.cjs": `const v=require('vscode');exports.activate=c=>c.subscriptions.push(v.commands.registerCommand('test.hello',()=>v.window.createTerminal('legacy')));`}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := Start(ctx, t.TempDir(), []Extension{e})
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(h.runtimeRoot, "terminals.cjs")
	if _, err = os.Stat(file); err != nil {
		t.Fatal(err)
	}
	if err = h.Call(ctx, "initialize", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err = h.Call(ctx, "execute", map[string]string{"command": "test.hello"}, nil); err == nil {
		t.Fatal("legacy client pretended to create a terminal")
	}
	if err = h.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("runtime module survived host shutdown", err)
	}
}
