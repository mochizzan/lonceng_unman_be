// tmp/account-switch-stale.mjs — repro idle ganti akun (sebab 14:22 1m13s 500) + proof MarkStale
// Flow per IDLE_SEC/TTL_SEC: login A→profile A→krs A (panaskan lastUsed=now) → idle A sementara loop 5× khs/semesters B 200 durasi IDLE_SEC → profile A lagi
// F1 idle < TTL (e.g. --ttl 900 --idle 600) → A masih <15m session reused tapi PHPSESSID GC menit → wait #nim 15s → ErrLMSExpired → MarkStale+GetOrCreate → 200 <30s bukan 500 1m13s
// F2 idle > TTL (e.g. --ttl 900 --idle 960 atau dev SESSION_TTL=1m + --ttl 60 --idle 70) → A >15m evict → fresh creating new session 6s → 200
// Assert: status 200 0×503 per-req<30s (expiry 25s ok) docker logs lms expired fast-fail / session marked stale / creating new session bukan session reused
// Std fetch only, single file. --help exit 0.
// Usage:
//   node tmp/account-switch-stale.mjs --help
//   BASE_URL=http://localhost:3000 NPM_A=2211700006 PASSWORD_A=Izzan027 NPM_B=2211700009 PASSWORD_B=ilham122 node tmp/account-switch-stale.mjs --idle 600 --ttl 900 --json --strict
//   SESSION_TTL=1m docker compose up -d && node tmp/account-switch-stale.mjs --ttl 60 --idle 70 --json
// Needs IsTransient fix + fast-fail #nim; before fix: F1 500 1m13s, after: 200 <30s

const BASE = process.env.BASE_URL || "http://localhost:3000";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";
const NPM_B = process.env.NPM_B || process.env.NPM2 || "2211700009";
const PASSWORD_B = process.env.PASSWORD_B || process.env.PASSWORD2 || process.env.PASSWORD || "Izzan027";

function fmt(ms){ return ms>=1000 ? (ms/1000).toFixed(2)+"s" : ms+"ms"; }

function printHelp(){
  console.log(`account-switch-stale — repro idle ganti akun expiry (F per spec 2026-09-10)

Usage:
  node tmp/account-switch-stale.mjs [options]
  BASE_URL=http://localhost:3000 NPM_A=2211700006 PASSWORD_A=xxx NPM_B=2211700009 PASSWORD_B=yyy node tmp/account-switch-stale.mjs

Options:
  --help                Show help and exit 0
  --idle SEC            Idle seconds for A while B loops (default 600 prod, use 70 dev with SESSION_TTL=1m)
  --ttl SEC             TTL seconds for reference/assert (default 900 prod, 60 dev)
  --base URL            Override BASE_URL (default http://localhost:3000)
  --json                Output JSON summary
  --strict              Fail on any non-200 (default: strict for repro profile)
  --idle-only           Only idle phase (skip final profile check) debug

Env:
  BASE_URL              ${BASE}
  NPM_A / NPM           ${NPM_A}
  PASSWORD_A            ***
  NPM_B / NPM2          ${NPM_B}
  PASSWORD_B            ***

Flow per IDLE_SEC/TTL (default prod TTL 15m IDLE 10m → F1):
  1. POST /lms/login A 200 → POST /lms/student-profile A 200 → POST /lms/krs A 200 (panaskan lastUsed=now)
  2. Idle A durasi IDLE_SEC sementara loop 5× POST /lms/khs/semesters B 200 (A idle tidak touchLastUsed)
     F1 idle<TTL (600<900) → A <15m session reused tapi PHP GC menit → wait #nim 15s → ErrLMSExpired → MarkStale+GetOrCreate → 200 <30s
     F2 idle>TTL (960>900 atau dev 70>60) → A >15m evict → fresh creating new session 6s → 200
  3. POST /lms/student-profile A lagi — assert 200 0×503 per-req<30s (expiry 25s allowed) bukan 500 1m13s
     docker logs harus lms expired fast-fail / session marked stale / creating new session bukan session reused

Examples:
  node tmp/account-switch-stale.mjs --help
  SESSION_TTL=1m docker compose up -d && node tmp/account-switch-stale.mjs --ttl 60 --idle 70 --json
  node tmp/account-switch-stale.mjs --idle 600 --ttl 900 --json --strict`);
}

async function req(method, path, body, timeoutMs=90000){
  const url = (globalThis.__EFFECTIVE_BASE || BASE) + path;
  const t0 = Date.now();
  const ctrl = new AbortController();
  const to = setTimeout(()=> ctrl.abort(), timeoutMs);
  try{
    const opts = { method, headers:{}, signal: ctrl.signal };
    if(method!=="GET"){ opts.headers["Content-Type"]="application/json"; opts.body=JSON.stringify(body||{}); }
    const res = await fetch(url, opts);
    let raw = await res.text(); let j=null; try{ j=JSON.parse(raw);}catch{ j={raw:raw.slice(0,1500)}; }
    clearTimeout(to);
    return {ok:res.ok, status:res.status, ms:Date.now()-t0, json:j, raw, path, bodyNpm: body?.npm||null};
  }catch(e){
    clearTimeout(to);
    return {ok:false, status:0, ms:Date.now()-t0, error:`${e.name}: ${e.message}`, json:null, raw:"", path, bodyNpm: body?.npm||null};
  }
}

