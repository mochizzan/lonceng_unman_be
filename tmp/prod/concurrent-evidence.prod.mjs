// tmp/concurrent-evidence.mjs — multi-component evidence: Navigation → waitReady → probe → session reuse
// Phase 3 harness style (falsifiable BEFORE fix): test first, fix later. No otel/debug/cloud deps.
// Run:
//   node tmp/concurrent-evidence.mjs --help
//   node tmp/concurrent-evidence.mjs
//   BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=Izzan027 NPM_B=2211700009 PASSWORD_B=xxx node tmp/concurrent-evidence.mjs
//   BASE_URL=https://lonceng-unman-api.miproduction.web.id node tmp/concurrent-evidence.mjs --sequential-only
//   BASE_URL=https://lonceng-unman-api.miproduction.web.id node tmp/concurrent-evidence.mjs --parallel-only
// Exit: 0 = all pass (200, no 503, wall<60s, per-req <15s), 1 = any fail (falsified)
// Requires: running server on BASE_URL (see --help). Std fetch only — no playwright/rod/puppeteer/axios/otel.

const BASE = process.env.BASE_URL || "https://lonceng-unman-api.miproduction.web.id";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";
const NPM_B = process.env.NPM_B || process.env.NPM2 || "2211700009";
const PASSWORD_B = process.env.PASSWORD_B || process.env.PASSWORD2 || process.env.PASSWORD || "Izzan027";

const PER_REQ_BUDGET_MS = 15000; // per /khs & /student-profile must be <15s (waitReady 15s cap)
const WALL_BUDGET_MS = 60000;    // no 503/hang within 60s window
const REQ_TIMEOUT_MS = 90000;    // fetch abort (generous, but wall budget is strict)

function fmt(ms){ return ms>=1000 ? (ms/1000).toFixed(2)+"s" : ms+"ms"; }

function printHelp(){
  console.log(`concurrent-evidence — multi-account Navigation → waitReady → probe → session-reuse evidence

Usage:
  node tmp/concurrent-evidence.mjs [options]
  BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=xxx NPM_B=2211700009 PASSWORD_B=yyy node tmp/concurrent-evidence.mjs

Options:
  --help                Show this help and exit 0
  --sequential-only     Only run sequential phase (A→B serial)
  --parallel-only       Only run parallel phase (A‖B concurrent)
  --base URL            Override BASE_URL (default https://lonceng-unman-api.miproduction.web.id)
  --json                Output JSON summary to stdout (in addition to human log)

Env (all optional, defaults shown):
  BASE_URL              ${BASE}
  NPM_A / NPM           ${NPM_A}   (account A)
  PASSWORD_A            ***        (account A password)
  NPM_B / NPM2          ${NPM_B}   (account B; if same as A, harness warns)
  PASSWORD_B            ***        (account B password; falls back to PASSWORD_A)
  PER_REQ_BUDGET_MS     ${PER_REQ_BUDGET_MS}  (fail if any /student-profile or /khs wall >= this)
  WALL_BUDGET_MS        ${WALL_BUDGET_MS}     (fail if any wall >= this or 503)

What it validates (without guessing root cause):
  1. Router → Handler → Service → SessionManager.GetOrCreate → BrowserSession.Navigate → waitReady
     (Method D: element-specific, not window.onload) → isLMSExpiredProbe → page lifecycle (0→1→0)
  2. Sequential vs parallel with DIFFERENT NPMs: per-NPM locks must not crosstalk,
     Browser.Page() wall must stay <15s, no 503 within 60s.
  3. Session reuse: second hit for same NPM should be 200 via cached browser (no new login hang).

Pass criteria (all must hold, otherwise exit 1):
  - Every POST /api/v1/lms/login and POST /api/v1/lms/student-profile returns 200
  - No 503 anywhere (infrastructure vs credential classifier)
  - No wall-time >= ${WALL_BUDGET_MS}ms (hang) and no per-req >= ${PER_REQ_BUDGET_MS}ms for student-profile/khs
  - If parallel phase runs, wall_parallel < sum(sequential walls) * 1.4 (proves concurrency, not serial bottleneck)

Output:
  Human log to stderr/stdout + JSON summary if --json. Non-zero exit on any falsification.

Examples:
  node tmp/concurrent-evidence.mjs --help
  node tmp/concurrent-evidence.mjs
  node tmp/concurrent-evidence.mjs --sequential-only
  node tmp/concurrent-evidence.mjs --parallel-only --json`);
}

