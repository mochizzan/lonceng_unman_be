// tmp/e2e-full-flow.mjs — E2E full flow: login → profile×2 → avatar → krs → extract→get → semesters → per-semester khs download→extract→get
// Covers ALL endpoints requested:
//   1 login (POST /lms/login)
//   2 get profile data 2x (POST /lms/student-profile/data)
//   3 get profile avatar 1x (POST /lms/student-profile/photo)
//   4 download KRS (POST /lms/krs)
//   5 extract KRS (POST /lms/krs/extract)
//   6 get KRS (POST /lms/krs/data)
//   7 Get LIST TAHUN AJARAN (POST /lms/khs/semesters)
//   8 loop per tahun ajaran: download KHS (POST /lms/khs) → extract KHS (POST /lms/khs/extract) → get KHS (POST /lms/khs/data)
// Std fetch only, single file. No playwright/rod/puppeteer/axios/otel.
// Sequential 2 NPM (A→B) + Parallel 2 NPM (A‖B).
// Usage:
//   node tmp/e2e-full-flow.mjs --help
//   BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=Izzan027 NPM_B=2211700009 PASSWORD_B=ilham122 node tmp/e2e-full-flow.mjs
//   node tmp/e2e-full-flow.mjs --sequential-only
//   node tmp/e2e-full-flow.mjs --parallel-only --json
//   node tmp/e2e-full-flow.mjs --base https://lonceng-unman-api.miproduction.web.id
// Exit 0 = pass (no 503, wall budgets, login+semesters 200, lenient for PDFs), 1 = fail

const BASE = process.env.BASE_URL || "https://lonceng-unman-api.miproduction.web.id";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";
const NPM_B = process.env.NPM_B || process.env.NPM2 || "2211700009";
const PASSWORD_B = process.env.PASSWORD_B || process.env.PASSWORD2 || process.env.PASSWORD || "ilham122";

const PER_REQ_BUDGET_MS = 15000;
const WALL_BUDGET_MS = 90000; // full flow per NPM may take >60s due to many steps (login 9s + profile 4s + krs 5s + extract 2s + semesters 1s + N*khs 5s each) → 90s wall per flow
const REQ_TIMEOUT_MS = 90000;

function fmt(ms){ return ms>=1000 ? (ms/1000).toFixed(2)+"s" : ms+"ms"; }

function printHelp(){
  console.log(`e2e-full-flow — sequential + parallel full E2E (ephemeral TTL15m)

Usage:
  node tmp/e2e-full-flow.mjs [options]
  BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM_A=2211700006 PASSWORD_A=xxx NPM_B=2211700009 PASSWORD_B=yyy node tmp/e2e-full-flow.mjs

Options:
  --help                Show this help and exit 0
  --sequential-only     Only sequential A→B (each full flow serial)
  --parallel-only       Only parallel A‖B (both NPM flows concurrent)
  --base URL            Override BASE_URL (default https://lonceng-unman-api.miproduction.web.id)
  --json                Output JSON summary
  --strict              Require 200 for every step (default: download/extract 404/500 is warn, only 503 is fail)

Env:
  BASE_URL              ${BASE}
  NPM_A / NPM           ${NPM_A}
  PASSWORD_A            ***
  NPM_B / NPM2          ${NPM_B}
  PASSWORD_B            ***

Flow per NPM (sequential within NPM):
  1. login                               POST /lms/login                          {npm,password}                     → 200 must
  2. get profile data (1)                POST /lms/student-profile/data           {npm}                               → 200 (or 404 warn if never scraped)
  3. get profile data (2)                POST /lms/student-profile/data           {npm}                               → 200
  4. get profile avatar                  POST /lms/student-profile/photo          {npm,password}                     → 200 or 204 (no photo)
  5. download KRS                        POST /lms/krs                            {npm,password}                     → 200 (lenient)
  6. extract KRS                         POST /lms/krs/extract                    {npm,password}                     → 200 (lenient)
  7. get KRS                             POST /lms/krs/data                       {npm}                               → 200 (lenient)
  8. Get LIST TAHUN AJARAN               POST /lms/khs/semesters                  {npm,password}                     → 200 must
  9. Loop per semester from #8:
       download KHS                      POST /lms/khs                            {npm,password,tahun_ajaran,semester} → 200 (lenient)
       extract KHS                       POST /lms/khs/extract                    {npm,password,tahun_ajaran,semester} → 200 (lenient)
       get KHS                           POST /lms/khs/data                       {npm,tahun_ajaran,semester}        → 200 (lenient)

Budgets: per-req <15s, per-flow wall <90s, no 503 anywhere. Strict mode makes every step 200-required.

Examples:
  node tmp/e2e-full-flow.mjs --help
  node tmp/e2e-full-flow.mjs --sequential-only
  node tmp/e2e-full-flow.mjs --parallel-only --json
  BASE_URL=https://lonceng-unman-api.miproduction.web.id node tmp/e2e-full-flow.mjs`);
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
    let j=null; try{ j=JSON.parse(raw);}catch{ j={raw: raw.slice(0,1500)}; }
    const ct = res.headers.get("content-type")||"";
    const isBinary = ct.includes("application/pdf") || ct.includes("image/");
    // For photo binary, raw is not JSON; keep j as null but mark success if status 200/204
    clearTimeout(to);
    return { ok: res.ok, status: res.status, ms: Date.now()-t0, json:j, raw, ct, url, path, bodyNpm: body?.npm||null };
  }catch(e){
    clearTimeout(to);
    return { ok:false, status:0, ms: Date.now()-t0, error: `${e.name}: ${e.message}`, json:null, raw:"", ct:"", url, path, bodyNpm: body?.npm||null };
  }
}

