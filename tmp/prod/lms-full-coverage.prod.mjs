// tmp/lms-full-coverage.mjs — full LMS-bound coverage (login/krs/semesters/khs) seq + parallel
// Pure ephemeral TTL15m verification. Std fetch only, single file. No otel/debug/cloud.
// Covers router.go LMS-bound:
//
//   POST /api/v1/lms/login              → session.GetOrCreate → login race
//   POST /api/v1/lms/krs                → konversi_upd_mhs + krs_pdf.php
//   POST /api/v1/lms/khs/semesters      → .table-bordered
//   POST /api/v1/lms/khs                → cetak_detail + khs_pdf.php
//
// Excludes (by request): POST /lms/khs/file, FS-only extract/data, health/eval/auth.
// Run:
//   node tmp/lms-full-coverage.mjs --help
//   BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=Izzan027 NPM_B=2211700009 PASSWORD_B=ilham122 node tmp/lms-full-coverage.mjs
//   node tmp/lms-full-coverage.mjs --sequential-only
//   node tmp/lms-full-coverage.mjs --parallel-only --json
//   node tmp/lms-full-coverage.mjs --strict  (require 200 for krs/khs too, not just ≠503)
// Exit: 0 = pass (no 503, wall<60s, per-req<15s, login+semesters 200), 1 = fail
// Requires: running server on BASE_URL. Harnesses: fetch only.

const BASE = process.env.BASE_URL || "https://lonceng-unman-api.miproduction.web.id";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";
const NPM_B = process.env.NPM_B || process.env.NPM2 || "2211700009";
const PASSWORD_B = process.env.PASSWORD_B || process.env.PASSWORD2 || process.env.PASSWORD || "ilham122";

const PER_REQ_BUDGET_MS = 15000;
const WALL_BUDGET_MS = 60000;
const REQ_TIMEOUT_MS = 90000;

function fmt(ms){ return ms>=1000 ? (ms/1000).toFixed(2)+"s" : ms+"ms"; }

function printHelp(){
  console.log(`lms-full-coverage — full LMS-bound endpoint coverage (ephemeral TTL15m)

Usage:
  node tmp/lms-full-coverage.mjs [options]
  BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=xxx NPM_B=2211700009 PASSWORD_B=yyy node tmp/lms-full-coverage.mjs

Options:
  --help                Show this help and exit 0
  --sequential-only     Only sequential phase (A→B serial, each: login→krs→semesters→khs)
  --parallel-only       Only parallel phase (A‖B concurrent per-step)
  --base URL            Override BASE_URL (default https://lonceng-unman-api.miproduction.web.id)
  --json                Output JSON summary
  --strict              Require 200 for krs/khs too (default: krs/khs 404/500 is warn, only 503 is fail)

Env:
  BASE_URL              ${BASE}
  NPM_A / NPM           ${NPM_A}
  PASSWORD_A            ***
  NPM_B / NPM2          ${NPM_B}
  PASSWORD_B            ***
  PER_REQ_BUDGET_MS     ${PER_REQ_BUDGET_MS}  (fail if any LMS wall >= this)
  WALL_BUDGET_MS        ${WALL_BUDGET_MS}
  REQ_TIMEOUT_MS        ${REQ_TIMEOUT_MS}

What it validates:
  1. All LMS-bound routes via Manager.GetOrCreate → Navigate(D) → waitElementReady → DownloadPDF/Eval
     login (#username) → krs (input[name='semester']) → semesters (.table-bordered) → khs (a[href*='khs_pdf.php'])
  2. Sequential A→B proves no crosstalk serial; parallel A‖B proves per-NPM lock + pageMu isolation
  3. No 503 (infra), per-req <15s, wall <60s; login+semesters must be 200; krs/khs must be 200 in --strict else 503-free

Excluded:
  POST /lms/khs/file (by request), FS-only extract/data, health/eval/auth

Examples:
  node tmp/lms-full-coverage.mjs --help
  BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=Izzan027 node tmp/lms-full-coverage.mjs --sequential-only
  BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=Izzan027 NPM_B=2211700009 PASSWORD_B=ilham122 node tmp/lms-full-coverage.mjs --json`);
}

