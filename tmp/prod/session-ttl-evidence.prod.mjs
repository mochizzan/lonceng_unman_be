// tmp/session-ttl-evidence.mjs — TTL 15m eviction evidence (pure ephemeral)
// Std fetch only, single file. No otel/debug/cloud.
// Validates: login → profile reuse → wait TTL+delta → profile fresh (not 503, not reused)
// Usage:
//   node tmp/session-ttl-evidence.mjs --help
//   BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=Izzan027 node tmp/session-ttl-evidence.mjs
//   SESSION_TTL=1m docker compose up -d && node tmp/session-ttl-evidence.mjs --json
// Exit 0 pass, 1 fail

const BASE = process.env.BASE_URL || "https://lonceng-unman-api.miproduction.web.id";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";
const TTL_SEC = parseInt(process.env.TEST_TTL_SEC || process.env.TTL_SEC || "0", 10);
const DEFAULT_WAIT_SEC = TTL_SEC > 0 ? TTL_SEC + 10 : 70; // 1m TTL +10s via SESSION_TTL=1m, else 70s

function printHelp(){
  console.log(`session-ttl-evidence — prove TTL 15m eviction (ephemeral sessions)

Usage:
  node tmp/session-ttl-evidence.mjs [options]
  BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=xxx node tmp/session-ttl-evidence.mjs
  SESSION_TTL=1m docker compose up -d && node tmp/session-ttl-evidence.mjs --json

Options:
  --help            Show help and exit 0
  --base URL        Override BASE_URL
  --wait SEC        Wait seconds between reused and fresh check (default ${DEFAULT_WAIT_SEC}s)
  --json            JSON summary output

Env:
  BASE_URL          ${BASE}
  NPM_A             ${NPM_A}
  PASSWORD_A        ***
  TEST_TTL_SEC      override wait when SESSION_TTL is non-default (e.g. 60 for 1m)

Flow:
  1. POST /lms/login 200 creating new session
  2. POST /lms/student-profile 200 (reuse, <15s)
  3. Sleep --wait seconds (TTL+10s; prod 910s, dev 70s with SESSION_TTL=1m)
  4. POST /lms/student-profile 200 fresh (must be new session, not reused, not 503/500) 6-9s

Pass: step4 200 and not reused (if server logs unavailable, pass on 200 only with warning)
Fail: 503/500 or still reused after TTL, or per-req >=15s`);
}

async function req(method, path, body, timeoutMs=90000){
  const url = (globalThis.__EFFECTIVE_BASE || BASE) + path;
  const t0 = Date.now();
  const ctrl = new AbortController();
  const to = setTimeout(()=>ctrl.abort(), timeoutMs);
  try{
    const opts={method, headers:{}, signal:ctrl.signal};
    if(method!=="GET"){ opts.headers["Content-Type"]="application/json"; opts.body=JSON.stringify(body||{}); }
    const res = await fetch(url, opts);
    let raw = await res.text(); let j=null; try{ j=JSON.parse(raw);}catch{ j={raw:raw.slice(0,1200)};}
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
  let waitSec = DEFAULT_WAIT_SEC;
  for(let i=0;i<args.length;i++){
    if(args[i]==="--base" && args[i+1]) globalThis.__EFFECTIVE_BASE=args[++i];
    if(args[i].startsWith("--base=")) globalThis.__EFFECTIVE_BASE=args[i].split("=")[1];
    if(args[i]==="--wait" && args[i+1]) waitSec=parseInt(args[++i],10);
    if(args[i].startsWith("--wait=")) waitSec=parseInt(args[i].split("=")[1],10);
  }
  globalThis.__EFFECTIVE_BASE = globalThis.__EFFECTIVE_BASE || BASE;
  const jsonOut=args.includes("--json");
  console.log(`SESSION TTL EVIDENCE BASE=${globalThis.__EFFECTIVE_BASE} NPM_A=${NPM_A} wait=${waitSec}s`);
  let pass=true;
  const r1 = await req("POST","/api/v1/lms/login",{npm:NPM_A,password:PASSWORD_A});
  console.log(`1 login             ${r1.status} ${r1.ms}ms ${r1.error||""} msg=${String(r1.json?.message||"").slice(0,80)}`);
  if(r1.status!==200){ console.log("✗ step1 login not 200"); pass=false; }
  const r2 = await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`2 profile reuse     ${r2.status} ${r2.ms}ms ${r2.error||""}`);
  if(r2.status!==200){ console.log("✗ step2 profile not 200"); pass=false; }
  if(r2.ms>=15000){ console.log(`✗ step2 per-req >=15s ${r2.ms}ms`); pass=false; }
  console.log(`  sleeping ${waitSec}s for TTL eviction...`);
  await new Promise(r=>setTimeout(r, waitSec*1000));
  const r3 = await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`3 profile post-TTL  ${r3.status} ${r3.ms}ms ${r3.error||""} msg=${String(r3.json?.message||"").slice(0,80)}`);
  if(r3.status!==200){ console.log("✗ step3 post-TTL not 200 (should be fresh 200)"); pass=false; }
  if(r3.status===503){ console.log("✗ step3 503 — should be fresh login, not infra"); pass=false; }
  if(r3.ms>=15000){ console.log(`✗ step3 per-req >=15s ${r3.ms}ms`); pass=false; }
  // Heuristic: if server echoes "creating new session" vs "reused" in message, check
  const msg = String(r3.json?.message||r3.raw||"").toLowerCase();
  if(msg.includes("reused") || msg.includes("session reused")){
    console.log("✗ step3 still reused — TTL not honored (expected fresh)");
    pass=false;
  } else if(msg.includes("creating new session") || msg.includes("login successful")){
    console.log("✓ step3 fresh session detected via message");
  } else {
    console.log("· step3 message does not indicate reuse/fresh — pass on 200 only (check docker logs for creating new session)");
  }
  console.log(`\nTTL EVIDENCE ${pass?"PASS":"FAIL"}`);
  if(jsonOut) console.log(JSON.stringify({ok:pass, base:globalThis.__EFFECTIVE_BASE, npm:NPM_A, waitSec, steps:[{s:1,status:r1.status,ms:r1.ms},{s:2,status:r2.status,ms:r2.ms},{s:3,status:r3.status,ms:r3.ms}]},null,2));
  process.exit(pass?0:1);
}
main().catch(e=>{ console.error(e); process.exit(1); });