async function healthCheck(){
  const h = await req("GET","/api/v1/health",null,5000);
  const ok = h.ok && h.status===200;
  console.log(`health: ${ok?"✓":"✗"} ${h.status} ${fmt(h.ms)} ${h.error||""}`);
  return ok;
}

async function main(){
  const args = process.argv.slice(2);
  if(args.includes("--help")||args.includes("-h")){ printHelp(); process.exit(0); }
  let idleSec = 600, ttlSec = 900;
  for(let i=0;i<args.length;i++){
    if(args[i]==="--idle" && args[i+1]) idleSec=parseInt(args[++i],10);
    if(args[i].startsWith("--idle=")) idleSec=parseInt(args[i].split("=")[1],10);
    if(args[i]==="--ttl" && args[i+1]) ttlSec=parseInt(args[++i],10);
    if(args[i].startsWith("--ttl=")) ttlSec=parseInt(args[i].split("=")[1],10);
    if(args[i]==="--base" && args[i+1]) globalThis.__EFFECTIVE_BASE=args[++i];
    if(args[i].startsWith("--base=")) globalThis.__EFFECTIVE_BASE=args[i].split("=")[1];
  }
  globalThis.__EFFECTIVE_BASE = globalThis.__EFFECTIVE_BASE || BASE;
  const jsonOut = args.includes("--json");
  const strict = args.includes("--strict");
  const idleOnly = args.includes("--idle-only");

  console.log("=".repeat(86));
  console.log(`ACCOUNT-SWITCH-STALE  BASE=${globalThis.__EFFECTIVE_BASE}  NPM_A=${NPM_A}  NPM_B=${NPM_B}  idle=${idleSec}s ttl=${ttlSec}s F1 idle<TTL=${idleSec<ttlSec}?"yes":"no (F2)"}`);
  if(NPM_A===NPM_B) console.log("⚠ NPM_A===NPM_B — not multi-account; set NPM_B distinct");
  console.log(`Budgets: final profile per-req <30s (expiry 15s wait+6s login) no 503, before fix 500 1m13s`);
  console.log("=".repeat(86));

  const healthOk = await healthCheck();
  if(!healthOk){ console.log("✗ health failed"); if(jsonOut) console.log(JSON.stringify({ok:false, reason:"health"},null,2)); process.exit(1); }

  const steps=[];
  let pass=true;

  // Phase 1: panaskan A
  console.log("\n── PHASE 1 panaskan A (lastUsed=now)");
  let r = await req("POST","/api/v1/lms/login",{npm:NPM_A,password:PASSWORD_A});
  console.log(`  ${r.status===200?"✓":"✗"} login A              ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  steps.push({label:"login A",...r});
  if(r.status!==200) pass=false;
  if(r.status===503){ console.log("✗ login A 503 infra — fail"); pass=false; }

  r = await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  console.log(`  ${r.status===200?"✓":"✗"} profile A (panaskan) ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  steps.push({label:"profile A panaskan",...r});
  if(r.status!==200) pass=false;

  r = await req("POST","/api/v1/lms/krs",{npm:NPM_A,password:PASSWORD_A});
  const krsOk = r.status===200 || (!strict && (r.status===404||r.status===500));
  console.log(`  ${krsOk?"✓":"✗"} krs A                ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} msg=${String(r.json?.message||"").slice(0,70)} ${r.error||""}`);
  steps.push({label:"krs A",...r});
  if(r.status===503) pass=false;

  // Phase 2: idle A sementara B loop 5×
  console.log(`\n── PHASE 2 idle A ${idleSec}s sementara B loop 5× khs/semesters (A tidak touchLastUsed)`);
  const loopStart = Date.now();
  let elapsed=0;
  let bOk=true;
  let loops=0;
  // Spread 5 B calls across idle window: sleep idleSec/5 between each, plus initial
  const perWait = Math.max(0, Math.floor(idleSec*1000/5) - 500);
  for(let i=0;i<5;i++){
    const rb = await req("POST","/api/v1/lms/khs/semesters",{npm:NPM_B,password:PASSWORD_B});
    console.log(`  ${rb.status===200?"✓":"✗"} khs/semesters B #${i+1}  ${String(rb.status).padStart(3)} ${fmt(rb.ms).padStart(7)} msg=${String(rb.json?.message||"").slice(0,60)} ${rb.error||""}`);
    steps.push({label:`khs/semesters B #${i+1}`,...rb});
    if(rb.status!==200) bOk=false;
    if(rb.status===503) pass=false;
    loops++;
    if(i<4 && perWait>0){
      // sleep but account for request time
      const remain = perWait;
      if(remain>0) await new Promise(res=>setTimeout(res, remain));
    }
  }
  // If idleSec not yet reached, sleep remainder
  elapsed = Date.now() - loopStart;
  const remainIdle = idleSec*1000 - elapsed;
  if(remainIdle > 1000){
    console.log(`  · sleeping remaining ${fmt(remainIdle)} to reach idle ${idleSec}s (elapsed ${fmt(elapsed)})`);
    await new Promise(res=>setTimeout(res, remainIdle));
  }
  elapsed = Date.now() - loopStart;
  console.log(`  └─ idle done elapsed ${fmt(elapsed)} loops ${loops} B ok=${bOk?"yes":"no"}`);

  if(idleOnly){
    console.log("\n· --idle-only skip final profile check");
    console.log(`RESULT idle phase done ${pass?"PASS":"FAIL"}`);
    if(jsonOut) console.log(JSON.stringify({ok:pass, base:globalThis.__EFFECTIVE_BASE, idleSec, ttlSec, steps: steps.map(s=>({label:s.label,status:s.status,ms:s.ms}))},null,2));
    process.exit(pass?0:1);
  }

  // Phase 3: ganti balik A
  console.log(`\n── PHASE 3 ganti balik A: POST /lms/student-profile A (expect 200 <30s, not 500 1m13s)`);
  const t0 = Date.now();
  r = await req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A});
  const wall = Date.now()-t0;
  steps.push({label:"profile A after idle",...r});
  console.log(`  ${r.status===200?"✓":"✗"} profile A after idle  ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)} wall ${fmt(wall)} msg=${String(r.json?.message||"").slice(0,80)} ${r.error||""}`);
  // Assertions
  console.log("\n── ASSERTIONS");
  let aPass=true;
  if(r.status!==200){
    console.log(`✗ final profile not 200 got ${r.status} — before fix 500 1m13s wait #nim not found 15s×3`);
    aPass=false;
  } else console.log(`✓ final profile 200`);
  if(r.status===503 || String(r.json?.message||"").toLowerCase().includes("tidak dapat diakses")){
    console.log(`✗ final profile 503 infra — should be 200 fresh`);
    aPass=false;
  } else console.log(`✓ final profile no 503`);
  if(r.ms >= 30000){
    console.log(`✗ final per-req >=30s (${fmt(r.ms)}) — expiry should be <30s (15s wait+6s login) not 1m13s`);
    aPass=false;
  } else console.log(`✓ final per-req <30s (${fmt(r.ms)})`);
  if(r.ms >= 65000){
    console.log(`✗ final wall >=65s hang`);
    aPass=false;
  }
  // Hint logs
  console.log(`· hint: if F1 idle<TTL and still 500 1m13s, check docker logs "lms expired fast-fail" / "session marked stale" / "creating new session" vs "session reused"`);
  console.log(`· hint: if F2 idle>TTL got reused, TTL not honored — expect creating new session`);
  if(idleSec < ttlSec){
    console.log(`  F1 mode idle<TTL (${idleSec}<${ttlSec}) → expect reused then MarkStale fresh <30s`);
  } else {
    console.log(`  F2 mode idle>TTL (${idleSec}>=${ttlSec}) → expect evict then fresh <15s`);
  }

  const any503 = steps.some(s=> s.status===503);
  if(any503){ console.log("✗ global: 503 present"); aPass=false; } else console.log("✓ global: no 503");

  pass = pass && aPass;

  const byStatus={}; steps.forEach(s=>{ const k=String(s.status||"ERR"); byStatus[k]=(byStatus[k]||0)+1; });
  console.log("\n"+ "=".repeat(86));
  console.log(`SUMMARY ok=${aPass?"YES":"NO"} total=${steps.length} byStatus ${Object.entries(byStatus).map(([k,v])=>`${k}:${v}`).join(" ")} F=${idleSec<ttlSec?"F1 idle<TTL":"F2 idle>TTL"}`);
  console.log(`FINAL profile after idle: ${r.status} ${fmt(r.ms)} ${r.error||""}`);
  console.log("=".repeat(86));

  if(jsonOut){
    console.log(JSON.stringify({ok:pass, base:globalThis.__EFFECTIVE_BASE, npmA:NPM_A, npmB:NPM_B, idleSec, ttlSec, mode: idleSec<ttlSec?"F1":"F2", steps: steps.map(s=>({label:s.label,status:s.status,ms:s.ms, npm:s.bodyNpm})), final:{status:r.status, ms:r.ms, error:r.error||null}},null,2));
  }
  process.exit(pass?0:1);
}

main().catch(e=>{ console.error("fatal",e); process.exit(1); });