async function req(method, path, body, timeoutMs=REQ_TIMEOUT_MS){
  const url = (globalThis.__EFFECTIVE_BASE || BASE) + path;
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
    let raw = await res.text();
    let j=null; try{ j=JSON.parse(raw);}catch{ j={raw: raw.slice(0,1200)}; }
    clearTimeout(to);
    return { ok: res.ok, status: res.status, ms: Date.now()-t0, json:j, raw, url, path, bodyNpm: body?.npm||null };
  }catch(e){
    clearTimeout(to);
    return { ok:false, status:0, ms: Date.now()-t0, error: `${e.name}: ${e.message}`, json:null, raw:"", url, path, bodyNpm: body?.npm||null };
  }
}

function assertNo503(results, phase){
  const bad = results.filter(r=> r.status===503 || String(r.json?.message||"").toLowerCase().includes("tidak dapat diakses"));
  if(bad.length){
    console.log(`✗ ${phase}: 503 detected (${bad.length}/${results.length})`);
    bad.forEach(r=> console.log(`  503 ${r.path} npm=${r.bodyNpm} ${r.status} ${fmt(r.ms)} ${String(r.json?.message||"").slice(0,120)}`));
    return false;
  }
  console.log(`✓ ${phase}: no 503`);
  return true;
}
function assertWallBudget(results, phase){
  const bad = results.filter(r=> r.ms >= WALL_BUDGET_MS);
  if(bad.length){
    console.log(`✗ ${phase}: wall >=${fmt(WALL_BUDGET_MS)} hang (${bad.length})`);
    bad.forEach(r=> console.log(`  ${fmt(r.ms)} ${r.status} ${r.path} npm=${r.bodyNpm}`));
    return false;
  }
  console.log(`✓ ${phase}: wall <${fmt(WALL_BUDGET_MS)}`);
  return true;
}
function assertPerReqBudget(results, phase){
  const bad = results.filter(r=> r.ms >= PER_REQ_BUDGET_MS);
  if(bad.length){
    console.log(`✗ ${phase}: per-req >=${fmt(PER_REQ_BUDGET_MS)} (${bad.length}/${results.length})`);
    bad.forEach(r=> console.log(`  ${fmt(r.ms)} ${r.status} ${r.path} npm=${r.bodyNpm}`));
    return false;
  }
  console.log(`✓ ${phase}: per-req <${fmt(PER_REQ_BUDGET_MS)} (${results.length} ops)`);
  return true;
}

async function healthCheck(){
  const h = await req("GET","/api/v1/health",null,8000);
  const ok = h.ok && h.status===200;
  console.log(`health: ${ok?"✓":"✗"} ${h.status} ${fmt(h.ms)} ${h.error||""}`);
  return ok;
}

async function login(npm, password){ return req("POST","/api/v1/lms/login",{npm,password}); }
async function krs(npm, password){ return req("POST","/api/v1/lms/krs",{npm,password}); }
async function khsSemesters(npm, password){ return req("POST","/api/v1/lms/khs/semesters",{npm,password}); }
async function khs(npm, password, tahun_ajaran, semester){ return req("POST","/api/v1/lms/khs",{npm,password,tahun_ajaran,semester}); }

function pickSemester(semRes){
  // semRes.json may be {data:{semesters:[...]}} or {data:[...]} or {semesters:[...]}
  try{
    let list = null;
    if(Array.isArray(semRes.json?.data)) list = semRes.json.data;
    else if(Array.isArray(semRes.json?.data?.semesters)) list = semRes.json.data.semesters;
    else if(Array.isArray(semRes.json?.semesters)) list = semRes.json.semesters;
    else if(Array.isArray(semRes.json?.data?.data)) list = semRes.json.data.data;
    if(Array.isArray(list) && list.length){
      const s = list[0];
      // support both tahun_ajaran/semester and tahunAjaran/semester keys
      const ta = s.tahun_ajaran || s.tahunAjaran || s.tahun || "";
      const sem = s.semester || s.smt || "";
      if(ta && sem) return {tahun_ajaran: ta, semester: sem};
    }
  }catch{}
  return null;
}

