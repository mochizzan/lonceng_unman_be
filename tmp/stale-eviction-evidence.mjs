// tmp/stale-eviction-evidence.mjs — B: prove MarkStale next hit creating new session not reused <30s 0×503
// Flow: login A 200 → profile A 200 reuse → induce via SESSION_TTL=1m expiry alami (sleep TTL+10) → profile A next must be fresh creating new session
// Std fetch only, single file. --help exit 0. Spec B — no /__test endpoint, uses natural TTL expiry.
// Usage:
//   node tmp/stale-evidence-evidence.mjs --help
//   SESSION_TTL=1m docker compose up -d && node tmp/stale-eviction-evidence.mjs --ttl 60 --json
const BASE = process.env.BASE_URL || "http://localhost:3000";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";
function fmt(ms){ return ms>=1000 ? (ms/1000).toFixed(2)+"s": ms+"ms"; }
function printHelp(){
  console.log(`stale-eviction-evidence — B: MarkStale fresh via TTL expiry

Usage:
  node tmp/stale-eviction-evidence.mjs [options]
  SESSION_TTL=1m docker compose up -d && node tmp/stale-eviction-evidence.mjs --ttl 60 --json

Options:
  --help        Show help and exit 0
  --ttl SEC     TTL seconds (default 60 dev with SESSION_TTL=1m, 900 prod)
  --base URL    Override BASE_URL (default http://localhost:3000)
  --json        JSON summary

Flow:
  1. POST /lms/login A 200
  2. POST /lms/student-profile A 200 reuse <15s
  3. Sleep TTL+10s → POST /lms/student-profile A 200 fresh <30s 0×503 (creating new session, not reused)
  Pass 200 <30s, no 503. Before MarkStale, would be reused even after IsTransient.`); }
async function req(method, path, body, timeoutMs=90000){
  const url=(globalThis.__EFFECTIVE_BASE||BASE)+path; const t0=Date.now(); const ctrl=new AbortController(); const to=setTimeout(()=>ctrl.abort(),timeoutMs);
  try{ const opts={method, headers:{}, signal:ctrl.signal}; if(method!=="GET"){ opts.headers["Content-Type"]="application/json"; opts.body=JSON.stringify(body||{}); } const res=await fetch(url,opts); let raw=await res.text(); let j=null; try{ j=JSON.parse(raw);}catch{ j={raw:raw.slice(0,1200)}; } clearTimeout(to); return {ok:res.ok, status:res.status, ms:Date.now()-t0, json:j, raw, path}; }catch(e){ clearTimeout(to); return {ok:false,status:0,ms:Date.now()-t0,error:`${e.name}: ${e.message}`,json:null,raw:"",path}; }
}
async function main(){
  const args=process.argv.slice(2);
  if(args.includes("--help")||args.includes("-h")){ printHelp(); process.exit(0); }
  let ttlSec=60;
  for(let i=0;i<args.length;i++){
    if(args[i]==="--ttl"&&args[i+1]) ttlSec=parseInt(args[++i],10);
    if(args[i].startsWith("--ttl=")) ttlSec=parseInt(args[i].split("=")[1],10);
    if(args[i]==="--base"&&args[i+1]) globalThis.__EFFECTIVE_BASE=args[++i];
    if(args[i].startsWith("--base=")) globalThis.__EFFECTIVE_BASE=args[i].split("=")[1];
  }
  globalThis.__EFFECTIVE_BASE=globalThis.__EFFECTIVE_BASE||BASE;
  const jsonOut=args.includes("--json");
  console.log("=".repeat(78));
  console.log(`STALE-EVICTION  BASE=${globalThis.__EFFECTIVE_BASE}  NPM_A=${NPM_A}  ttl=${ttlSec}s`);
  console.log("=".repeat(78));
  let pass=true;
  let r=await req("POST","/api/v1/lms/login",{npm:NPM_A,password:PASSWORD_A});
  console.log(`1 login          ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  if(r.status!==200) pass=false;
  r=await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`2 profile reuse  ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} ${r.error||""}`);
  if(r.status!==200) pass=false; if(r.ms>=15000) pass=false;
  const waitSec=ttlSec+10;
  console.log(`  sleeping ${waitSec}s TTL+10 for fresh...`);
  await new Promise(res=>setTimeout(res, waitSec*1000));
  r=await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`3 profile fresh  ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  if(r.status!==200){ console.log("✗ step3 not 200 fresh"); pass=false; }
  if(r.status===503) pass=false;
  if(r.ms>=30000){ console.log(`✗ step3 >=30s ${fmt(r.ms)}`); pass=false; } else console.log(`✓ step3 <30s ${fmt(r.ms)}`);
  const any503=[r].some(s=> s.status===503); if(any503) pass=false; else console.log("✓ no 503");
  console.log(`\nSTALE-EVICTION ${pass?"PASS":"FAIL"}`);
  if(jsonOut) console.log(JSON.stringify({ok:pass, base:globalThis.__EFFECTIVE_BASE, ttlSec, waitSec},null,2));
  process.exit(pass?0:1);
}
main().catch(e=>{ console.error(e); process.exit(1); });
