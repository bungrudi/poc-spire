package web

import "html/template"

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Process identity · SPIRE PoC</title><link rel="stylesheet" href="/assets/style.css"><script src="/assets/app.js" defer></script></head>
<body><main><header><p class="eyebrow">SPIFFE / SPIRE · process identity</p><h1>Same user. Different identity.</h1><p>The caller process makes the request. Money authenticates its SVID, then checks its permission.</p></header>
{{if .Local}}<aside class="notice">Local-only preview · OS-owner login · HTTP over loopback/Unix sockets. The public assumed-operator HTTPS demo uses a separate profile.</aside>{{else}}<aside class="notice">Public SPIRE demo · operator identity is assumed, not authenticated. Anyone can trigger these fake transfers. SPIRE verifies the caller processes.</aside>{{end}}
{{if .Logged}}<div class="session"><span>{{if .Local}}Signed in as {{.Role}}{{else}}Assumed demo operator · no human login{{end}}</span><button id="logout" class="quiet">Clear session</button></div>
<section class="balances"><h2>Fake ledger</h2><p id="balances">Loading verified balances…</p><small>Fixed transfer: USD 1.00 (100 minor units), demo-a → demo-b. Money restart resets balances and retry history.</small></section>
<h2>Sync (signed HTTP)</h2><section class="cards"><article><p class="eyebrow">Registered executable</p><h2>Trusted caller</h2><p>Expected: <code>spiffe://poc.internal/caller/trusted</code></p><button data-caller="trusted">Attempt trusted transfer</button><button class="quiet new" data-new="trusted">New logical attempt</button><pre id="trusted-result" aria-live="polite">No attempt yet.</pre></article>
<article><p class="eyebrow">Unregistered executable · same UID</p><h2>Unidentified caller</h2><p>Expected: no eligible SVID. The probe still verifies Money before testing client-certificate rejection.</p><button data-caller="unidentified">Attempt unidentified transfer</button><button class="quiet new" data-new="unidentified">New logical attempt</button><pre id="unidentified-result" aria-live="polite">No attempt yet.</pre></article></section>
{{if .Kafka}}<section id="kafka-menu" data-stream="{{.KafkaStream}}" data-ready="{{.KafkaReady}}"><h2>Async (Kafka)</h2><p>Callers publish commands. Money verifies the original producer signature and returns evidence for the exact record bytes.</p>{{if not .KafkaReady}}<p>Kafka is not ready. Reload after the isolated demo finishes starting.</p>{{end}}<div class="cards"><article><p class="eyebrow">Signed command</p><h2>Trusted Kafka caller</h2><button data-scenario="kafka-trusted">Publish trusted transfer</button><button class="quiet new" data-new="kafka-trusted">New logical attempt</button><pre id="kafka-trusted-result" aria-live="polite">No attempt yet.</pre></article><article><p class="eyebrow">Same UID · no eligible SVID</p><h2>Unsigned Kafka probe</h2><button data-scenario="kafka-unidentified">Publish unsigned probe</button><button class="quiet new" data-new="kafka-unidentified">New logical attempt</button><pre id="kafka-unidentified-result" aria-live="polite">No attempt yet.</pre></article><article><p class="eyebrow">Changed after signing</p><h2>Tampered Kafka probe</h2><button data-scenario="kafka-tampered">Publish tampered probe</button><button class="quiet new" data-new="kafka-tampered">New logical attempt</button><pre id="kafka-tampered-result" aria-live="polite">No attempt yet.</pre></article></div><p class="note">Queued means broker publication. Applied or rejected needs Money's authenticated receipt. Missing evidence remains unknown. A Money restart resets this fake ledger; an unknown outcome keeps its original key.</p></section>{{end}}
<p class="note">Repeating or interrupting an attempt keeps its browser key. Use “New logical attempt” only after the previous outcome is resolved. Caller evidence is distinct from Money's HTTP result; a timeout is not identity rejection. UI balances use <code>spiffe://poc.internal/ui</code>, which cannot transfer.</p>
{{else}}<section class="login"><h2>Authentication required</h2>{{if .Local}}<p>Run the local login helper as the existing OS owner. Enter its one-use code within one minute. Codes and sessions stay in memory.</p><form method="post" action="/auth/local"><label for="code">One-use login code</label><input id="code" name="code" type="password" autocomplete="off" required maxlength="128"><button>Sign in</button></form>{{end}}</section>{{end}}
<footer>Fake-money PoC · no production payment guarantee · host/SPIRE/authorized-process compromise is outside the claim.</footer></main></body></html>`))

const style = `:root{color-scheme:dark;font-family:system-ui,sans-serif;background:#0e1520;color:#dce6ed}*{box-sizing:border-box}body{margin:0}main{max-width:1120px;margin:60px auto;padding:0 24px}header{max-width:850px}h1{font-size:clamp(2rem,5vw,3.8rem);letter-spacing:-.05em;line-height:1.06;margin:12px 0 20px}h2{margin:10px 0 18px;font-size:1.4rem}p{line-height:1.6;color:#b9c7d6}.eyebrow{letter-spacing:.14em;text-transform:uppercase;color:#78d7bd;font-size:.73rem;font-weight:700}.notice{border:1px solid #4b4f36;background:#22281c;color:#d3d7ac;padding:14px 18px;border-radius:12px;margin:26px 0}.session{display:flex;justify-content:space-between;align-items:center;margin-bottom:20px}.balances,.login,article{border:1px solid #2b3a4b;background:#131e2b;padding:24px;border-radius:16px}.balances{margin-bottom:22px}.cards{display:grid;grid-template-columns:1fr 1fr;gap:22px}code{overflow-wrap:anywhere;font-size:.85rem}small,.note,footer{color:#899eaf;line-height:1.6}button,.button{background:#81e3c5;color:#08271e;border:0;border-radius:8px;padding:11px 16px;font-weight:700;font-size:.9rem;cursor:pointer;display:inline-block;text-decoration:none}button:disabled{opacity:.45;cursor:wait}.quiet{background:transparent;color:#a5b9ca;border:1px solid #35495c}.new{margin:10px 0}pre{background:#0b131e;min-height:95px;padding:14px;white-space:pre-wrap;overflow-wrap:anywhere;line-height:1.5;border-radius:8px;font-size:.77rem;color:#bacbdb}.login{max-width:600px}label{display:block;margin-bottom:8px}input{display:block;width:100%;padding:14px;background:#091420;color:#fff;border:1px solid #42596e;border-radius:8px;margin-bottom:18px}footer{margin-top:48px;font-size:.8rem}@media(max-width:720px){main{margin-top:32px}.cards{grid-template-columns:1fr}}`

const script = `"use strict";
let session=null;
const pending=new Set();
function keyName(kind){return "poc-key-"+kind;}
function completedName(kind){return "poc-completed-"+kind;}
function runName(kind){return "poc-run-"+kind;}
function staleRun(kind,key){return Boolean(key&&session.run_id&&sessionStorage.getItem(runName(kind))!==session.run_id);}
function streamName(kind){return "poc-stream-"+kind;}
function isKafka(kind){return kind.startsWith("kafka-");}
function menu(){return document.getElementById("kafka-menu");}
function enabled(kind){return session&&session.role==="operator"&&(!isKafka(kind)||(menu()&&menu().dataset.ready==="true"));}
async function api(path,options={}){const r=await fetch(path,{credentials:"same-origin",...options});if(!r.ok){throw new Error(r.status===401?"Session expired. Reload the page, then retry the same logical attempt.":"UI request rejected ("+r.status+").");}return r.json();}
async function balances(){const el=document.getElementById("balances");if(!el)return;try{const b=await api("/api/balances");el.textContent="demo-a: "+(b.balances["demo-a"]/100).toFixed(2)+" USD · demo-b: "+(b.balances["demo-b"]/100).toFixed(2)+" USD · ledger version "+b.version;}catch(e){el.textContent=e.message;}}
async function attempt(kind){
 if(pending.has(kind)||!enabled(kind))return;
 pending.add(kind);
 const kafka=isKafka(kind);
 const button=document.querySelector(kafka?'[data-scenario="'+kind+'"]':'[data-caller="'+kind+'"]');
 const out=document.getElementById(kind+"-result");button.disabled=true;
 let key=sessionStorage.getItem(keyName(kind));
 if(!key){key=crypto.randomUUID();sessionStorage.setItem(keyName(kind),key);if(kafka)sessionStorage.setItem(streamName(kind),menu().dataset.stream);if(session.run_id)sessionStorage.setItem(runName(kind),session.run_id);}
 sessionStorage.removeItem(completedName(kind));
 try{
  if(staleRun(kind,key)||(kafka&&sessionStorage.getItem(streamName(kind))!==menu().dataset.stream))throw new Error("This key belongs to an earlier demo run. Start an explicit new logical attempt for the current run; the previous outcome stays unknown.");
  const body=kafka?{scenario:kind,browser_key:key,stream_id:sessionStorage.getItem(streamName(kind))}:{caller:kind,browser_key:key};
  let a=await api(kafka?"/api/kafka-attempts":"/api/attempts",{method:"POST",headers:{"Content-Type":"application/json","X-CSRF-Token":session.csrf_token},body:JSON.stringify(body)});
  for(let i=0;i<(kafka?85:35)&&a.status==="pending";i++){
   out.textContent=kafka?"Waiting for authenticated Money evidence…\n"+JSON.stringify(a.kafka_evidence||{},null,2):"Attempt accepted. Waiting for caller evidence…\n"+a.attempt_id;
   await new Promise(r=>setTimeout(r,400));a=await api("/api/attempts/"+a.attempt_id);
  }
  if(a.status==="pending")throw new Error("Result still pending. Retry with the same key.");
  if(a.status==="done")sessionStorage.setItem(completedName(kind),key);
  out.textContent=JSON.stringify(kafka?{attempt:a.attempt_id,status:a.status,transport:"Async (Kafka)",evidence:a.kafka_evidence}:{attempt:a.attempt_id,status:a.status,transport:"Sync (signed HTTP)",provenance:"caller diagnostics; verified Money result only when supplied",evidence:a.caller_evidence},null,2);
  await balances();
 }catch(e){out.textContent=e.message+"\nThe browser key is retained for retry.";}
 finally{pending.delete(kind);button.disabled=!enabled(kind);}
}
function newAttempt(kind){
 if(pending.has(kind)||!enabled(kind))return;
 const out=document.getElementById(kind+"-result");const key=sessionStorage.getItem(keyName(kind));
 const priorStream=sessionStorage.getItem(streamName(kind));const changedRun=staleRun(kind,key)||(isKafka(kind)&&key&&priorStream!==menu().dataset.stream);
 if(key&&!changedRun&&sessionStorage.getItem(completedName(kind))!==key){out.textContent="Resolve the unknown outcome with the same key before starting another attempt.";return;}
 if(changedRun)sessionStorage.setItem("poc-prior-"+kind,JSON.stringify({browser_key:key,run_id:sessionStorage.getItem(runName(kind)),stream_id:priorStream,outcome:"unknown"}));
 sessionStorage.removeItem(keyName(kind));sessionStorage.removeItem(completedName(kind));sessionStorage.removeItem(runName(kind));if(isKafka(kind))sessionStorage.removeItem(streamName(kind));
 out.textContent=changedRun?"New logical attempt selected for this demo run. The earlier run's outcome remains unknown. Click the publish button.":"New logical attempt selected. Click the caller button.";
}
async function init(){
 if(!document.getElementById("logout"))return;
 try{
  session=await api("/api/session");
  for(const b of document.querySelectorAll("[data-caller]")){b.disabled=!enabled(b.dataset.caller);b.addEventListener("click",()=>attempt(b.dataset.caller));}
  for(const b of document.querySelectorAll("[data-scenario]")){b.disabled=!enabled(b.dataset.scenario);b.addEventListener("click",()=>attempt(b.dataset.scenario));}
  for(const b of document.querySelectorAll("[data-new]")){b.disabled=!enabled(b.dataset.new);b.addEventListener("click",()=>newAttempt(b.dataset.new));}
  document.getElementById("logout").addEventListener("click",async()=>{await api("/auth/logout",{method:"POST",headers:{"X-CSRF-Token":session.csrf_token}});location.reload();});await balances();
 }catch(e){document.getElementById("balances").textContent=e.message;}
}
init();`