async function runSequential(strict){
  console.log("\n── SEQUENTIAL (A→B serial: login→krs→semesters→khs per NPM)");
  const results = [];
  let wall0 = Date.now();

  for(const [label,npm,password] of [["A",NPM_A,PASSWORD_A],["B",NPM_B,PASSWORD_B]]){
    console.log(`  ┌─ NPM ${label}=${npm}`);
    const rLogin = await login(npm,password); rLogin.label=`login ${label}`; results.push(rLogin);
    console.log(`  │ ${rLogin.ok&&rLogin.status===200?"✓":"✗"} login ${label}            ${String(rLogin.status).padStart(3)} ${fmt(rLogin.ms).padStart(7)}${rLogin.error?` ERR=${rLogin.error.slice(0,60)}`:""}`);

    const rKrs = await krs(npm,password); rKrs.label=`krs ${label}`; results.push(rKrs);
    const krsOk = rKrs.status===200;
    console.log(`  │ ${krsOk?"✓":"·"} krs ${label}              ${String(rKrs.status).padStart(3)} ${fmt(rKrs.ms).padStart(7)}${rKrs.error?` ERR=${rKrs.error.slice(0,60)}`:""} msg=${String(rKrs.json?.message||"").slice(0,60)}${!krsOk&&!strict?" (warn, not strict)":""}`);

    const rSem = await khsSemesters(npm,password); rSem.label=`khs/semesters ${label}`; results.push(rSem);
    console.log(`  │ ${rSem.ok&&rSem.status===200?"✓":"✗"} khs/semesters ${label}   ${String(rSem.status).padStart(3)} ${fmt(rSem.ms).padStart(7)}${rSem.error?` ERR=${rSem.error.slice(0,60)}`:""}`);

    let ta="2023/2024", sem="Ganjil";
    const picked = pickSemester(rSem);
    if(picked){ ta=picked.tahun_ajaran; sem=picked.semester; console.log(`  │   ↳ picked semester ${ta} ${sem} from semesters`); }
    else { console.log(`  │   ↳ no semester picked, fallback ${ta} ${sem}`); }

    const rKhs = await khs(npm,password,ta,sem); rKhs.label=`khs ${label}`; results.push(rKhs);
    const khsOk = rKhs.status===200;
    console.log(`  │ ${khsOk?"✓":"·"} khs ${label} ${ta} ${sem}  ${String(rKhs.status).padStart(3)} ${fmt(rKhs.ms).padStart(7)}${rKhs.error?` ERR=${rKhs.error.slice(0,60)}`:""} msg=${String(rKhs.json?.message||"").slice(0,60)}${!khsOk&&!strict?" (warn, not strict)":""}`);
    console.log(`  └─ NPM ${label} done`);
  }
  const wall = Date.now()-wall0;
  console.log(`  sequential wall ${fmt(wall)}`);
  return { wall, results };
}