// Endpoint wrappers
async function login(npm,password){ return req("POST","/api/v1/lms/login",{npm,password}); }
async function getProfileData(npm){ return req("POST","/api/v1/lms/student-profile/data",{npm}); }
async function getProfileAvatar(npm,password){ return req("POST","/api/v1/lms/student-profile/photo",{npm,password}); }
async function downloadKRS(npm,password){ return req("POST","/api/v1/lms/krs",{npm,password}); }
async function extractKRS(npm,password){ return req("POST","/api/v1/lms/krs/extract",{npm,password}); }
async function getKRS(npm){ return req("POST","/api/v1/lms/krs/data",{npm}); }
async function khsSemesters(npm,password){ return req("POST","/api/v1/lms/khs/semesters",{npm,password}); }
async function downloadKHS(npm,password,tahun_ajaran,semester){ return req("POST","/api/v1/lms/khs",{npm,password,tahun_ajaran,semester}); }
async function extractKHS(npm,password,tahun_ajaran,semester){ return req("POST","/api/v1/lms/khs/extract",{npm,password,tahun_ajaran,semester}); }
async function getKHS(npm,tahun_ajaran,semester){ return req("POST","/api/v1/lms/khs/data",{npm,tahun_ajaran,semester}); }

function pickSemesters(semRes){
  try{
    let list=null;
    if(Array.isArray(semRes.json?.data)) list=semRes.json.data;
    else if(Array.isArray(semRes.json?.data?.semesters)) list=semRes.json.data.semesters;
    else if(Array.isArray(semRes.json?.semesters)) list=semRes.json.semesters;
    else if(Array.isArray(semRes.json?.data?.data)) list=semRes.json.data.data;
    if(Array.isArray(list) && list.length){
      return list.map(s=>{
        const ta = s.tahun_ajaran || s.tahunAjaran || s.tahun || "";
        const sem = (s.semester || s.smt || "").toString().toUpperCase();
        return {tahun_ajaran: ta, semester: sem, raw:s};
      }).filter(x=> x.tahun_ajaran && (x.semester==="GANJIL"||x.semester==="GENAP"));
    }
  }catch{}
  return [];
}

