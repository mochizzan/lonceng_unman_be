// tmp/ttl-boundary-evidence.mjs — G TTL boundary: login→profile reuse → sleep TTL-10s → profile reused 200 → sleep 20s (lewat TTL+10s) → profile fresh 200
// Std fetch only, single file. --help exit 0. Spec G.
// Usage:
//   node tmp/ttl-boundary-evidence.mjs --help
//   BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=Izzan027 node tmp/ttl-boundary-evidence.mjs --ttl 60 --json
//   SESSION_TTL=1m docker compose up -d && node tmp/ttl-boundary-evidence.mjs --ttl 60 --json
// Needs Manager TTL eviction: reused <TTL fresh >TTL, per-req<15s.

const BASE = process.env.BASE_URL || "https://lonceng-unman-api.miproduction.web.id";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";

function fmt(ms){ return ms>=1000 ? (ms/1000).toFixed(2)+"s" : ms+"ms"; }

function printHelp(){
  console.log(`ttl-boundary-evidence — G: prove TTL boundary reused <TTL vs fresh >TTL

Usage:
  node tmp/ttl-boundary-evidence.mjs [options]
  BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=xxx node tmp/ttl-boundary-evidence.mjs

Options:
  --help            Show help and exit 0
  --ttl SEC         TTL seconds for boundary (default 900 prod =15m, 60 dev with SESSION_TTL=1m)
  --base URL        Override BASE_URL (default https://lonceng-unman-api.miproduction.web.id)
  --json            JSON summary

Env:
  BASE_URL          ${BASE}
  NPM_A             ${NPM_A}
  PASSWORD_A        ***

Flow (ttl=60 dev example):
  1. POST /lms/login 200
  2. POST /lms/student-profile 200 reused (<15s)
  3. sleep ttl-10s (50s) → POST /lms/student-profile 200 reused (<15s) — proves not evicted before TTL
  4. sleep 20s (lewat ttl+10s =70s) → POST /lms/student-profile 200 fresh (<15s) — proves evicted after TTL, creating new session

Budgets: each step per-req <15s, no 503, step3 reused 200, step4 fresh 200 (not reused)`);
}

async function req(method, path, body, timeoutMs=90000){
  const url = (globalThis.__EFFECTIVE_BASE || BASE) + path;
  const t0 = Date.now();
  const ctrl = new AbortController();
  const to = setTimeout(()=> ctrl.abort(), timeoutMs);
  try{
    const opts={method, headers:{}, signal:ctrl.signal};
    if(method!=="GET"){ opts.headers["Content-Type"]="application/json"; opts.body=JSON.stringify(body||{}); }
    const res = await fetch(url, opts);
    let raw = await res.text(); let j=null; try{ j=JSON.parse(raw);}catch{ j={raw:raw.slice(0,1200)}; }
    clearTimeout(to);
    return {ok:res.ok, status:res.status, ms:Date.now()-t0, json:j, raw, path};
  }catch(e){
    clearTimeout(to);
    return {ok:false, status:0, ms:Date.now()-t0, error:`${e.name}: ${e.message}`, json:null, raw:"", path};
  }
}

async function main(){
  const args=process.argv.slice(2);
  if(args.includes("--help")||args.includes("-h")){ printHelp(); process.exit(0); }
  let ttlSec=900;
  for(let i=0;i<args.length;i++){
    if(args[i]==="--ttl" && args[i+1]) ttlSec=parseInt(args[++i],10);
    if(args[i].startsWith("--ttl=")) ttlSec=parseInt(args[i].split("=")[1],10);
    if(args[i]==="--base" && args[i+1]) globalThis.__EFFECTIVE_BASE=args[++i];
    if(args[i].startsWith("--base=")) globalThis.__EFFECTIVE_BASE=args[i].split("=")[1];
  }
  globalThis.__EFFECTIVE_BASE = globalThis.__EFFECTIVE_BASE || BASE;
  const jsonOut=args.includes("--json");

  console.log("=".repeat(82));
  console.log(`TTL-BOUNDARY  BASE=${globalThis.__EFFECTIVE_BASE}  NPM_A=${NPM_A}  ttl=${ttlSec}s (TTL-10=${ttlSec-10}s → TTL+10=${ttlSec+10}s)`);
  console.log("=".repeat(82));

  let pass=true;
  const steps=[];

  let r = await req("POST","/api/v1/lms/login",{npm:NPM_A,password:PASSWORD_A});
  console.log(`1 login               ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  steps.push({label:"login",...r});
  if(r.status!==200) pass=false;
  if(r.status===503) pass=false;

  r = await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`2 profile reused      ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} ${r.error||""}`);
  steps.push({label:"profile reused init",...r});
  if(r.status!==200) pass=false;
  if(r.ms>=15000) pass=false;

  const wait1 = Math.max(5, ttlSec-10);
  console.log(`  sleeping ${wait1}s (TTL-10) — should stay reused...`);
  await new Promise(res=>setTimeout(res, wait1*1000));

  r = await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`3 profile TTL-10      ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  steps.push({label:"profile TTL-10",...r});
  if(r.status!==200){ console.log("✗ step3 TTL-10 not 200 — should still be reused"); pass=false; }
  else console.log(`✓ step3 TTL-10 reused 200 per-req ${fmt(r.ms)}`);
  if(r.ms>=15000) pass=false;
  if(r.status===503) pass=false;

  const wait2 = 20;
  console.log(`  sleeping ${wait2}s (lewat TTL+10 = ${ttlSec+10}s) — should be fresh creating new session...`);
  await new Promise(res=>setTimeout(res, wait2*1000));

  r = await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`4 profile fresh       ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  steps.push({label:"profile fresh post TTL",...r});
  if(r.status!==200){ console.log("✗ step4 fresh post TTL not 200 — TTL eviction failed"); pass=false; }
  else console.log(`✓ step4 fresh post TTL 200 per-req ${fmt(r.ms)}`);
  if(r.ms>=15000){ console.log(`✗ step4 per-req >=15s`); pass=false; }
  if(r.status===503) pass=false;

  console.log("\n── ASSERTIONS");
  if(steps[2]?.status===200 && steps[3]?.status===200) console.log(`✓ TTL boundary: TTL-10 reused 200 → fresh post TTL 200`);
  else { console.log(`✗ TTL boundary failed`); pass=false; }
  const any503 = steps.some(s=> s.status===503);
  if(any503) { console.log("✗ 503 present"); pass=false; } else console.log("✓ no 503");

  const byStatus={}; steps.forEach(s=>{ const k=String(s.status||"ERR"); byStatus[k]=(byStatus[k]||0)+1; });
  console.log("\n"+ "=".repeat(82));
  console.log(`SUMMARY ok=${pass?"YES":"NO"} ${steps.map(s=>`${s.label}:${s.status} ${fmt(s.ms)}`).join(" | ")} byStatus ${Object.entries(byStatus).map(([k,v])=>`${k}:${v}`).join(" ")}`);
  console.log("=".repeat(82));
  if(jsonOut) console.log(JSON.stringify({ok:pass, base:globalThis.__EFFECTIVE_BASE, ttlSec, wait1, wait2, steps: steps.map(s=>({label:s.label,status:s.status,ms:s.ms}))},null,2));
  process.exit(pass?0:1);
}

main().catch(e=>{ console.error(e); process.exit(1); });
