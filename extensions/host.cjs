'use strict';
// Supported API is deliberately explicit: unknown calls fail at activation.
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL, fileURLToPath } = require('node:url');
const Module = require('node:module');
const config = JSON.parse(process.argv[1]);
const write = value => process.stdout.write(JSON.stringify(value) + '\n');
const emit = event => write({ event });
for (const key of ['log','info','warn','error','debug']) console[key] = (...args) => process.stderr.write(args.map(String).join(' ')+'\n');
class Disposable { constructor(fn=()=>{}) { this.fn=fn; } dispose() { const fn=this.fn; this.fn=()=>{}; fn(); } static from(...items) { return new Disposable(()=>items.forEach(x=>x.dispose())); } }
class EventEmitter { constructor(){ this.listeners=new Set(); this.event=(fn,thisArg)=>{ const cb=value=>fn.call(thisArg,value); this.listeners.add(cb); return new Disposable(()=>this.listeners.delete(cb)); }; } fire(value){ for(const fn of this.listeners) fn(value); } dispose(){this.listeners.clear();} }
class Uri {
  constructor(value){this.value=value; const u=new URL(value); this.scheme=u.protocol.slice(0,-1); this.path=decodeURIComponent(u.pathname); this.fsPath=this.scheme==='file'?fileURLToPath(u):this.path;}
  static file(value){return new Uri(pathToFileURL(path.resolve(value)).href);} static parse(value){return new Uri(value);} static joinPath(base,...parts){return Uri.file(path.join(base.fsPath,...parts));} toString(){return this.value;} toJSON(){return this.value;}
}
class Position { constructor(line,character){this.line=line;this.character=character;} }
class Range { constructor(a,b,c,d){this.start=typeof a==='number'?new Position(a,b):a;this.end=typeof a==='number'?new Position(c,d):b;} }
class Selection extends Range { constructor(...args){super(...args);this.anchor=this.start;this.active=this.end;} }
const registrations=new Map(), active=new Map(), activating=new Map();
const folders=[{uri:Uri.file(config.workspace),name:path.basename(config.workspace),index:0}];
const unsupported = name => new Proxy({}, { get(_,key){if(key==='then')return undefined;throw new Error(`Unsupported VS Code API: ${name}.${String(key)}`);} });
const strict = (name,value) => new Proxy(value,{get(target,key){ if(key in target || typeof key==='symbol') return target[key]; throw new Error(`Unsupported VS Code API: ${name}.${String(key)}`); }});
const message = type => async (text,...items)=>{emit({type,text:String(text)});return undefined;};
function document(uri){ if(typeof uri==='string')uri=Uri.file(uri); const text=fs.readFileSync(uri.fsPath,'utf8');const lines=text.split(/\r?\n/);return {uri,fileName:uri.fsPath,languageId:path.extname(uri.fsPath)==='.go'?'go':'plaintext',version:1,isDirty:false,isClosed:false,lineCount:lines.length,getText:()=>text,lineAt:i=>({lineNumber:i,text:lines[i]||''})}; }
const vscode = strict('vscode',{
  Disposable,EventEmitter,Uri,Position,Range,Selection,
  StatusBarAlignment:{Left:1,Right:2},ExtensionMode:{Production:1,Development:2,Test:3},
  commands:strict('commands',{
    registerCommand(id,fn,thisArg){if(registrations.has(id))throw new Error(`Duplicate command: ${id}`);registrations.set(id,(...args)=>fn.apply(thisArg,args));return new Disposable(()=>registrations.delete(id));},
    async executeCommand(id,...args){await activateFor(id);const fn=registrations.get(id);if(!fn)throw new Error(`Command not found: ${id}`);return await fn(...args);},
    async getCommands(){return [...new Set([...registrations.keys(),...config.extensions.flatMap(e=>e.manifest.contributes?.commands?.map(c=>c.command)||[])])];}
  }),
  window:strict('window',{
    showInformationMessage:message('information'),showWarningMessage:message('warning'),showErrorMessage:message('error'),
    createOutputChannel(name){return {name,append:text=>emit({type:'output',channel:name,text:String(text)}),appendLine:text=>emit({type:'output',channel:name,text:String(text)+'\n'}),clear:()=>emit({type:'clear',channel:name,text:''}),show:()=>emit({type:'panel',channel:name,text:''}),hide(){},dispose(){}};},
    createStatusBarItem(){return {text:'',tooltip:'',command:undefined,show(){emit({type:'status',text:this.text});},hide(){emit({type:'status',text:''});},dispose(){this.hide();}};},
    async showTextDocument(value){const doc=value instanceof Uri?document(value):value;emit({type:'open',path:doc.uri.fsPath,text:''});return {document:doc};}
  }),
  workspace:strict('workspace',{
    workspaceFolders:folders,rootPath:config.workspace,
    async openTextDocument(value){return document(value);},
    getConfiguration(){return {get:(_,fallback)=>fallback,has:()=>false};},
    fs:{readFile:async uri=>new Uint8Array(await fs.promises.readFile(uri.fsPath)),stat:async uri=>{const s=await fs.promises.stat(uri.fsPath);return {type:s.isDirectory()?2:1,ctime:s.ctimeMs,mtime:s.mtimeMs,size:s.size};}}
  }),
  env:strict('env',{appName:'godesktop',language:'en',uiKind:1}),
  languages:unsupported('languages'),debug:unsupported('debug'),tasks:unsupported('tasks')
});
const originalLoad=Module._load;
Module._load=function(request,...args){return request==='vscode'?vscode:originalLoad.call(this,request,...args);};
function memento(){const values=new Map();return {get:(key,fallback)=>values.has(key)?values.get(key):fallback,keys:()=>[...values.keys()],update:async(key,value)=>{if(value===undefined)values.delete(key);else values.set(key,value);}};}
async function activate(extension){
  const id=extension.manifest.publisher+'.'+extension.manifest.name;
  if(active.has(id))return; if(activating.has(id))return activating.get(id);
  const promise=(async()=>{
    if(!extension.manifest.main)throw new Error(`${id}: no desktop main entry`);
    const location=path.resolve(extension.path,extension.manifest.main);
    const relative=path.relative(extension.path,location);if(relative.startsWith('..')||path.isAbsolute(relative))throw new Error('Extension main escapes installation directory');
    const subscriptions=[]; const context={subscriptions,extensionPath:extension.path,extensionUri:Uri.file(extension.path),asAbsolutePath:p=>path.join(extension.path,p),workspaceState:memento(),globalState:memento(),extensionMode:1};
    try {const module=require(location);const exports=await module.activate?.(context);active.set(id,{module,subscriptions,exports});emit({type:'activated',text:id});}
    catch(error){subscriptions.forEach(x=>x.dispose());throw error;}
  })(); activating.set(id,promise);try{await promise;}finally{activating.delete(id);}
}
async function activateFor(command){
  for(const e of config.extensions)if((e.manifest.contributes?.commands||[]).some(c=>c.command===command)||(e.manifest.activationEvents||[]).includes('onCommand:'+command))await activate(e);
}
async function handle(request){
  try{
    let result;
    if(request.method==='initialize'){
      for(const e of config.extensions)if((e.manifest.activationEvents||[]).some(x=>x==='*'||x==='onStartupFinished'))await activate(e);
      result=await vscode.commands.getCommands();
    }else if(request.method==='execute')result=await vscode.commands.executeCommand(request.params.command,...(request.params.args||[]));
    else throw new Error(`Unknown method: ${request.method}`);
    write({id:request.id,result:result??null});
  }catch(error){write({id:request.id,error:error?.stack||String(error)});}
}
require('node:readline').createInterface({input:process.stdin}).on('line',line=>{
  try{handle(JSON.parse(line));}catch(error){emit({type:'error',text:String(error)});}
}).on('close',async()=>{
  for(const {module,subscriptions} of active.values()){try{await module.deactivate?.();}catch{}subscriptions.forEach(x=>x.dispose());}
});
