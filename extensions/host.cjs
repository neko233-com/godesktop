'use strict';
// API implementations are explicit. Missing APIs throw, avoiding false compatibility.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL, fileURLToPath } = require('node:url');
const Module = require('node:module');
const config = JSON.parse(process.argv[1]);
const write = packet => {
  const data = Buffer.from(JSON.stringify({jsonrpc:'2.0',...packet}));
  process.stdout.write(Buffer.concat([Buffer.from(`Content-Length: ${data.length}\r\n\r\n`),data]));
};
const emit = event => write({method:'godesktop/event',params:event});
for (const key of ['log','info','warn','error','debug']) console[key] = (...args) => process.stderr.write(args.map(String).join(' ')+'\n');
const strict = (name,value) => new Proxy(value,{get(target,key){if(key in target || typeof key==='symbol')return target[key];throw new Error(`Unsupported VS Code API: ${name}.${String(key)}`);}});
const unsupported = name => strict(name,{then:undefined});
class Disposable {
  constructor(fn=()=>{}){this.fn=fn;}
  dispose(){const fn=this.fn;this.fn=()=>{};fn();}
  static from(...items){return new Disposable(()=>items.forEach(x=>x.dispose()));}
}
class EventEmitter {
  constructor(){this.listeners=new Set();this.event=(fn,thisArg,disposables)=>{const cb=value=>fn.call(thisArg,value);this.listeners.add(cb);const d=new Disposable(()=>this.listeners.delete(cb));disposables?.push(d);return d;};}
  fire(value){for(const fn of [...this.listeners]){try{fn(value);}catch(e){console.error(e);}}}
  dispose(){this.listeners.clear();}
}
class CancellationTokenSource {
  constructor(){this.emitter=new EventEmitter();this.token={isCancellationRequested:false,onCancellationRequested:this.emitter.event};}
  cancel(){if(!this.token.isCancellationRequested){this.token.isCancellationRequested=true;this.emitter.fire();}}
  dispose(){this.cancel();this.emitter.dispose();}
}
class Uri {
  constructor(value){const u=new URL(value);this.value=u.href;this.scheme=u.protocol.slice(0,-1);this.authority=u.host;this.path=decodeURIComponent(u.pathname);this.query=u.search.slice(1);this.fragment=u.hash.slice(1);this.fsPath=this.scheme==='file'?fileURLToPath(u):this.path;}
  static file(value){return new Uri(pathToFileURL(path.resolve(value)).href);}
  static parse(value){return new Uri(value);}
  static joinPath(base,...parts){const u=new URL(base.value);u.pathname=path.posix.join(u.pathname,...parts);return new Uri(u.href);}
  static from(parts){const u=new URL(`${parts.scheme}://${parts.authority||''}/`);u.pathname=parts.path||'/';u.search=parts.query||'';u.hash=parts.fragment||'';return new Uri(u.href);}
  with(parts){return Uri.from({scheme:this.scheme,authority:this.authority,path:this.path,query:this.query,fragment:this.fragment,...parts});}
  toString(){return this.value;} toJSON(){return this.value;}
}
class Position {
  constructor(line,character){if(line<0||character<0)throw new Error('Position must be non-negative');this.line=line;this.character=character;}
  compareTo(p){return Math.sign(this.line-p.line)||Math.sign(this.character-p.character);}
  isBefore(p){return this.compareTo(p)<0;} isBeforeOrEqual(p){return this.compareTo(p)<=0;}
  isAfter(p){return this.compareTo(p)>0;} isAfterOrEqual(p){return this.compareTo(p)>=0;} isEqual(p){return this.compareTo(p)===0;}
  translate(lineDelta=0,characterDelta=0){if(typeof lineDelta==='object')({lineDelta=0,characterDelta=0}=lineDelta);return new Position(this.line+lineDelta,this.character+characterDelta);}
  with(line=this.line,character=this.character){if(typeof line==='object')({line=this.line,character=this.character}=line);return new Position(line,character);}
}
class Range {
  constructor(a,b,c,d){let start=typeof a==='number'?new Position(a,b):a,end=typeof a==='number'?new Position(c,d):b;if(start.isAfter(end))[start,end]=[end,start];this.start=start;this.end=end;}
  get isEmpty(){return this.start.isEqual(this.end);} get isSingleLine(){return this.start.line===this.end.line;}
  contains(value){return value instanceof Range?this.contains(value.start)&&this.contains(value.end):this.start.isBeforeOrEqual(value)&&this.end.isAfterOrEqual(value);}
  isEqual(r){return this.start.isEqual(r.start)&&this.end.isEqual(r.end);}
  intersection(r){const a=this.start.isAfter(r.start)?this.start:r.start,b=this.end.isBefore(r.end)?this.end:r.end;return a.isAfter(b)?undefined:new Range(a,b);}
  union(r){return new Range(this.start.isBefore(r.start)?this.start:r.start,this.end.isAfter(r.end)?this.end:r.end);}
  with(start=this.start,end=this.end){if(!(start instanceof Position))({start=this.start,end=this.end}=start);return new Range(start,end);}
}
class Selection extends Range {
  constructor(a,b,c,d){const anchor=typeof a==='number'?new Position(a,b):a,active=typeof a==='number'?new Position(c,d):b;super(anchor,active);this.anchor=anchor;this.active=active;}
  get isReversed(){return this.anchor.isAfter(this.active);}
}
const position = p => new Position(p.line,p.character);
const range = r => new Range(position(r.start),position(r.end));
class TextEdit {
  constructor(range,newText){this.range=range;this.newText=newText;}
  static replace(range,text){return new TextEdit(range,text);} static insert(position,text){return new TextEdit(new Range(position,position),text);} static delete(range){return new TextEdit(range,'');}
}
class WorkspaceEdit {
  constructor(){this.edits=new Map();}
  replace(uri,range,text){this.set(uri,[...(this.get(uri)||[]),TextEdit.replace(range,text)]);}
  insert(uri,position,text){this.replace(uri,new Range(position,position),text);}
  delete(uri,range){this.replace(uri,range,'');}
  set(uri,edits){this.edits.set(uri.toString(),{uri,edits});}
  get(uri){return this.edits.get(uri.toString())?.edits||[];} has(uri){return this.edits.has(uri.toString());}
  entries(){return [...this.edits.values()].map(x=>[x.uri,x.edits]);} get size(){return this.edits.size;}
}
class CompletionItem {constructor(label,kind){this.label=label;this.kind=kind;}}
class CompletionList {constructor(items=[],isIncomplete=false){this.items=items;this.isIncomplete=isIncomplete;}}
class MarkdownString {constructor(value='',supportThemeIcons=false){this.value=value;this.supportThemeIcons=supportThemeIcons;}appendText(text){this.value+=text.replace(/[\\`*_{}[\]()#+.!-]/g,'\\$&');return this;}appendMarkdown(text){this.value+=text;return this;}appendCodeblock(code,language=''){this.value+=`\n\`\`\`${language}\n${code}\n\`\`\`\n`;return this;}}
class Hover {constructor(contents,range){this.contents=Array.isArray(contents)?contents:[contents];this.range=range;}}
class Location {constructor(uri,range){this.uri=uri;this.range=range instanceof Position?new Range(range,range):range;}}
class Diagnostic {constructor(range,message,severity=0){this.range=range;this.message=message;this.severity=severity;}}

let nextRequest=0;
const pending=new Map(), cancellations=new Map();
function native(method,params){
  if(pending.size>=128)return Promise.reject(new Error('Native request limit reached'));
  const id='native:'+ ++nextRequest;
  return new Promise((resolve,reject)=>{const timeout=setTimeout(()=>{pending.delete(id);reject(new Error('Native request timed out'));},10000);pending.set(id,{resolve,reject,timeout});write({id,method,params});});
}
const events={open:new EventEmitter(),change:new EventEmitter(),close:new EventEmitter(),save:new EventEmitter(),active:new EventEmitter(),selection:new EventEmitter(),configuration:new EventEmitter(),diagnostics:new EventEmitter()};
const documents=new Map(), editors=new Map();
let activeEditor, clientCapabilities={}, storageRoot;
function uriKey(uri){const key=uri.scheme==='file'?'file:'+path.resolve(uri.fsPath):uri.toString();return process.platform==='win32'?key.toLowerCase():key;}
class TextDocument {
  constructor(uri,text,languageId){this.uri=uri;this.fileName=uri.fsPath;this.languageId=languageId||language(uri.fsPath);this.version=1;this.isDirty=false;this.isClosed=false;this.update(text);}
  update(text){this.text=text;this.eol=text.includes('\r\n')?2:1;this.lines=text.split(/\r\n|\n|\r/);this.lineCount=this.lines.length;}
  validatePosition(p){const line=Math.min(this.lineCount-1,Math.max(0,p.line));return new Position(line,Math.min(this.lines[line].length,Math.max(0,p.character)));}
  validateRange(r){return new Range(this.validatePosition(r.start),this.validatePosition(r.end));}
  offsetAt(p){p=this.validatePosition(p);let offset=0;for(let i=0;i<p.line;i++)offset+=this.lines[i].length+(this.eol===2?2:1);return offset+p.character;}
  positionAt(offset){offset=Math.max(0,Math.min(this.text.length,offset));for(let i=0;i<this.lineCount;i++){if(offset<=this.lines[i].length)return new Position(i,offset);offset-=this.lines[i].length+(this.eol===2?2:1);}return new Position(this.lineCount-1,this.lines.at(-1).length);}
  getText(r){if(!r)return this.text;r=this.validateRange(r);return this.text.slice(this.offsetAt(r.start),this.offsetAt(r.end));}
  lineAt(value){const i=value instanceof Position?value.line:value;if(i<0||i>=this.lineCount)throw new Error('Invalid line number');const text=this.lines[i];return {lineNumber:i,text,range:new Range(i,0,i,text.length),rangeIncludingLineBreak:i+1<this.lineCount?new Range(i,0,i+1,0):new Range(i,0,i,text.length),firstNonWhitespaceCharacterIndex:text.search(/\S/)<0?text.length:text.search(/\S/),isEmptyOrWhitespace:!text.trim()};}
  getWordRangeAtPosition(p,regexp=/\w+/g){const line=this.lines[p.line];if(line===undefined)return undefined;const re=new RegExp(regexp.source,regexp.flags.includes('g')?regexp.flags:regexp.flags+'g');for(const match of line.matchAll(re)){if(!match[0].length)break;if(match.index<=p.character && p.character<=match.index+match[0].length)return new Range(p.line,match.index,p.line,match.index+match[0].length);}return undefined;}
  async save(){const result=await native('workspace/saveDocument',{path:this.fileName,version:this.version});if(result.document)syncDocument(result.document,'save');return result.saved===true;}
}
function language(file){return ({'.go':'go','.js':'javascript','.cjs':'javascript','.ts':'typescript','.json':'json','.md':'markdown','.py':'python','.rs':'rust'})[path.extname(file)]||'plaintext';}
function document(value){
  const uri=typeof value==='string'?Uri.file(value):value,key=uriKey(uri);
  let doc=documents.get(key);if(doc && !doc.isClosed)return doc;
  const text=fs.readFileSync(uri.fsPath,'utf8');if(text.length>1<<20 || text.includes('\0'))throw new Error('Document is not supported UTF-8 text');
  doc=new TextDocument(uri,text);documents.set(key,doc);events.open.fire(doc);return doc;
}
class TextEditor {
  constructor(doc){this.document=doc;this._selection=new Selection(0,0,0,0);this.options={tabSize:4,insertSpaces:true};this.viewColumn=1;this.visibleRanges=[new Range(0,0,doc.lineCount-1,doc.lines.at(-1).length)];}
  get selection(){return this._selection;}
  set selection(value){this._selection=value;void native('window/setSelection',{path:this.document.fileName,selection:value}).catch(e=>console.error(e));}
  get selections(){return [this._selection];}set selections(value){if(value.length!==1)throw new Error('Multiple selections are not supported yet');this.selection=value[0];}
  async edit(callback){const edits=[];callback({replace:(r,t)=>edits.push(TextEdit.replace(r,t)),insert:(p,t)=>edits.push(TextEdit.insert(p,t)),delete:r=>edits.push(TextEdit.delete(r))});return applyEditEntries([[this.document.uri,edits]]);}
  revealRange(r){emit({type:'reveal',path:this.document.fileName,data:r});}
}
function editorFor(doc){const key=uriKey(doc.uri);if(!editors.has(key))editors.set(key,new TextEditor(doc));return editors.get(key);}
function syncDocument(value,kind,changes=[]){
  if(kind==='focus' && !value){activeEditor=undefined;events.active.fire(undefined);return;}
  const uri=Uri.file(value.path),key=uriKey(uri);let doc=documents.get(key),fresh=!doc||doc.isClosed;
  if(kind==='close'){if(doc){doc.isClosed=true;events.close.fire(doc);}documents.delete(key);editors.delete(key);return;}
  if(fresh){doc=new TextDocument(uri,value.text,value.languageId);documents.set(key,doc);}
  if(value.version<doc.version)return;
  const changed=!fresh && value.version>doc.version;
  const contentChanges=changes.map(c=>({range:range(c.range),rangeOffset:doc.offsetAt(position(c.range.start)),rangeLength:c.rangeLength,text:c.text}));
  if(value.text!==undefined)doc.update(value.text);
  else if(changed){if(value.version!==doc.version+1)throw new Error('Document synchronization skipped a version');let text=doc.text;for(const c of changes){const a=doc.offsetAt(position(c.range.start)),z=doc.offsetAt(position(c.range.end));text=text.slice(0,a)+c.text+text.slice(z);}doc.update(text);}
  doc.version=value.version;doc.isDirty=!!value.dirty;
  const editor=editorFor(doc);if(value.selection)editor._selection=new Selection(position(value.selection.anchor),position(value.selection.active));
  if(fresh)events.open.fire(doc);
  if(changed)events.change.fire({document:doc,contentChanges,reason:value.reason});
  if(kind==='save')events.save.fire(doc);
  if(kind==='focus'){activeEditor=editor;events.active.fire(editor);}
  if(kind==='selection')events.selection.fire({textEditor:editor,selections:editor.selections,kind:1});
}
async function applyEditEntries(entries){
  const documentsToEdit=entries.map(([uri,edits])=>{const doc=document(uri);return {path:doc.fileName,version:doc.version,edits};});
  const result=await native('workspace/applyEdit',{documents:documentsToEdit});
  if(result.applied){for(const doc of result.documents||[])syncDocument(doc,'change',doc.changes||[]);}
  return result.applied===true;
}
const registrations=new Map(),active=new Map(),activating=new Map(),providers=[];
const folders=[{uri:Uri.file(config.workspace),name:path.basename(config.workspace),index:0}];
const message=type=>async(text,...items)=>{emit({type,text:String(text)});return undefined;};
const settingsFile=()=>path.join(config.workspace,'.vscode','settings.json');
function readSettings(file){try{return JSON.parse(fs.readFileSync(file,'utf8'));}catch(e){if(e.code==='ENOENT')return {};throw e;}}
function getConfiguration(section=''){
  const prefix=section?section+'.':'';
  const load=()=>({...storageRoot?readSettings(path.join(storageRoot,'settings.json')):{},...readSettings(settingsFile())});
  return {
    get:(key,fallback)=>load()[prefix+key]??fallback,has:key=>Object.hasOwn(load(),prefix+key),
    inspect:key=>{const full=prefix+key;return {key:full,globalValue:storageRoot?readSettings(path.join(storageRoot,'settings.json'))[full]:undefined,workspaceValue:readSettings(settingsFile())[full]};},
    async update(key,value,target=false){const file=target===true||target===1?path.join(storageRoot,'settings.json'):settingsFile();const data=readSettings(file),full=prefix+key;if(value===undefined)delete data[full];else data[full]=value;fs.mkdirSync(path.dirname(file),{recursive:true});fs.writeFileSync(file,JSON.stringify(data,null,2)+'\n');events.configuration.fire({affectsConfiguration:other=>full===other||full.startsWith(other+'.')});emit({type:'configuration',text:full});}
  };
}
function matches(selector,doc){
  if(Array.isArray(selector))return selector.some(s=>matches(s,doc));if(typeof selector==='string')return selector==='*'||selector===doc.languageId;
  if(!selector)return false;if(selector.language && selector.language!=='*' && selector.language!==doc.languageId)return false;if(selector.scheme && selector.scheme!==doc.uri.scheme)return false;
  if(selector.pattern){const pattern=typeof selector.pattern==='string'?selector.pattern:selector.pattern.pattern;const regex='^'+pattern.split('**').map(part=>part.split('*').map(p=>p.replace(/[.+?^${}()|[\]\\]/g,'\\$&')).join('[^/]*')).join('.*')+'$';if(!new RegExp(regex).test(doc.uri.fsPath.replace(/\\/g,'/'))&&!new RegExp(regex).test(path.basename(doc.uri.fsPath)))return false;}
  return true;
}
function registerProvider(kind,selector,provider){const entry={kind,selector,provider};providers.push(entry);return new Disposable(()=>{const i=providers.indexOf(entry);if(i>=0)providers.splice(i,1);});}
async function activateLanguage(doc){for(const e of config.extensions)if((e.manifest.activationEvents||[]).includes('onLanguage:'+doc.languageId))await activate(e);}
const vscode=strict('vscode',{
  version:'1.140.0',Disposable,EventEmitter,CancellationTokenSource,Uri,Position,Range,Selection,TextEdit,WorkspaceEdit,CompletionItem,CompletionList,MarkdownString,Hover,Location,Diagnostic,
  StatusBarAlignment:{Left:1,Right:2},ExtensionMode:{Production:1,Development:2,Test:3},EndOfLine:{LF:1,CRLF:2},ConfigurationTarget:{Global:1,Workspace:2,WorkspaceFolder:3},
  DiagnosticSeverity:{Error:0,Warning:1,Information:2,Hint:3},TextDocumentChangeReason:{Undo:1,Redo:2},CompletionTriggerKind:{Invoke:0,TriggerCharacter:1,TriggerForIncompleteCompletions:2},
  CompletionItemKind:Object.fromEntries(['Text','Method','Function','Constructor','Field','Variable','Class','Interface','Module','Property','Unit','Value','Enum','Keyword','Snippet','Color','File','Reference','Folder','EnumMember','Constant','Struct','Event','Operator','TypeParameter','User','Issue'].map((x,i)=>[x,i])),
  commands:strict('commands',{
    registerCommand(id,fn,thisArg){if(registrations.has(id))throw new Error(`Duplicate command: ${id}`);registrations.set(id,(...args)=>fn.apply(thisArg,args));return new Disposable(()=>registrations.delete(id));},
    async executeCommand(id,...args){await activateFor(id);const fn=registrations.get(id);if(!fn)throw new Error(`Command not found: ${id}`);return await fn(...args);},
    async getCommands(){return [...new Set([...registrations.keys(),...config.extensions.flatMap(e=>e.manifest.contributes?.commands?.map(c=>c.command)||[])])];}
  }),
  window:strict('window',{
    get activeTextEditor(){return activeEditor;},get visibleTextEditors(){return [...editors.values()];},onDidChangeActiveTextEditor:events.active.event,onDidChangeTextEditorSelection:events.selection.event,
    showInformationMessage:message('information'),showWarningMessage:message('warning'),showErrorMessage:message('error'),
    createOutputChannel(name){return {name,append:text=>emit({type:'output',channel:name,text:String(text)}),appendLine:text=>emit({type:'output',channel:name,text:String(text)+'\n'}),clear:()=>emit({type:'clear',channel:name,text:''}),show:()=>emit({type:'panel',channel:name,text:''}),hide(){},dispose(){}};},
    createStatusBarItem(){return {text:'',tooltip:'',command:undefined,show(){emit({type:'status',text:this.text});},hide(){emit({type:'status',text:''});},dispose(){this.hide();}};},
    async showTextDocument(value){let doc=value instanceof Uri||typeof value==='string'?document(value):value;if(clientCapabilities.showDocument){const result=await native('window/showTextDocument',{path:doc.fileName});syncDocument(result.document,'focus');doc=document(doc.uri);}else{emit({type:'open',path:doc.uri.fsPath,text:''});activeEditor=editorFor(doc);events.active.fire(activeEditor);}return editorFor(doc);}
  }),
  workspace:strict('workspace',{
    workspaceFolders:folders,rootPath:config.workspace,get textDocuments(){return [...documents.values()];},onDidOpenTextDocument:events.open.event,onDidChangeTextDocument:events.change.event,onDidCloseTextDocument:events.close.event,onDidSaveTextDocument:events.save.event,onDidChangeConfiguration:events.configuration.event,
    async openTextDocument(value){return document(value);},applyEdit:async edit=>applyEditEntries(edit.entries()),getConfiguration,
    getWorkspaceFolder(uri){return folders.find(f=>{const rel=path.relative(f.uri.fsPath,uri.fsPath);return !rel.startsWith('..')&&!path.isAbsolute(rel);});},asRelativePath:value=>path.relative(config.workspace,value instanceof Uri?value.fsPath:value),
    fs:{readFile:async uri=>new Uint8Array(await fs.promises.readFile(uri.fsPath)),writeFile:async(uri,data)=>fs.promises.writeFile(uri.fsPath,data),createDirectory:async uri=>fs.promises.mkdir(uri.fsPath,{recursive:true}),readDirectory:async uri=>(await fs.promises.readdir(uri.fsPath,{withFileTypes:true})).map(e=>[e.name,e.isDirectory()?2:e.isSymbolicLink()?64:1]),delete:async(uri,options={})=>fs.promises.rm(uri.fsPath,{recursive:!!options.recursive}),stat:async uri=>{const s=await fs.promises.stat(uri.fsPath);return {type:s.isDirectory()?2:1,ctime:s.ctimeMs,mtime:s.mtimeMs,size:s.size};}}
  }),
  env:strict('env',{appName:'gocode',appHost:'desktop',language:'en',uiKind:1}),
  languages:strict('languages',{
    registerCompletionItemProvider:(selector,provider)=>registerProvider('completion',selector,provider),registerHoverProvider:(selector,provider)=>registerProvider('hover',selector,provider),registerDefinitionProvider:(selector,provider)=>registerProvider('definition',selector,provider),
    createDiagnosticCollection(name=''){const values=new Map();const publish=(uri,data)=>{emit({type:'diagnostics',channel:name,path:uri.fsPath,data});events.diagnostics.fire({uris:[uri]});};return {name,set(uri,diagnostics){if(Array.isArray(uri)){for(const entry of uri)this.set(...entry);return;}values.set(uri.toString(),[uri,diagnostics||[]]);publish(uri,diagnostics||[]);},get:uri=>values.get(uri.toString())?.[1],has:uri=>values.has(uri.toString()),delete(uri){values.delete(uri.toString());publish(uri,[]);},clear(){for(const [uri] of values.values())publish(uri,[]);values.clear();},dispose(){this.clear();},forEach(fn,thisArg){for(const [uri,diagnostics] of values.values())fn.call(thisArg,uri,diagnostics,this);},[Symbol.iterator](){return [...values.values()][Symbol.iterator]();}};},onDidChangeDiagnostics:events.diagnostics.event,
    getLanguages:async()=>[...new Set([...documents.values()].map(d=>d.languageId))]
  }),debug:unsupported('debug'),tasks:unsupported('tasks')
});
const originalLoad=Module._load;Module._load=function(request,...args){return request==='vscode'?vscode:originalLoad.call(this,request,...args);};
function memento(file){
  const values=file?readSettings(file):{};let saving=Promise.resolve();
  return {get:(key,fallback)=>Object.hasOwn(values,key)?values[key]:fallback,keys:()=>Object.keys(values),setKeysForSync(){},update(key,value){if(value===undefined)delete values[key];else values[key]=value;if(!file)return Promise.resolve();const data=JSON.stringify(values);saving=saving.then(async()=>{await fs.promises.mkdir(path.dirname(file),{recursive:true});const temporary=file+'.'+crypto.randomUUID()+'.tmp';await fs.promises.writeFile(temporary,data);await fs.promises.rename(temporary,file);});return saving;}};
}
async function activate(extension){
  const id=extension.manifest.publisher+'.'+extension.manifest.name;
  if(active.has(id))return;if(activating.has(id))return activating.get(id);
  const promise=(async()=>{
    if(!extension.manifest.main)throw new Error(`${id}: no desktop main entry`);
    const location=path.resolve(extension.path,extension.manifest.main),relative=path.relative(extension.path,location);if(relative.startsWith('..')||path.isAbsolute(relative))throw new Error('Extension main escapes installation directory');
    const subscriptions=[],workspaceHash=crypto.createHash('sha256').update(config.workspace).digest('hex').slice(0,24);
    const globalPath=storageRoot?path.join(storageRoot,'globalStorage',id):undefined,workspacePath=storageRoot?path.join(storageRoot,'workspaceStorage',workspaceHash,id):undefined;
    const context={subscriptions,extensionPath:extension.path,extensionUri:Uri.file(extension.path),asAbsolutePath:p=>path.join(extension.path,p),workspaceState:memento(workspacePath&&path.join(workspacePath,'state.json')),globalState:memento(globalPath&&path.join(globalPath,'state.json')),globalStorageUri:globalPath&&Uri.file(globalPath),storageUri:workspacePath&&Uri.file(workspacePath),extensionMode:1,environmentVariableCollection:unsupported('environmentVariableCollection'),secrets:unsupported('secrets')};
    try{const module=require(location),exports=await module.activate?.(context);active.set(id,{module,subscriptions,exports});emit({type:'activated',text:id});}catch(error){subscriptions.forEach(x=>x.dispose());throw error;}
  })();activating.set(id,promise);try{await promise;}finally{activating.delete(id);}
}
async function activateFor(command){for(const e of config.extensions)if((e.manifest.contributes?.commands||[]).some(c=>c.command===command)||(e.manifest.activationEvents||[]).includes('onCommand:'+command))await activate(e);}
async function handle(request){
  if(!request.method){const p=pending.get(request.id);if(p){pending.delete(request.id);clearTimeout(p.timeout);request.error?p.reject(new Error(request.error.message)):p.resolve(request.result);}return;}
  if(request.method==='$/cancelRequest'){cancellations.get(request.params?.id)?.cancel();return;}
  const source=new CancellationTokenSource();cancellations.set(request.id,source);
  try{
    let result;
    if(request.method==='initialize'){
      clientCapabilities=request.params?.clientCapabilities||{};storageRoot=request.params?.storageRoot;
      for(const d of request.params?.documents||[])syncDocument(d,'open');if(request.params?.active)syncDocument(request.params.active,'focus');
      for(const e of config.extensions)if((e.manifest.activationEvents||[]).some(x=>x==='*'||x==='onStartupFinished'))await activate(e);
      result=await vscode.commands.getCommands();
    }else if(request.method==='execute')result=await vscode.commands.executeCommand(request.params.command,...(request.params.args||[]));
    else if(request.method==='syncDocument'){syncDocument(request.params.document,request.params.kind,request.params.changes||[]);if(request.params.document && request.params.kind!=='close')await activateLanguage(document(Uri.file(request.params.document.path)));result=true;}
    else if(['provideCompletionItems','provideHover','provideDefinition'].includes(request.method)){
      const doc=document(Uri.parse(request.params.uri));await activateLanguage(doc);
      const kind=request.method==='provideCompletionItems'?'completion':request.method==='provideHover'?'hover':'definition';
      const values=[];
      for(const {kind:providerKind,selector,provider} of [...providers]){if(providerKind!==kind||!matches(selector,doc)||source.token.isCancellationRequested)continue;const value=await provider[request.method](doc,position(request.params.position),source.token,request.params.context||{triggerKind:0});if(value!=null)values.push(...(Array.isArray(value)?value:kind==='completion'?value.items||[]:[value]));}
      result=values;
    }else throw new Error(`Unknown method: ${request.method}`);
    if(request.id!==undefined)write({id:request.id,result:result??null});
  }catch(error){if(request.id!==undefined)write({id:request.id,error:{code:-32603,message:error?.stack||String(error)}});else emit({type:'error',text:String(error)});}
  finally{source.dispose();cancellations.delete(request.id);}
}
let buffer=Buffer.alloc(0),expected;
process.stdin.on('data',chunk=>{
  buffer=Buffer.concat([buffer,chunk]);
  while(true){
    if(expected===undefined){const end=buffer.indexOf('\r\n\r\n');if(end<0){if(buffer.length>8192)process.exit(2);return;}const match=/^Content-Length: ([0-9]+)$/im.exec(buffer.subarray(0,end).toString());if(!match || +match[1]>16<<20)process.exit(2);expected=+match[1];buffer=buffer.subarray(end+4);}
    if(buffer.length<expected)return;
    const data=buffer.subarray(0,expected);buffer=buffer.subarray(expected);expected=undefined;
    try{void handle(JSON.parse(data.toString()));}catch(e){emit({type:'error',text:String(e)});}
  }
}).on('end',async()=>{
  for(const p of pending.values()){clearTimeout(p.timeout);p.reject(new Error('Extension host closed'));}pending.clear();
  for(const {module,subscriptions} of active.values()){try{await module.deactivate?.();}catch{}subscriptions.forEach(x=>x.dispose());}
});