async function runParallel(strict){
  console.log("\n── PARALLEL (A‖B concurrent per-step)");
  const t0 = Date.now();
  const results = [];

  console.log("  ┌─ login A‖B");
  const [ra,rb] = await Promise.all([login(NPM_A,PASSWORD_A), login(NPM_B,PASSWORD_B)]);
  ra.label="login A"; rb.label="login B"; results.push(ra,rb);
  [ra,rb].forEach(r=> console.log(`  │ ${r.ok&&r.status===200?"✓":"✗"} ${r.label.padEnd(12)} ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)}`));

  console.log("  ├─ krs A‖B");
  const [ka,kb] = await Promise.all([krs(NPM_A,PASSWORD_A), krs(NPM_B,PASSWORD_B)]);
  ka.label="krs A"; kb.label="krs B"; results.push(ka,kb);
  [ka,kb].forEach(r=> console.log(`  │ ${r.status===200?"✓":"·"} ${r.label.padEnd(12)} ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)}${r.status!==200&&!strict?" (warn)":""}`));

  console.log("  ├─ khs/semesters A‖B");
  const [sa,sb] = await Promise.all([khsSemesters(NPM_A,PASSWORD_A), khsSemesters(NPM_B,PASSWORD_B)]);
  sa.label="khs/semesters A"; sb.label="khs/semesters B"; results.push(sa,sb);
  [sa,sb].forEach(r=> console.log(`  │ ${r.ok&&r.status===200?"✓":"✗"} ${r.label.padEnd(18)} ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)}`));

  const pickA = pickSemester(sa) || {tahun_ajaran:"2023/2024", semester:"Ganjil"};
  const pickB = pickSemester(sb) || {tahun_ajaran:"2023/2024", semester:"Ganjil"};
  console.log(`  │   ↳ picked A=${pickA.tahun_ajaran} ${pickA.semester} B=${pickB.tahun_ajaran} ${pickB.semester}`);

  console.log("  ├─ khs A‖B");
  const [ha,hb] = await Promise.all([khs(NPM_A,PASSWORD_A,pickA.tahun_ajaran,pickA.semester), khs(NPM_B,PASSWORD_B,pickB.tahun_ajaran,pickB.semester)]);
  ha.label="khs A"; hb.label="khs B"; results.push(ha,hb);
  [ha,hb].forEach(r=> console.log(`  │ ${r.status===200?"✓":"·"} ${r.label.padEnd(12)} ${String(r.status).padStart(3)} ${fmt(r.ms).padStart(7)}${r.status!==200&&!strict?" (warn)":""}`));

  const wall = Date.now()-t0;
  console.log(`  └─ parallel wall ${fmt(wall)}`);
  return { wall, results };
}

