'use strict';

// Terminal objects retain identity while the native client owns their processes.
module.exports = ({native, strict, EventEmitter, Uri, enabled, report}) => {
  const objects = new Map();
  const events = {open:new EventEmitter(),close:new EventEmitter(),active:new EventEmitter(),state:new EventEmitter()};
  let generation = -1, active, sequence = 0, queued = 0, queuedBytes = 0;
  const prefix = require('node:crypto').randomUUID();
  const string = (value, limit, name) => {
    if (typeof value !== 'string' || value.includes('\0') || Buffer.byteLength(value) > limit) throw Error(`Invalid terminal ${name}`);
    return value;
  };
  function options(value = {}) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw Error('Invalid terminal options');
    const copy = {};
    const allowed = new Set(['name','shellPath','shellArgs','cwd','env','strictEnv','hideFromUser','message','isTransient','location']);
    for (const key of Object.keys(value)) {
      if (!allowed.has(key)) throw Error(`Unsupported TerminalOptions.${key}`);
      if (value[key] === undefined) continue;
      if (['name','shellPath','message'].includes(key)) copy[key] = string(value[key], key === 'message' ? 8192 : key === 'name' ? 256 : 8192, key);
      else if (key === 'cwd') copy.cwd = value.cwd instanceof Uri ? Uri.parse(value.cwd.toString()) : string(value.cwd,8192,key);
      else if (key === 'shellArgs') {
        if (typeof value[key] === 'string') {
          if (process.platform !== 'win32') throw Error('String shellArgs require Windows');
          copy[key] = string(value[key],32768,key);
        } else {
          if (!Array.isArray(value[key]) || value[key].length > 128) throw Error('Invalid terminal shellArgs');
          copy[key] = Object.freeze(value[key].map(arg => string(arg,8192,key)));
        }
      } else if (key === 'env') {
        if (!value.env || typeof value.env !== 'object' || Array.isArray(value.env) || Object.keys(value.env).length > 256) throw Error('Invalid terminal environment');
        const env = Object.create(null);
        for (const [key, item] of Object.entries(value.env)) {
          string(key,256,'environment key');
          if (!key || key.includes('=')) throw Error('Invalid terminal environment key');
          if (item !== undefined) env[key] = item === null ? null : string(item,8192,'environment value');
        }
        copy.env = Object.freeze(env);
      } else if (key === 'location') {
        if (value[key] !== 1) throw Error('Native terminals currently require panel location');
        copy[key] = 1;
      } else {
        if (typeof value[key] !== 'boolean') throw Error(`Invalid terminal ${key}`);
        copy[key] = value[key];
      }
    }
    if (copy.cwd instanceof Uri && copy.cwd.scheme !== 'file') throw Error('Native terminal cwd requires a file URI');
    if (Buffer.byteLength(JSON.stringify(copy)) > 65536) throw Error('Terminal options exceed 64 KiB');
    return Object.freeze(copy);
  }
  const wireOptions = value => ({...value,...(value.cwd instanceof Uri ? {cwd:value.cwd.fsPath} : {})});
  function terminal(id, creationOptions, local = false) {
    let pidResolve;
    const item = {
      id, creationOptions, local, name:creationOptions.name || 'Terminal', opened:false, closed:false, disposed:false,
      state:Object.freeze({isInteractedWith:false,shell:undefined}), exitStatus:undefined,
      processId:new Promise(resolve => {pidResolve = resolve;}), resolvePid:pidResolve,
      ready:Promise.resolve(), tail:Promise.resolve()
    };
    function action(kind, params = {}, bytes = 0) {
      if ((item.disposed || item.closed) && kind !== 'dispose') throw Error('Terminal has been disposed');
      if (queued >= 128 || queuedBytes + bytes > 1024*1024) throw Error('Terminal operation queue limit reached');
      queued++; queuedBytes += bytes;
      const pending = item.tail.then(() => item.ready).then(() => native('window/terminalAction',{id:item.id,kind,...params})).then(sync);
      item.tail = pending.catch(error => report(error)).finally(() => {queued--;queuedBytes-=bytes;});
    }
    item.public = strict('Terminal',{
      then:undefined,
      get name(){return item.name;},get processId(){return item.processId;},get creationOptions(){return item.creationOptions;},
      get exitStatus(){return item.exitStatus;},get state(){return item.state;},get shellIntegration(){return undefined;},
      sendText(text, shouldExecute = true){
        if (typeof text !== 'string' || typeof shouldExecute !== 'boolean' || Buffer.byteLength(text) > 65534) throw Error('Invalid terminal input');
        action('sendText',{text,shouldExecute},Buffer.byteLength(text)+2);
      },
      show(preserveFocus = false){if(typeof preserveFocus !== 'boolean')throw Error('Invalid terminal preserveFocus');action('show',{preserveFocus});},
      hide(){action('hide');},
      dispose(){if(item.disposed)return;if(!item.closed)action('dispose');item.disposed=true;}
    });
    objects.set(id,item);
    return item;
  }
  function sync(snapshot) {
    if (!snapshot || !Number.isSafeInteger(snapshot.generation) || snapshot.generation < 0 || !Array.isArray(snapshot.terminals) || snapshot.terminals.length > 8 || !Array.isArray(snapshot.closed || []) || (snapshot.closed || []).length > 32) throw Error('Invalid native terminal snapshot');
    if (snapshot.generation <= generation) return;
    const records = new Map(), closed = new Map();
    const validate = record => {
      string(record.id,96,'identity');string(record.name,256,'name');
      if (!record.id || (record.processId != null && (!Number.isSafeInteger(record.processId) || record.processId < 1)) || typeof record.isInteractedWith !== 'boolean') throw Error('Invalid native terminal state');
      if (record.exitStatus != null && (!Number.isInteger(record.exitStatus.reason) || record.exitStatus.reason < 0 || record.exitStatus.reason > 4 || (record.exitStatus.code != null && !Number.isInteger(record.exitStatus.code)))) throw Error('Invalid terminal exit status');
      return {...record,creationOptions:options(record.creationOptions || {})};
    };
    for (const record of snapshot.terminals) {
      const value=validate(record);
      if(records.has(value.id))throw Error('Duplicate terminal identity');
      records.set(value.id,value);
    }
    for (const record of snapshot.closed || []) {
      const value=validate(record);
      if(closed.has(value.id)||records.has(value.id))throw Error('Conflicting terminal closure');
      closed.set(value.id,value);
    }
    if(snapshot.activeId && !records.has(snapshot.activeId))throw Error('Unknown active terminal');
    generation=snapshot.generation;
    const opened=[],ended=[],changed=[];
    for (const record of records.values()) {
      let item=objects.get(record.id);
      if(!item)item=terminal(record.id,record.creationOptions);
      item.name=record.name;
      if(!item.local)item.creationOptions=record.creationOptions;
      item.exitStatus=record.exitStatus == null ? undefined : Object.freeze({...record.exitStatus});
      if(record.processId != null)item.resolvePid(record.processId);
      if(record.exitStatus != null)item.resolvePid(undefined);
      if(!item.opened){item.opened=true;opened.push(item.public);}
      if(item.state.isInteractedWith !== record.isInteractedWith){item.state=Object.freeze({isInteractedWith:record.isInteractedWith,shell:undefined});changed.push(item.public);}
    }
    for (const [id,item] of objects) {
      if(records.has(id)||(!item.opened&&!closed.has(id)))continue;
      const record=closed.get(id);
      item.exitStatus=Object.freeze(record?.exitStatus || item.exitStatus || {reason:0});
      item.closed=true;item.resolvePid(undefined);objects.delete(id);ended.push(item.public);
    }
    const next=objects.get(snapshot.activeId)?.public, previous=active;
    active=next;
    for(const item of opened)events.open.fire(item);
    for(const item of changed)events.state.fire(item);
    if(previous!==active)events.active.fire(active);
    for(const item of ended)events.close.fire(item);
  }
  function create(value, shellPath, shellArgs) {
    if(!enabled())throw Error('Native terminal capability is unavailable');
    if(objects.size >= 8)throw Error('Native terminal limit reached');
    const original=options(typeof value==='string'||value===undefined ? {name:value,shellPath,shellArgs} : value);
    const item=terminal(`extension:${prefix}:${++sequence}`,original,true);
    item.ready=native('window/createTerminal',{id:item.id,options:wireOptions(original)}).then(snapshot=>{sync(snapshot);if(snapshot.creationError)throw Error(snapshot.creationError);if(!objects.has(item.id))throw Error('Terminal closed before creation acknowledgement');}).catch(error=>{
      const closed=item.closed;
      item.closed=true;item.disposed=true;item.exitStatus ||= Object.freeze({reason:0});item.resolvePid(undefined);objects.delete(item.id);
      if(active===item.public){active=undefined;events.active.fire(undefined);}
      if(!closed)events.close.fire(item.public);
      report(error);throw error;
    });
    // processId never rejects. Void API operations surface errors through Events.
    item.ready.catch(()=>{});
    return item.public;
  }
  return {sync,create,get terminals(){return [...objects.values()].filter(item=>!item.closed).map(item=>item.public);},get active(){return active;},events};
};
