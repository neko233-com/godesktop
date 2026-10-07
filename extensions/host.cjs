'use strict';
// API implementations are explicit. Missing APIs throw, avoiding false compatibility.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const { pathToFileURL, fileURLToPath } = require('node:url');
const Module = require('node:module');
const invocation = new (require('node:async_hooks').AsyncLocalStorage)();
const config = JSON.parse(fs.readFileSync(process.argv[2],'utf8'));
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
  constructor(line,character){if(!Number.isSafeInteger(line)||!Number.isSafeInteger(character)||line<0||character<0)throw new Error('Position must contain non-negative integers');this.line=line;this.character=character;}
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
const events={open:new EventEmitter(),change:new EventEmitter(),close:new EventEmitter(),save:new EventEmitter(),active:new EventEmitter(),selection:new EventEmitter(),visible:new EventEmitter(),column:new EventEmitter(),ranges:new EventEmitter(),configuration:new EventEmitter(),diagnostics:new EventEmitter()};
const documents=new Map(), editors=new Map();
let activeEditor, visibleEditors=[], editorGeneration=0, clientCapabilities={}, storageRoot;
function uriKey(uri){const key=uri.scheme==='file'?'file:'+path.resolve(uri.fsPath):uri.toString();return process.platform==='win32'?key.toLowerCase():key;}
class TextDocument {
  constructor(uri,text,languageId,instance){this.uri=uri;this.fileName=uri.fsPath;this.languageId=languageId||language(uri.fsPath);this._instance=instance;this.version=1;this.isDirty=false;this.isClosed=false;this.update(text);}
  update(text){this.text=text;this.eol=text.includes('\r\n')?2:1;this.lines=text.split(/\r\n|\n|\r/);this.lineCount=this.lines.length;}
  validatePosition(p){const line=Math.min(this.lineCount-1,Math.max(0,p.line));return new Position(line,Math.min(this.lines[line].length,Math.max(0,p.character)));}
  validateRange(r){return new Range(this.validatePosition(r.start),this.validatePosition(r.end));}
  offsetAt(p){p=this.validatePosition(p);let offset=0;for(let i=0;i<p.line;i++)offset+=this.lines[i].length+(this.eol===2?2:1);return offset+p.character;}
  positionAt(offset){offset=Math.max(0,Math.min(this.text.length,offset));for(let i=0;i<this.lineCount;i++){if(offset<=this.lines[i].length)return new Position(i,offset);offset-=this.lines[i].length+(this.eol===2?2:1);}return new Position(this.lineCount-1,this.lines.at(-1).length);}
  getText(r){if(!r)return this.text;r=this.validateRange(r);return this.text.slice(this.offsetAt(r.start),this.offsetAt(r.end));}
  lineAt(value){const i=value instanceof Position?value.line:value;if(i<0||i>=this.lineCount)throw new Error('Invalid line number');const text=this.lines[i];return {lineNumber:i,text,range:new Range(i,0,i,text.length),rangeIncludingLineBreak:i+1<this.lineCount?new Range(i,0,i+1,0):new Range(i,0,i,text.length),firstNonWhitespaceCharacterIndex:text.search(/\S/)<0?text.length:text.search(/\S/),isEmptyOrWhitespace:!text.trim()};}
  getWordRangeAtPosition(p,regexp=/\w+/g){const line=this.lines[p.line];if(line===undefined)return undefined;const re=new RegExp(regexp.source,regexp.flags.includes('g')?regexp.flags:regexp.flags+'g');for(const match of line.matchAll(re)){if(!match[0].length)break;if(match.index<=p.character && p.character<=match.index+match[0].length)return new Range(p.line,match.index,p.line,match.index+match[0].length);}return undefined;}
  async save(){if(this.isClosed)return false;const result=await native('workspace/saveDocument',{path:this.fileName,version:this.version,instance:this._instance});if(result.document)syncDocument(result.document,result.saved===true?'save':'ack');return result.saved===true;}
}
function language(file){return ({'.go':'go','.js':'javascript','.cjs':'javascript','.ts':'typescript','.json':'json','.md':'markdown','.py':'python','.rs':'rust'})[path.extname(file)]||'plaintext';}
function readDocument(file){const limit=2<<20,handle=fs.openSync(file,'r');try{const stat=fs.fstatSync(handle);if(!stat.isFile()||stat.size>limit)throw new Error('Document exceeds the 2 MiB UTF-8 policy');const bytes=Buffer.allocUnsafe(limit+1);let used=0;while(used<bytes.length){const n=fs.readSync(handle,bytes,used,bytes.length-used,null);if(!n)break;used+=n;}if(used>limit)throw new Error('Document exceeds the 2 MiB UTF-8 policy');const text=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(bytes.subarray(0,used));if(text.includes('\0'))throw new Error('Document contains NUL');return text;}finally{fs.closeSync(handle);}}
function document(value){
  const uri=typeof value==='string'?Uri.file(value):value,key=uriKey(uri);
  let doc=documents.get(key);if(doc && !doc.isClosed)return doc;
  const text=readDocument(uri.fsPath);
  doc=new TextDocument(uri,text);documents.set(key,doc);events.open.fire(doc);return doc;
}
class TextEditor {
  constructor(doc,id){this.document=doc;this._id=id;this._disposed=false;this._selection=new Selection(0,0,0,0);this._nativeSelection=this._selection;this._selectionRequest=0;this._operation=Promise.resolve();this._operationCount=0;this.options={tabSize:4,insertSpaces:true};this.viewColumn=1;this.visibleRanges=[];}
  _enqueue(fn){if(this._operationCount>=128)throw new Error('Native editor operation limit reached');this._operationCount++;const result=this._operation.then(()=>{if(this._disposed)throw new Error('TextEditor is disposed');return fn();}).finally(()=>this._operationCount--);this._operation=result.catch(()=>{});return result;}
  get selection(){return this._selection;}
  set selection(value){if(this._disposed)throw new Error('TextEditor is disposed');if(!(value instanceof Selection))throw new Error('Invalid Selection');const request=++this._selectionRequest,version=this.document.version;const operation=this._enqueue(()=>native('window/setSelection',{path:this.document.fileName,editorId:clientCapabilities.editorGroups?this._id:undefined,version,selection:value}));this._selection=value;void operation.then(result=>{if(result?.editors)syncEditors(result.editors);}).catch(e=>{if(request===this._selectionRequest)this._selection=this._nativeSelection;console.error(e);});}
  get selections(){return [this._selection];}set selections(value){if(value.length!==1)throw new Error('Multiple selections are not supported yet');this.selection=value[0];}
  async edit(callback,options={}){if(this._disposed)return false;if(options.undoStopBefore===false||options.undoStopAfter===false)throw new Error('Unsupported TextEditor.edit undo-stop merging');const version=this.document.version,edits=[];let valid=true;const add=e=>{if(!valid)throw new Error('TextEditorEdit is no longer valid');edits.push(e);};try{callback({replace:(r,t)=>add(TextEdit.replace(r,t)),insert:(p,t)=>add(TextEdit.insert(p,t)),delete:r=>add(TextEdit.delete(r))});}finally{valid=false;}return this._enqueue(()=>this.document.version===version?applyEditEntries([[this.document.uri,edits]],clientCapabilities.editorGroups?this._id:undefined):false);}
  revealRange(r,revealType=0){if(this._disposed)throw new Error('TextEditor is disposed');if(clientCapabilities.editorGroups){const version=this.document.version;void this._enqueue(()=>native('window/revealRange',{path:this.document.fileName,editorId:this._id,version,range:r,revealType})).then(result=>{if(result?.editors)syncEditors(result.editors);}).catch(e=>console.error(e));}else emit({type:'reveal',path:this.document.fileName,data:r});}
}
function editorFor(doc){const key='legacy:'+uriKey(doc.uri);if(!editors.has(key))editors.set(key,new TextEditor(doc,key));return editors.get(key);}
function setLegacyActive(editor){const changed=activeEditor!==editor,previous=visibleEditors;activeEditor=editor;visibleEditors=editor?[editor]:[];if(changed)events.active.fire(editor);if(previous.length!==visibleEditors.length||previous[0]!==editor)events.visible.fire([...visibleEditors]);}
function syncEditors(state){
  if(!state||!Number.isSafeInteger(state.generation)||state.generation<=editorGeneration)return;
  if(!Array.isArray(state.editors)||state.editors.length>9)throw new Error('Invalid native editor count');
  const ids=new Set(), entries=state.editors.map(value=>{const doc=documents.get(uriKey(Uri.file(value.path)));if(!value.id||typeof value.id!=='string'||ids.has(value.id)||!doc||doc.isClosed||!Number.isInteger(value.viewColumn)||value.viewColumn<1||value.viewColumn>9||!value.selection||!Array.isArray(value.visibleRanges)||value.visibleRanges.length>256)throw new Error('Invalid native editor state');ids.add(value.id);const old=editors.get(value.id);if(old&&old.document!==doc)throw new Error('Editor identity changed its document');const selection=new Selection(position(value.selection.anchor),position(value.selection.active)),ranges=value.visibleRanges.map(range);for(const p of [selection.anchor,selection.active,...ranges.flatMap(r=>[r.start,r.end])])if(!doc.validatePosition(p).isEqual(p))throw new Error('Native editor coordinates exceed its document');return {value,doc,selection,ranges,editor:old||new TextEditor(doc,value.id)};});
  if(state.active&&!ids.has(state.active))throw new Error('Active native editor is not visible');
  const previous=visibleEditors,next=entries.map(x=>x.editor),notifications=[];
  for(const [id,editor] of editors){if(!ids.has(id)){editor._disposed=true;editor.viewColumn=undefined;editors.delete(id);}}
  for(const {value,editor,selection,ranges} of entries){if(!editor._nativeSelection.anchor.isEqual(selection.anchor)||!editor._nativeSelection.active.isEqual(selection.active))notifications.push(()=>events.selection.fire({textEditor:editor,selections:editor.selections,kind:state.selectionKind||undefined}));if(editor.viewColumn!==value.viewColumn)notifications.push(()=>events.column.fire({textEditor:editor,viewColumn:editor.viewColumn}));if(JSON.stringify(editor.visibleRanges)!==JSON.stringify(ranges))notifications.push(()=>events.ranges.fire({textEditor:editor,visibleRanges:editor.visibleRanges}));editor._selection=selection;editor._nativeSelection=selection;editor.viewColumn=value.viewColumn;editor.visibleRanges=ranges;editors.set(value.id,editor);}
  const nextActive=editors.get(state.active),activeChanged=activeEditor!==nextActive;
  activeEditor=nextActive;visibleEditors=next;editorGeneration=state.generation;
  if(previous.length!==next.length||previous.some((e,i)=>e!==next[i]))events.visible.fire([...next]);
  if(activeChanged)events.active.fire(nextActive);
  for(const notify of notifications)notify();
}
function syncDocument(value,kind,changes=[],layout){
  if(kind==='focus' && !value){if(clientCapabilities.editorGroups)syncEditors(layout);else setLegacyActive(undefined);return;}
  const uri=Uri.file(value.path),key=uriKey(uri);let doc=documents.get(key),fresh=!doc||doc.isClosed;
  if(value.instance&&(!/^\d+$/.test(value.instance)||typeof value.instance!=='string'))throw new Error('Invalid document instance');
  if(kind==='close'){if(doc&&value.instance&&doc._instance&&value.instance!==doc._instance)return;if(doc)doc.isClosed=true;documents.delete(key);if(clientCapabilities.editorGroups)syncEditors(layout);else{for(const [id,e] of editors)if(e.document===doc){e._disposed=true;editors.delete(id);}if(activeEditor?.document===doc)setLegacyActive(undefined);}if(doc)events.close.fire(doc);return;}
  let replaced;
  if(!fresh&&value.instance&&doc._instance&&value.instance!==doc._instance){if(BigInt(value.instance)<BigInt(doc._instance))return;replaced=doc;fresh=true;}
  if(fresh){if(typeof value.text!=='string')throw new Error('New document instance requires a full snapshot');doc=new TextDocument(uri,value.text,value.languageId,value.instance);documents.set(key,doc);if(replaced)replaced.isClosed=true;}
  if(value.version<doc.version)return;
  const changed=!fresh && value.version>doc.version;
  const contentChanges=changes.map(c=>({range:range(c.range),rangeOffset:doc.offsetAt(position(c.range.start)),rangeLength:c.rangeLength,text:c.text}));
  if(value.text!==undefined)doc.update(value.text);
  else if(changed){if(value.version!==doc.version+1)throw new Error('Document synchronization skipped a version');let text=doc.text;for(const c of changes){const a=doc.offsetAt(position(c.range.start)),z=doc.offsetAt(position(c.range.end));text=text.slice(0,a)+c.text+text.slice(z);}doc.update(text);}
  doc.version=value.version;doc.isDirty=!!value.dirty;
  let editor;if(clientCapabilities.editorGroups)syncEditors(layout);else{editor=editorFor(doc);if(value.selection)editor._selection=new Selection(position(value.selection.anchor),position(value.selection.active));}
  if(replaced)events.close.fire(replaced);
  if(fresh)events.open.fire(doc);
  if(changed)events.change.fire({document:doc,contentChanges,reason:value.reason});
  // Native save notifications can race the awaited RPC acknowledgement. A save
  // identity makes both paths update state while firing each event exactly once.
  if(kind==='save' && (value.saveId===undefined || value.saveId>(doc.lastSaveId??-1))){
    doc.lastSaveId=value.saveId;events.save.fire(doc);
  }
  if(kind==='focus'&&!clientCapabilities.editorGroups)setLegacyActive(editor);
  if(kind==='selection'&&!clientCapabilities.editorGroups)events.selection.fire({textEditor:editor,selections:editor.selections,kind:1});
}
async function applyEditEntries(entries,editorId){
  const documentsToEdit=entries.map(([uri,edits])=>{const doc=document(uri);return {path:doc.fileName,version:doc.version,instance:doc._instance,edits,editorId};});
  const result=await native('workspace/applyEdit',{documents:documentsToEdit});
  if(result.applied){for(const doc of result.documents||[])syncDocument(doc,'change',doc.changes||[]);if(result.editors)syncEditors(result.editors);}
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
  ViewColumn:{Active:-1,Beside:-2,One:1,Two:2,Three:3,Four:4,Five:5,Six:6,Seven:7,Eight:8,Nine:9},TextEditorRevealType:{Default:0,InCenter:1,InCenterIfOutsideViewport:2,AtTop:3},TextEditorSelectionChangeKind:{Keyboard:1,Mouse:2,Command:3},
  CompletionItemKind:Object.fromEntries(['Text','Method','Function','Constructor','Field','Variable','Class','Interface','Module','Property','Unit','Value','Enum','Keyword','Snippet','Color','File','Reference','Folder','EnumMember','Constant','Struct','Event','Operator','TypeParameter','User','Issue'].map((x,i)=>[x,i])),
  commands:strict('commands',{
    registerCommand(id,fn,thisArg){if(registrations.has(id))throw new Error(`Duplicate command: ${id}`);registrations.set(id,(...args)=>fn.apply(thisArg,args));return new Disposable(()=>registrations.delete(id));},
    async executeCommand(id,...args){await activateFor(id);const fn=registrations.get(id);if(!fn)throw new Error(`Command not found: ${id}`);return await fn(...args);},
    async getCommands(){return [...new Set([...registrations.keys(),...config.extensions.flatMap(e=>e.manifest.contributes?.commands?.map(c=>c.command)||[])])];}
  }),
  window:strict('window',{
    get activeTextEditor(){return activeEditor;},get visibleTextEditors(){return [...visibleEditors];},onDidChangeActiveTextEditor:events.active.event,onDidChangeTextEditorSelection:events.selection.event,onDidChangeVisibleTextEditors:events.visible.event,onDidChangeTextEditorViewColumn:events.column.event,onDidChangeTextEditorVisibleRanges:events.ranges.event,
    showInformationMessage:message('information'),showWarningMessage:message('warning'),showErrorMessage:message('error'),
    createOutputChannel(name){return {name,append:text=>emit({type:'output',channel:name,text:String(text)}),appendLine:text=>emit({type:'output',channel:name,text:String(text)+'\n'}),clear:()=>emit({type:'clear',channel:name,text:''}),show:()=>emit({type:'panel',channel:name,text:''}),hide(){},dispose(){}};},
    createStatusBarItem(){return {text:'',tooltip:'',command:undefined,show(){emit({type:'status',text:this.text});},hide(){emit({type:'status',text:''});},dispose(){this.hide();}};},
    async showTextDocument(value,columnOrOptions,preserveFocus){let doc=value instanceof Uri||typeof value==='string'?await vscode.workspace.openTextDocument(value):value;const options=typeof columnOrOptions==='number'?{viewColumn:columnOrOptions,preserveFocus:!!preserveFocus}:columnOrOptions||{};if(!clientCapabilities.editorGroups&&((options.viewColumn&&options.viewColumn!==1&&options.viewColumn!==-1)||options.preserveFocus||options.selection))throw new Error('Native editor group/show options are unavailable');if(clientCapabilities.showDocument){const receipts=invocation.getStore()?.receipts,key=uriKey(doc.uri),focusReceipt=receipts?.get(key);receipts?.delete(key);const result=await native('window/showTextDocument',{path:doc.fileName,...options,focusReceipt});syncDocument(result.document,clientCapabilities.editorGroups?'ack':'focus',[],result.editors);if(clientCapabilities.editorGroups){const editor=editors.get(result.editorId);if(!editor||editor._disposed)throw new Error('Native editor was superseded before acknowledgement');return editor;}doc=document(doc.uri);}else{emit({type:'open',path:doc.uri.fsPath,text:''});setLegacyActive(editorFor(doc));}return editorFor(doc);}
  }),
  workspace:strict('workspace',{
    workspaceFolders:folders,rootPath:config.workspace,get textDocuments(){return [...documents.values()];},onDidOpenTextDocument:events.open.event,onDidChangeTextDocument:events.change.event,onDidCloseTextDocument:events.close.event,onDidSaveTextDocument:events.save.event,onDidChangeConfiguration:events.configuration.event,
    async openTextDocument(value){const uri=typeof value==='string'?Uri.file(value):value;if(clientCapabilities.openDocument){const result=await native('workspace/openTextDocument',{path:uri.fsPath});syncDocument(result.document,'ack',[],result.editors);const doc=document(Uri.file(result.document.path)),receipts=invocation.getStore()?.receipts;if(result.focusReceipt&&receipts){const key=uriKey(doc.uri);if(receipts.size>=128&&!receipts.has(key))throw new Error('Native opening receipt limit reached');receipts.set(key,result.focusReceipt);}return doc;}return document(uri);},applyEdit:async edit=>applyEditEntries(edit.entries()),getConfiguration,
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
      for(const d of request.params?.documents||[])syncDocument(d,'open');if(clientCapabilities.editorGroups)syncEditors(request.params?.editors);else if(request.params?.active)syncDocument(request.params.active,'focus');
      for(const e of config.extensions)if((e.manifest.activationEvents||[]).some(x=>x==='*'||x==='onStartupFinished'))await activate(e);
      result=await vscode.commands.getCommands();
    }else if(request.method==='execute')result=await vscode.commands.executeCommand(request.params.command,...(request.params.args||[]));
    else if(request.method==='syncDocument'){syncDocument(request.params.document,request.params.kind,request.params.changes||[],request.params.editors);if(request.params.document && request.params.kind!=='close')await activateLanguage(document(Uri.file(request.params.document.path)));result=true;}
    else if(request.method==='syncEditors'){syncEditors(request.params);result=true;}
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
    try{void invocation.run({receipts:new Map()},()=>handle(JSON.parse(data.toString())));}catch(e){emit({type:'error',text:String(e)});}
  }
}).on('end',async()=>{
  for(const p of pending.values()){clearTimeout(p.timeout);p.reject(new Error('Extension host closed'));}pending.clear();
  for(const {module,subscriptions} of active.values()){try{await module.deactivate?.();}catch{}subscriptions.forEach(x=>x.dispose());}
});