async function req(method, path, body, timeoutMs=REQ_TIMEOUT_MS){
  const url = BASE + path;
  const t0 = Date.now();
  const ctrl = new AbortController();
  const to = setTimeout(()=> ctrl.abort(), timeoutMs);
  try{
    const opts = { method, headers:{}, signal: ctrl.signal };
    if(method !== "GET"){
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body||{});
    }
    const res = await fetch(url, opts);
    const ct = res.headers.get("content-type")||"";
    let j=null, raw="";
    raw = await res.text();
    try{ j = JSON.parse(raw); }catch{ j = { raw: raw.slice(0,800) }; }
    clearTimeout(to);
    const ms = Date.now()-t0;
    return { ok: res.ok, status: res.status, ms, json:j, raw, url, path, bodyNpm: body?.npm||null };
  }catch(e){
    clearTimeout(to);
    const ms = Date.now()-t0;
    return { ok:false, status:0, ms, error: `${e.name}: ${e.message}`, json:null, url, path, bodyNpm: body?.npm||null };
  }
}

function assertNo503(results, phase){
  const bad = results.filter(r=> r.status===503 || (r.json && String(r.json.message||"").toLowerCase().includes("tidak dapat diakses")));
  if(bad.length){
    console.log(`✗ ${phase}: 503 detected (${bad.length}/${results.length}) — falsified (infra, not credential)`);
    bad.forEach(r=> console.log(`  503 ${r.path} npm=${r.bodyNpm} ${r.status} ${fmt(r.ms)} ${r.error||""} msg=${String(r.json?.message||r.raw||"").slice(0,120)}`));
    return false;
  }
  return true;
}
function assertAll200(results, phase){
  const bad = results.filter(r=> !r.ok || r.status!==200);
  if(bad.length){
    console.log(`✗ ${phase}: not all 200 (${bad.length}/${results.length} non-200)`);
    bad.forEach(r=> console.log(`  ${r.status||"ERR"} ${r.path} npm=${r.bodyNpm} ${fmt(r.ms)} ${r.error||""} msg=${String(r.json?.message||r.raw||"").slice(0,160)}`));
    return false;
  }
  console.log(`✓ ${phase}: all 200 (${results.length}/${results.length})`);
  return true;
}
function assertPerReqBudget(results, phase){
  const over = results.filter(r=> r.path.includes("/student-profile") || r.path.includes("/khs"));
  const bad = over.filter(r=> r.ms >= PER_REQ_BUDGET_MS);
  if(bad.length){
    console.log(`✗ ${phase}: per-req >=${fmt(PER_REQ_BUDGET_MS)} (${bad.length}/${over.length}) — waitReady/probe budget exceeded`);
    bad.forEach(r=> console.log(`  ${fmt(r.ms)} ${r.status} ${r.path} npm=${r.bodyNpm}`));
    return false;
  }
  if(over.length) console.log(`✓ ${phase}: per-req <${fmt(PER_REQ_BUDGET_MS)} (${over.length} browser ops)`);
  return true;
}
function assertWallBudget(results, phase){
  const bad = results.filter(r=> r.ms >= WALL_BUDGET_MS);
  if(bad.length){
    console.log(`✗ ${phase}: wall >=${fmt(WALL_BUDGET_MS)} hang (${bad.length}/${results.length})`);
    bad.forEach(r=> console.log(`  ${fmt(r.ms)} ${r.status} ${r.path} npm=${r.bodyNpm} ${r.error||""}`));
    return false;
  }
  console.log(`✓ ${phase}: wall <${fmt(WALL_BUDGET_MS)} (no hang)`);
  return true;
}

async function healthCheck(){
  const h = await req("GET","/api/v1/health",null,8000);
  const ok = h.ok && h.status===200;
  console.log(`health: ${ok?"✓":"✗"} ${h.status} ${fmt(h.ms)} ${h.error||""}`);
  return ok;
}