async function runFlow(npm,password, label, strict){
  const steps=[];
  const t0=Date.now();
  const log = (ok, name, res)=>{
    const st = res.status;
    const mark = ok ? "✓" : (strict ? "✗" : "·");
    console.log(`  │ ${mark} ${name.padEnd(28)} ${String(st).padStart(3)} ${fmt(res.ms).padStart(7)}${res.error?` ERR=${res.error.slice(0,60)}`:""}${res.ct.includes("image/")||res.ct.includes("pdf")?` ct=${res.ct}`:""} msg=${String(res.json?.message||res.json?.raw||"").slice(0,60)}`);
  };

  console.log(`  ┌─ Flow ${label} npm=${npm}`);

  // 1 login must 200
  let r = await login(npm,password); r.label=`login ${label}`; steps.push(r);
  log(r.status===200, `login ${label}`, r);

  // 2 get profile data 1
  r = await getProfileData(npm); r.label=`profile/data 1 ${label}`; steps.push(r);
  // 404 is warn (not strict) if never scraped before; but after login+scrape earlier, usually 200
  log(r.status===200 || r.status===404 && !strict, `profile/data 1 ${label}`, r);

  // 3 get profile data 2
  r = await getProfileData(npm); r.label=`profile/data 2 ${label}`; steps.push(r);
  log(r.status===200 || r.status===404 && !strict, `profile/data 2 ${label}`, r);

  // 4 avatar (200 or 204)
  r = await getProfileAvatar(npm,password); r.label=`profile/photo ${label}`; steps.push(r);
  const avatarOk = r.status===200 || r.status===204;
  log(avatarOk || !strict, `profile/photo ${label}`, r);

  // 5 download KRS
  r = await downloadKRS(npm,password); r.label=`krs/download ${label}`; steps.push(r);
  log(r.status===200 || !strict, `krs/download ${label}`, r);

  // 6 extract KRS
  r = await extractKRS(npm,password); r.label=`krs/extract ${label}`; steps.push(r);
  log(r.status===200 || !strict, `krs/extract ${label}`, r);

  // 7 get KRS
  r = await getKRS(npm); r.label=`krs/get ${label}`; steps.push(r);
  log(r.status===200 || !strict, `krs/get ${label}`, r);

  // 8 semesters must 200
  r = await khsSemesters(npm,password); r.label=`khs/semesters ${label}`; steps.push(r);
  log(r.status===200, `khs/semesters ${label}`, r);

  let semesters = pickSemesters(r);
  if(!semesters.length){
    console.log(`  │ · no semesters parsed, fallback 2022/2023 GANJIL for loop`);
    semesters = [{tahun_ajaran:"2022/2023", semester:"GANJIL"}];
  } else {
    console.log(`  │   ↳ semesters (${semesters.length}): ${semesters.map(s=>`${s.tahun_ajaran} ${s.semester}`).join(", ")}`);
  }

  // 9 loop per semester: download→extract→get
  for(const s of semesters){
    const ta=s.tahun_ajaran, sem=s.semester;
    r = await downloadKHS(npm,password,ta,sem); r.label=`khs/download ${ta} ${sem} ${label}`; steps.push(r);
    log(r.status===200 || !strict, `khs/download ${ta} ${sem}`, r);

    r = await extractKHS(npm,password,ta,sem); r.label=`khs/extract ${ta} ${sem} ${label}`; steps.push(r);
    log(r.status===200 || !strict, `khs/extract ${ta} ${sem}`, r);

    r = await getKHS(npm,ta,sem); r.label=`khs/get ${ta} ${sem} ${label}`; steps.push(r);
    log(r.status===200 || !strict, `khs/get ${ta} ${sem}`, r);
  }

  const wall = Date.now()-t0;
  console.log(`  └─ Flow ${label} done wall ${fmt(wall)} steps ${steps.length} semesters ${semesters.length}`);
  return { wall, steps, semesters };
}

function assertNo503(results, phase){
  const bad = results.filter(r=> r.status===503 || String(r.json?.message||"").toLowerCase().includes("tidak dapat diakses") );
  if(bad.length){
    console.log(`✗ ${phase}: 503 detected (${bad.length}/${results.length})`);
    bad.forEach(r=> console.log(`  503 ${r.label||r.path} npm=${r.bodyNpm} ${r.status} ${fmt(r.ms)}`));
    return false;
  }
  console.log(`✓ ${phase}: no 503`);
  return true;
}

