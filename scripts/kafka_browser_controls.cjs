'use strict';
// Executes the shipped script against a deterministic DOM/fetch fixture; this
// verifies recovery controls, not rendered-browser or visual acceptance.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync(new URL('../internal/web/assets.go','file://'+__filename),'utf8').match(/const script = `([\s\S]*?)`\s*$/)[1];
const kinds=['trusted','unidentified','kafka-trusted','kafka-unidentified','kafka-tampered'];
function fixture(storage=new Map(),mode='done',stream='a'.repeat(32),role='operator'){
 const elements=new Map();const buttons=[];const news=[];const calls=[];
 for(const kind of kinds){elements.set(kind+'-result',{textContent:'No attempt yet.'});buttons.push({dataset:kind.startsWith('kafka-')?{scenario:kind}:{caller:kind},disabled:false,addEventListener(){}});news.push({dataset:{new:kind},disabled:false,addEventListener(){}});}
 elements.set('balances',{textContent:''});elements.set('logout',{addEventListener(){}});elements.set('kafka-menu',{dataset:{stream,ready:'true'}});
 const document={getElementById:id=>elements.get(id),querySelector:selector=>buttons.find(b=>selector.includes(b.dataset.caller||b.dataset.scenario)),querySelectorAll:selector=>selector==='[data-new]'?news:buttons.filter(b=>selector==='[data-caller]'?b.dataset.caller:b.dataset.scenario)};
 let generation=0;
 const context=vm.createContext({document,sessionStorage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,value),removeItem:key=>storage.delete(key)},crypto:{randomUUID:()=>('b'.repeat(31)+(++generation))},location:{reload(){}},setTimeout:callback=>callback(),fetch:async(path,options={})=>{
  calls.push({path,options});
  let value;
  if(path==='/api/session')value={role,csrf_token:'fixture'};
  else if(path==='/api/balances')value={balances:{'demo-a':100000,'demo-b':100000},version:0};
  else{if(mode==='unread')throw new Error('response lost');value={attempt_id:'attempt',status:mode==='pending'?'pending':mode,caller_evidence:{outcome:'transfer_succeeded'},kafka_evidence:{outcome:'transfer_succeeded'}};}
  return{ok:true,json:async()=>value};
 }});
 vm.runInContext(source,context);
 return {context,storage,calls,elements,buttons,news};
}
async function run(){
 for(const kind of ['trusted','kafka-trusted']){
  let f=fixture();await vm.runInContext('init()',f.context);await vm.runInContext(`attempt('${kind}')`,f.context);
  const first=f.storage.get('poc-key-'+kind);assert.equal(f.storage.get('poc-completed-'+kind),first);
  await vm.runInContext(`attempt('${kind}')`,f.context);assert.equal(f.storage.get('poc-key-'+kind),first);
  vm.runInContext(`newAttempt('${kind}')`,f.context);assert.equal(f.storage.has('poc-key-'+kind),false);
  f=fixture(f.storage,'unread');await vm.runInContext('init()',f.context);await vm.runInContext(`attempt('${kind}')`,f.context);
  const unknown=f.storage.get('poc-key-'+kind);vm.runInContext(`newAttempt('${kind}');newAttempt('${kind}')`,f.context);assert.equal(f.storage.get('poc-key-'+kind),unknown);
  f=fixture(f.storage,'retryable');await vm.runInContext('init()',f.context);vm.runInContext(`newAttempt('${kind}')`,f.context);assert.equal(f.storage.get('poc-key-'+kind),unknown);
  await vm.runInContext(`attempt('${kind}')`,f.context);assert.equal(f.storage.get('poc-key-'+kind),unknown);
 }
 let f=fixture(new Map([['poc-key-kafka-trusted','old-key'],['poc-stream-kafka-trusted','c'.repeat(32)]]));await vm.runInContext('init()',f.context);await vm.runInContext("attempt('kafka-trusted')",f.context);
 assert.equal(f.calls.filter(c=>c.path==='/api/kafka-attempts').length,0);assert.equal(f.storage.get('poc-key-kafka-trusted'),'old-key');
 vm.runInContext("newAttempt('kafka-trusted')",f.context);assert.equal(JSON.parse(f.storage.get('poc-prior-kafka-trusted')).browser_key,'old-key');
 f=fixture(new Map(),'done','a'.repeat(32),'viewer');await vm.runInContext('init()',f.context);assert(f.buttons.every(b=>b.disabled));assert(f.news.every(b=>b.disabled));await vm.runInContext("attempt('kafka-trusted')",f.context);assert.equal(f.calls.filter(c=>c.path==='/api/kafka-attempts').length,0);
 console.log('Browser controls passed: stable Sync/Kafka keys, unknown/reload/new guards, stream reset and viewer controls.');
}
run().catch(()=>{console.error('Browser control fixture failed');process.exitCode=1;});