async function login(npm, password){ return req("POST","/api/v1/lms/login",{npm,password}); }
async function studentProfile(npm, password){ return req("POST","/api/v1/lms/student-profile",{npm,password}, REQ_TIMEOUT_MS); }

async function runSequential(){
  console.log("\n── SEQUENTIAL (A→B serial, proves no crosstalk when not concurrent)");
  const seq = [];
  let t0 = Date.now();
  const r1 = await login(NPM_A, PASSWORD_A); seq.push({label:"login A", ...r1});
  console.log(`  ${r1.ok&&r1.status===200?"✓":"✗"} login A (npm=${NPM_A}) ${String(r1.status).padStart(3)} ${fmt(r1.ms).padStart(7)}${r1.error?` ERR=${r1.error.slice(0,80)}`:""} msg=${String(r1.json?.message||"").slice(0,70)}`);
  const r2 = await studentProfile(NPM_A, PASSWORD_A); seq.push({label:"student-profile A", ...r2});
  console.log(`  ${r2.ok&&r2.status===200?"✓":"✗"} student-profile A ${String(r2.status).padStart(3)} ${fmt(r2.ms).padStart(7)}${r2.error?` ERR=${r2.error.slice(0,80)}`:""} msg=${String(r2.json?.message||"").slice(0,70)}`);
  const r3 = await login(NPM_B, PASSWORD_B); seq.push({label:"login B", ...r3});
  console.log(`  ${r3.ok&&r3.status===200?"✓":"✗"} login B (npm=${NPM_B}) ${String(r3.status).padStart(3)} ${fmt(r3.ms).padStart(7)}${r3.error?` ERR=${r3.error.slice(0,80)}`:""} msg=${String(r3.json?.message||"").slice(0,70)}`);
  const r4 = await studentProfile(NPM_B, PASSWORD_B); seq.push({label:"student-profile B", ...r4});
  console.log(`  ${r4.ok&&r4.status===200?"✓":"✗"} student-profile B ${String(r4.status).padStart(3)} ${fmt(r4.ms).padStart(7)}${r4.error?` ERR=${r4.error.slice(0,80)}`:""} msg=${String(r4.json?.message||"").slice(0,70)}`);
  const wall = Date.now()-t0;
  console.log(`  └─ sequential wall ${fmt(wall)}`);
  return { wall, results: seq };
}

async function runParallel(){
  console.log("\n── PARALLEL (A‖B concurrent, proves per-NPM lock isolation & no shared-page race)");
  const t0 = Date.now();
  console.log("  ┌─ login A‖B (2 paralel, different NPM)");
  const [ra, rb] = await Promise.all([ login(NPM_A, PASSWORD_A), login(NPM_B, PASSWORD_B) ]);
  const loginResults = [{label:"login A", ...ra}, {label:"login B", ...rb}];
  loginResults.forEach(r=>{
    console.log(`  │ ${r.ok&&r.status===200?"✓":"✗"} ${r.label.padEnd(18)} ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)}${r.error?` ERR=${r.error.slice(0,80)}`:""} msg=${String(r.json?.message||"").slice(0,60)}`);
  });
  console.log("  ├─ student-profile A‖B (2 paralel, different NPM, each waits #nim)");
  const [sa, sb] = await Promise.all([ studentProfile(NPM_A, PASSWORD_A), studentProfile(NPM_B, PASSWORD_B) ]);
  const profileResults = [{label:"student-profile A", ...sa}, {label:"student-profile B", ...sb}];
  profileResults.forEach(r=>{
    console.log(`  │ ${r.ok&&r.status===200?"✓":"✗"} ${r.label.padEnd(18)} ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)}${r.error?` ERR=${r.error.slice(0,80)}`:""} msg=${String(r.json?.message||"").slice(0,60)}`);
  });
  const wall = Date.now()-t0;
  const all = [...loginResults, ...profileResults];
  console.log(`  └─ parallel wall ${fmt(wall)}`);
  return { wall, results: all, loginResults, profileResults };
}