async function healthCheck(){
  const h = await req("GET","/api/v1/health",null,8000);
  const ok = h.ok && h.status===200;
  console.log(`health: ${ok?"✓":"✗"} ${h.status} ${fmt(h.ms)} ${h.error||""}`);
  return ok;
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

  console.log("=".repeat(90));
  console.log(`E2E FULL FLOW  BASE=${globalThis.__EFFECTIVE_BASE}  NPM_A=${NPM_A}  NPM_B=${NPM_B}  strict=${strict}`);
  if(NPM_A===NPM_B) console.log("⚠ NPM_A===NPM_B — parallel not multi-account");
  console.log(`Budgets: per-req <${fmt(PER_REQ_BUDGET_MS)}  per-flow wall <${fmt(WALL_BUDGET_MS)}  reqTimeout=${fmt(REQ_TIMEOUT_MS)}  no 503`);
  console.log(`Flow: login → profile/data×2 → photo → krs dl→extract→get → semesters → loop[khs dl→extract→get]`);
  console.log("=".repeat(90));

  const healthOk = await healthCheck();
  if(!healthOk){
    console.log("✗ health failed — abort");
    if(jsonOut) console.log(JSON.stringify({ok:false, reason:"health failed", base: globalThis.__EFFECTIVE_BASE}, null, 2));
    process.exit(1);
  }

  let seqRes=null, parRes=null;
  const allSteps=[];

  if(!parOnly){
    console.log("\n── SEQUENTIAL (A→B serial full flow)");
    const t0=Date.now();
    const flowA = await runFlow(NPM_A,PASSWORD_A,"A",strict);
    const flowB = await runFlow(NPM_B,PASSWORD_B,"B",strict);
    const wall = Date.now()-t0;
    const steps = [...flowA.steps, ...flowB.steps];
    allSteps.push(...steps);
    seqRes={ wall, steps, flowA, flowB };
    console.log(` sequential wall ${fmt(wall)} total steps ${steps.length}`);
  }

  if(!seqOnly){
    console.log("\n── PARALLEL (A‖B concurrent full flow)");
    const t0=Date.now();
    const [flowA, flowB] = await Promise.all([
      runFlow(NPM_A,PASSWORD_A,"A",strict),
      runFlow(NPM_B,PASSWORD_B,"B",strict),
    ]);
    const wall = Date.now()-t0;
    const steps=[...flowA.steps, ...flowB.steps];
    allSteps.push(...steps);
    parRes={ wall, steps, flowA, flowB };
    console.log(` parallel wall ${fmt(wall)} total steps ${steps.length}`);
  }

  console.log("\n── ASSERTIONS");
  let pass=true;
  const phases=[];
  if(seqRes) phases.push(["sequential", seqRes.steps]);
  if(parRes) phases.push(["parallel", parRes.steps]);
  if(!phases.length){ console.log("✗ no phase ran"); pass=false; }

  for(const [name, steps] of phases){
    const loginOk = steps.filter(s=> s.label.startsWith("login")).every(s=> s.status===200);
    if(!loginOk){ console.log(`✗ ${name}: login not all 200`); pass=false; } else console.log(`✓ ${name}: login 200`);
    const semOk = steps.filter(s=> s.label.startsWith("khs/semesters")).every(s=> s.status===200);
    if(!semOk){ console.log(`✗ ${name}: khs/semesters not all 200`); pass=false; } else console.log(`✓ ${name}: khs/semesters 200`);

    // profile/data: warn if 404 when not strict
    if(strict){
      const bad = steps.filter(s=> s.label.includes("profile/data") && s.status!==200);
      if(bad.length){ console.log(`✗ ${name}: profile/data strict ${bad.length} not 200`); pass=false; }
      else console.log(`✓ ${name}: profile/data 200 strict`);
      const photoBad = steps.filter(s=> s.label.includes("profile/photo") && !(s.status===200||s.status===204));
      if(photoBad.length){ console.log(`✗ ${name}: photo strict ${photoBad.length} not 200/204`); pass=false; }
      else console.log(`✓ ${name}: photo 200/204 strict`);
    } else {
      console.log(`· ${name}: profile/data & photo lenient (404/204 warn, 503 fail)`);
    }

    if(strict){
      const bad = steps.filter(s=> s.status!==200 && s.status!==204);
      // 204 only for photo
      const filtered = bad.filter(s=> !(s.label.includes("profile/photo") && s.status===204));
      // Actually 204 not in bad because status 204 !=200 but we already excluded
      const downloadBad = steps.filter(s=> (s.label.includes("krs/")||s.label.includes("khs/")) && s.status!==200);
      if(downloadBad.length){ console.log(`✗ ${name}: krs/khs strict ${downloadBad.length} not 200`); pass=false; }
      else console.log(`✓ ${name}: krs/khs 200 strict`);
    } else {
      console.log(`· ${name}: krs/khs lenient (404/500 warn, 503 fail)`);
    }

    const a1 = assertNo503(steps, name);
    // wall per-flow check: each flow wall < WALL_BUDGET
    const flowWalls = name==="sequential" ? [seqRes.flowA.wall, seqRes.flowB.wall] : [parRes.flowA.wall, parRes.flowB.wall];
    const wallBad = flowWalls.filter(w=> w>=WALL_BUDGET_MS);
    if(wallBad.length){ console.log(`✗ ${name}: per-flow wall >=${fmt(WALL_BUDGET_MS)} (${wallBad.length})`); pass=false; } else console.log(`✓ ${name}: per-flow wall <${fmt(WALL_BUDGET_MS)} (${flowWalls.map(fmt).join(", ")})`);
    // per-req
    const over = steps.filter(s=> s.ms>=PER_REQ_BUDGET_MS);
    if(over.length){
      console.log(`✗ ${name}: per-req >=${fmt(PER_REQ_BUDGET_MS)} (${over.length}/${steps.length})`);
      over.forEach(s=> console.log(`  ${fmt(s.ms)} ${s.status} ${s.label}`));
      pass=false;
    } else console.log(`✓ ${name}: per-req <${fmt(PER_REQ_BUDGET_MS)} (${steps.length} ops)`);
    if(!a1) pass=false;
  }

  const any503 = allSteps.some(r=> r.status===503);
  if(any503){ console.log("✗ global: 503 present"); pass=false; } else console.log("✓ global: no 503");

  if(seqRes && parRes){
    const seqSum = seqRes.flowA.wall + seqRes.flowB.wall;
    const parWall = parRes.wall;
    const bound = seqSum * 1.5; // full flow is heavy, allow 1.5x
    if(parWall < bound) console.log(`✓ concurrency: parallel ${fmt(parWall)} < 1.5× seq sum ${fmt(seqSum)}`);
    else console.log(`· concurrency warn: parallel ${fmt(parWall)} >= 1.5× seq sum ${fmt(seqSum)} (LMS slowness, not fail)`);
  }

  const total=allSteps.length, okCount=allSteps.filter(s=> s.status===200 || s.status===204).length;
  const byStatus={}; allSteps.forEach(r=>{ const k=String(r.status||"ERR"); byStatus[k]=(byStatus[k]||0)+1; });
  console.log("\n"+ "=".repeat(90));
  console.log(`SUMMARY total=${total} ok(200/204)=${okCount} fail=${total-okCount} pass=${pass?"YES":"NO"}`);
  console.log(`by status: ${Object.entries(byStatus).map(([k,v])=>`${k}:${v}`).join(" ")}`);
  const slowest=[...allSteps].sort((a,b)=>b.ms-a.ms).slice(0,7);
  console.log("slowest 7:");
  slowest.forEach(r=> console.log(`  ${fmt(r.ms).padStart(7)} ${String(r.status).padStart(3)} ${r.label} npm=${r.bodyNpm||""}`));
  console.log("=".repeat(90));
  if(NPM_A===NPM_B) console.log("NOTE: NPM_A===NPM_B; re-run distinct for isolation proof.");

  if(jsonOut){
    const summary={ ok: pass, base: globalThis.__EFFECTIVE_BASE, npmA:NPM_A, npmB:NPM_B, strict, budgets:{perReqMs:PER_REQ_BUDGET_MS, wallMs:WALL_BUDGET_MS}, sequential: seqRes? {wallMs: seqRes.wall, flows:[seqRes.flowA.wall, seqRes.flowB.wall], steps: seqRes.steps.map(s=>({label:s.label,status:s.status,ms:s.ms}))}: null, parallel: parRes? {wallMs: parRes.wall, flows:[parRes.flowA.wall, parRes.flowB.wall], steps: parRes.steps.map(s=>({label:s.label,status:s.status,ms:s.ms}))}: null, byStatus };
    console.log(JSON.stringify(summary,null,2));
  }
  process.exit(pass?0:1);
}

main().catch(e=>{ console.error("fatal",e); process.exit(1); });