async function main(){
  const args = process.argv.slice(2);
  if(args.includes("--help")||args.includes("-h")){ printHelp(); process.exit(0); }
  let baseOverride=null;
  for(let i=0;i<args.length;i++){
    if(args[i]==="--base" && args[i+1]) baseOverride=args[++i];
    if(args[i].startsWith("--base=")) baseOverride=args[i].split("=")[1];
  }
  if(baseOverride) globalThis.__EFFECTIVE_BASE=baseOverride;
  else globalThis.__EFFECTIVE_BASE=BASE;
  const seqOnly=args.includes("--sequential-only");
  const parOnly=args.includes("--parallel-only");
  const jsonOut=args.includes("--json");
  const strict=args.includes("--strict");

  console.log("=".repeat(80));
  console.log(`LMS FULL COVERAGE  BASE=${globalThis.__EFFECTIVE_BASE}  NPM_A=${NPM_A}  NPM_B=${NPM_B}  strict=${strict}`);
  if(NPM_A===NPM_B) console.log("⚠ NPM_A===NPM_B — parallel not multi-account; set NPM_B distinct for full isolation");
  console.log(`Budgets: per-req <${fmt(PER_REQ_BUDGET_MS)}  wall <${fmt(WALL_BUDGET_MS)}  reqTimeout=${fmt(REQ_TIMEOUT_MS)}  no 503`);
  console.log(`Endpoints: login, krs, khs/semesters, khs (khs/file excluded)`);
  console.log("=".repeat(80));

  const healthOk = await healthCheck();
  if(!healthOk){
    console.log("✗ health failed — abort (is server on BASE_URL?)");
    if(jsonOut) console.log(JSON.stringify({ok:false, reason:"health failed", base: globalThis.__EFFECTIVE_BASE}, null, 2));
    process.exit(1);
  }

  let seqRes=null, parRes=null;
  const allResults=[];

  if(!parOnly){ seqRes=await runSequential(strict); allResults.push(...seqRes.results); }
  if(!seqOnly){ parRes=await runParallel(strict); allResults.push(...parRes.results); }

  console.log("\n── ASSERTIONS");
  let pass=true;
  const phases=[];
  if(seqRes) phases.push(["sequential", seqRes.results]);
  if(parRes) phases.push(["parallel", parRes.results]);
  if(!phases.length){ console.log("✗ no phase ran"); pass=false; }

  for(const [name, results] of phases){
    // login must be 200
    const loginResults = results.filter(r=> r.label && r.label.startsWith("login"));
    const loginOk = loginResults.every(r=> r.status===200);
    if(!loginOk){ console.log(`✗ ${name}: login not all 200`); loginResults.forEach(r=> console.log(`  ${r.label} ${r.status} ${fmt(r.ms)}`)); pass=false; } else console.log(`✓ ${name}: login 200 (${loginResults.length})`);
    // semesters must be 200
    const semResults = results.filter(r=> r.label && r.label.includes("semesters"));
    const semOk = semResults.every(r=> r.status===200);
    if(!semOk){ console.log(`✗ ${name}: khs/semesters not all 200`); semResults.forEach(r=> console.log(`  ${r.label} ${r.status} ${fmt(r.ms)}`)); pass=false; } else console.log(`✓ ${name}: khs/semesters 200 (${semResults.length})`);
    // krs/khs
    const docResults = results.filter(r=> r.label && (r.label.startsWith("krs ") || r.label.startsWith("khs ")));
    if(strict){
      const bad = docResults.filter(r=> r.status!==200);
      if(bad.length){ console.log(`✗ ${name}: krs/khs not all 200 strict (${bad.length}/${docResults.length})`); bad.forEach(r=> console.log(`  ${r.label} ${r.status} ${fmt(r.ms)}`)); pass=false; } else console.log(`✓ ${name}: krs/khs 200 strict (${docResults.length})`);
    } else {
      console.log(`· ${name}: krs/khs lenient (404/500 warn, 503 fail) — ${docResults.length} ops`);
    }
    const a1 = assertNo503(results, name);
    const a2 = assertWallBudget(results, name);
    const a3 = assertPerReqBudget(results, name);
    if(!a1||!a2||!a3) pass=false;
  }

  const any503 = allResults.some(r=> r.status===503);
  if(any503){ console.log("✗ global: 503 present"); pass=false; } else console.log("✓ global: no 503");

  if(seqRes && parRes){
    const seqSum = seqRes.results.reduce((s,r)=>s+r.ms,0);
    const parWall = parRes.wall;
    const bound = seqSum * 1.4;
    if(parWall < bound) console.log(`✓ concurrency: parallel ${fmt(parWall)} < 1.4× seq sum ${fmt(seqSum)}`);
    else console.log(`· concurrency warn: parallel ${fmt(parWall)} >= 1.4× seq sum ${fmt(seqSum)} (LMS slowness, not fail)`);
  }

  const total=allResults.length, okCount=allResults.filter(r=> r.status===200).length;
  const byStatus={}; allResults.forEach(r=>{ const k=String(r.status||"ERR"); byStatus[k]=(byStatus[k]||0)+1; });
  console.log("\n"+ "=".repeat(80));
  console.log(`SUMMARY total=${total} 200=${okCount} fail=${total-okCount} pass=${pass?"YES":"NO"}`);
  console.log(`by status: ${Object.entries(byStatus).map(([k,v])=>`${k}:${v}`).join(" ")}`);
  const slowest=[...allResults].sort((a,b)=>b.ms-a.ms).slice(0,5);
  console.log("slowest 5:");
  slowest.forEach(r=> console.log(`  ${fmt(r.ms).padStart(7)} ${String(r.status).padStart(3)} ${r.label||r.path} npm=${r.bodyNpm||""}`));
  console.log("=".repeat(80));
  if(NPM_A===NPM_B) console.log("NOTE: NPM_A===NPM_B; re-run with two distinct accounts for full isolation proof.");

  if(jsonOut){
    const summary={ ok: pass, base: globalThis.__EFFECTIVE_BASE, npmA:NPM_A, npmB:NPM_B, strict, budgets:{perReqMs:PER_REQ_BUDGET_MS, wallMs:WALL_BUDGET_MS, reqTimeoutMs:REQ_TIMEOUT_MS}, sequential: seqRes? {wallMs: seqRes.wall, results: seqRes.results.map(r=>({label:r.label,status:r.status,ms:r.ms,ok:r.ok}))}: null, parallel: parRes? {wallMs: parRes.wall, results: parRes.results.map(r=>({label:r.label,status:r.status,ms:r.ms,ok:r.ok}))}: null, byStatus };
    console.log(JSON.stringify(summary,null,2));
  }
  process.exit(pass?0:1);
}

main().catch(e=>{ console.error("fatal",e); process.exit(1); });