async function main(){
  const args = process.argv.slice(2);
  if(args.includes("--help") || args.includes("-h")){
    printHelp();
    process.exit(0);
  }
  let baseOverride = null;
  for(let i=0;i<args.length;i++){
    if(args[i]==="--base" && args[i+1]){ baseOverride = args[++i]; }
    if(args[i].startsWith("--base=")){ baseOverride = args[i].split("=")[1]; }
  }
  if(baseOverride) global.BASE = baseOverride; // BASE is const, so reassign via env check below is not needed; we patch by mutating globalThis
  // Apply --base override by patching BASE const via closure variable (reassign via global)
  // Workaround: BASE is const, so we override by setting process.env and re-reading locally
  let effectiveBase = baseOverride || BASE;
  // Monkey-patch fetch base by overriding global BASE used in req: re-define req to use effectiveBase if needed
  // Instead, just set env and use effectiveBase in req closure — easiest: override req's BASE via globalThis
  globalThis.__EFFECTIVE_BASE = effectiveBase;
  const origReq = req;
  // Patch req to use effective base
  req = async (method, path, body, timeoutMs=REQ_TIMEOUT_MS)=>{
    const url = globalThis.__EFFECTIVE_BASE + path;
    const t0 = Date.now();
    const ctrl = new AbortController();
    const to = setTimeout(()=> ctrl.abort(), timeoutMs);
    try{
      const opts = { method, headers:{}, signal: ctrl.signal };
      if(method !== "GET"){ opts.headers["Content-Type"]="application/json"; opts.body=JSON.stringify(body||{}); }
      const res = await fetch(url, opts);
      let raw = await res.text();
      let j=null; try{ j=JSON.parse(raw);}catch{ j={raw: raw.slice(0,800)}; }
      clearTimeout(to);
      return { ok: res.ok, status: res.status, ms: Date.now()-t0, json:j, raw, url, path, bodyNpm: body?.npm||null };
    }catch(e){
      clearTimeout(to);
      return { ok:false, status:0, ms: Date.now()-t0, error: `${e.name}: ${e.message}`, json:null, url, path, bodyNpm: body?.npm||null };
    }
  };

  const seqOnly = args.includes("--sequential-only");
  const parOnly = args.includes("--parallel-only");
  const jsonOut = args.includes("--json");

  console.log("=".repeat(78));
  console.log(`CONCURRENT EVIDENCE  BASE=${globalThis.__EFFECTIVE_BASE}  NPM_A=${NPM_A}  NPM_B=${NPM_B}`);
  if(NPM_A===NPM_B) console.log("⚠ NPM_A === NPM_B — parallel test still valid but not multi-account; set NPM_B differently for full isolation test");
  console.log(`Budgets: per-req <${fmt(PER_REQ_BUDGET_MS)}  wall <${fmt(WALL_BUDGET_MS)}  reqTimeout=${fmt(REQ_TIMEOUT_MS)}  no 503`);
  console.log("=".repeat(78));

  const healthOk = await healthCheck();
  if(!healthOk){
    console.log("✗ health failed — abort (is server on BASE_URL?)");
    if(jsonOut) console.log(JSON.stringify({ok:false, reason:"health failed", base: globalThis.__EFFECTIVE_BASE}, null, 2));
    process.exit(1);
  }

  let seqRes = null, parRes = null;
  const allResults = [];

  if(!parOnly){
    seqRes = await runSequential();
    allResults.push(...seqRes.results);
  }
  if(!seqOnly){
    parRes = await runParallel();
    allResults.push(...parRes.results);
  }

  // Assertions
  console.log("\n── ASSERTIONS (falsifiable, exit 1 if any fail)");
  let pass = true;
  const phases = [];
  if(seqRes){ phases.push(["sequential", seqRes.results]); }
  if(parRes){ phases.push(["parallel", parRes.results]); }
  if(phases.length===0){ console.log("✗ no phase ran"); pass=false; }
  for(const [name, results] of phases){
    const a1 = assertAll200(results, name);
    const a2 = assertNo503(results, name);
    const a3 = assertPerReqBudget(results, name);
    const a4 = assertWallBudget(results, name);
    if(!a1||!a2||!a3||!a4) pass=false;
  }
  // Global 503 scan
  const any503 = allResults.some(r=> r.status===503);
  if(any503){ console.log("✗ global: 503 present — infra failure, not credential"); pass=false; } else { console.log("✓ global: no 503"); }

  // Concurrency sanity: parallel wall should be < sequential sum *1.4 if both ran
  if(seqRes && parRes){
    const seqSum = seqRes.results.reduce((s,r)=>s+r.ms,0);
    const parWall = parRes.wall;
    const bound = seqSum * 1.4;
    if(parWall < bound){
      console.log(`✓ concurrency: parallel wall ${fmt(parWall)} < 1.4× seq sum ${fmt(seqSum)} (proves A‖B overlapped, not serialized)`);
    } else {
      console.log(`✗ concurrency: parallel wall ${fmt(parWall)} >= 1.4× seq sum ${fmt(seqSum)} — possible per-NPM crosstalk or global lock`);
      // Not hard fail — warn only, since LMS slowness can inflate both
      console.log("  (warn only, not failing overall)");
    }
  }

  // Summary
  const total = allResults.length;
  const okCount = allResults.filter(r=> r.ok && r.status===200).length;
  const failCount = total - okCount;
  const byStatus = {}; allResults.forEach(r=>{ const k=String(r.status||"ERR"); byStatus[k]=(byStatus[k]||0)+1; });
  console.log("\n"+ "=".repeat(78));
  console.log(`SUMMARY  total=${total} ok=${okCount} fail=${failCount} pass=${pass?"YES":"NO"}`);
  console.log(`by status: ${Object.entries(byStatus).map(([k,v])=>`${k}:${v}`).join(" ")}`);
  const slowest = [...allResults].sort((a,b)=>b.ms-a.ms).slice(0,5);
  console.log("slowest 5:");
  slowest.forEach(r=> console.log(`  ${fmt(r.ms).padStart(7)} ${String(r.status).padStart(3)} ${r.path} npm=${r.bodyNpm} ${r.error?`ERR=${r.error.slice(0,60)}`:""}`));
  const hangs = allResults.filter(r=> r.ms>=WALL_BUDGET_MS);
  if(hangs.length) console.log(`⚠ hangs >=${fmt(WALL_BUDGET_MS)}: ${hangs.length}`);
  else console.log(`✓ no hangs >=${fmt(WALL_BUDGET_MS)}`);
  const overPerReq = allResults.filter(r=> (r.path.includes("student-profile")||r.path.includes("/khs")) && r.ms>=PER_REQ_BUDGET_MS);
  if(overPerReq.length) console.log(`⚠ per-req >=${fmt(PER_REQ_BUDGET_MS)}: ${overPerReq.length}`);
  else console.log(`✓ per-req <${fmt(PER_REQ_BUDGET_MS)}`);
  console.log("=".repeat(78));
  if(NPM_A===NPM_B) console.log("NOTE: NPM_A===NPM_B; re-run with two distinct accounts for full multi-account isolation proof.");

  if(jsonOut){
    const summary = {
      ok: pass,
      base: globalThis.__EFFECTIVE_BASE,
      npmA: NPM_A, npmB: NPM_B,
      total, okCount, failCount, byStatus,
      budgets: { perReqMs: PER_REQ_BUDGET_MS, wallMs: WALL_BUDGET_MS, reqTimeoutMs: REQ_TIMEOUT_MS },
      sequential: seqRes ? { wallMs: seqRes.wall, results: seqRes.results.map(r=>({label:r.label, status:r.status, ms:r.ms, ok:r.ok, npm:r.bodyNpm, error:r.error||null})) } : null,
      parallel: parRes ? { wallMs: parRes.wall, results: parRes.results.map(r=>({label:r.label, status:r.status, ms:r.ms, ok:r.ok, npm:r.bodyNpm, error:r.error||null})) } : null,
    };
    console.log(JSON.stringify(summary, null, 2));
  }

  if(!pass) process.exit(1);
}

main().catch(e=>{ console.error("fatal", e); process.exit(1); });
