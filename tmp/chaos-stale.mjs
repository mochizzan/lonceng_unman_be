// tmp/chaos-stale.mjs — C: burst 10× profile 5‖5 tiap 200ms burst → 0×503 per-req<15s (expiry 25s <30s) MarkStale tidak deadlock pageMu/npmLock
// Std fetch only. --help exit 0. Spec C.
const BASE = process.env.BASE_URL || "http://localhost:3000";
const NPM_A = process.env.NPM_A || process.env.NPM || "2211700006";
const PASSWORD_A = process.env.PASSWORD_A || process.env.PASSWORD || "Izzan027";
const NPM_B = process.env.NPM_B || process.env.NPM2 || "2211700009";
const PASSWORD_B = process.env.PASSWORD_B || process.env.PASSWORD2 || process.env.PASSWORD || "Izzan027";
function fmt(ms){ return ms>=1000 ? (ms/1000).toFixed(2)+"s": ms+"ms"; }
function printHelp(){ console.log(`chaos-stale — C: 10× burst 5‖5 profile parallel

Usage:
  node tmp/chaos-stale.mjs [options]
  BASE_URL=http://localhost:3000 NPM_A=2211700006 PASSWORD_A=xxx NPM_B=2211700009 PASSWORD_B=yyy node tmp/chaos-stale.mjs

Options:
  --help     Show help and exit 0
  --base URL Override BASE_URL (default http://localhost:3000)
  --json     JSON summary

Flow: 10× POST /lms/student-profile burst 5×A ‖ 5×B tiap 200ms interval — MarkStale tidak deadlock pageMu/npmLock
Assert: 10/10 200 0×503 per-req<30s (expiry 25s allowed) wall <60s`); }
async function req(method, path, body, timeoutMs=90000){
  const url=(globalThis.__EFFECTIVE_BASE||BASE)+path; const t0=Date.now(); const ctrl=new AbortController(); const to=setTimeout(()=>ctrl.abort(),timeoutMs);
  try{ const opts={method, headers:{}, signal:ctrl.signal}; if(method!=="GET"){ opts.headers["Content-Type"]="application/json"; opts.body=JSON.stringify(body||{}); } const res=await fetch(url,opts); let raw=await res.text(); let j=null; try{ j=JSON.parse(raw);}catch{ j={raw:raw.slice(0,1200)}; } clearTimeout(to); return {ok:res.ok,status:res.status, ms:Date.now()-t0, json:j, raw, path, npm: body?.npm||null}; }catch(e){ clearTimeout(to); return {ok:false,status:0, ms:Date.now()-t0,error:`${e.name}: ${e.message}`,json:null,raw:"",path, npm: body?.npm||null}; }
}
async function main(){
  const args=process.argv.slice(2);
  if(args.includes("--help")||args.includes("-h")){ printHelp(); process.exit(0); }
  for(let i=0;i<args.length;i++){ if(args[i]==="--base"&&args[i+1]) globalThis.__EFFECTIVE_BASE=args[++i]; if(args[i].startsWith("--base=")) globalThis.__EFFECTIVE_BASE=args[i].split("=")[1]; }
  globalThis.__EFFECTIVE_BASE=globalThis.__EFFECTIVE_BASE||BASE;
  const jsonOut=args.includes("--json");
  console.log("=".repeat(78));
  console.log(`CHAOS-STALE  BASE=${globalThis.__EFFECTIVE_BASE}  NPM_A=${NPM_A} NPM_B=${NPM_B} 10× burst 5‖5`);
  console.log("=".repeat(78));
  let all=[];
  for(let burst=0;burst<2;burst++){
    console.log(` burst ${burst+1}/2: 5‖5 parallel`);
    const batch = await Promise.all([
      req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A}),
      req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A}),
      req("POST","/api/v1/lms/student-profile",{npm:NPM_A,password:PASSWORD_A}),
      req("POST","/api/v1/lms/student-profile",{npm:NPM_B,password:PASSWORD_B}),
      req("POST","/api/v1/lms/student-profile",{npm:NPM_B,password:PASSWORD_B}),
    ]);
    batch.forEach((r,i)=> console.log(`  ${r.status===200?"✓":"✗"} ${fmt(r.ms).padStart(7)} ${String(r.status).padStart(3)} npm=${r.npm} ${r.error||""}`));
    all.push(...batch);
    if(burst<1) await new Promise(res=>setTimeout(res,200));
  }
  console.log("\n── ASSERTIONS");
  let pass=true;
  const okCount=all.filter(r=> r.status===200).length;
  if(okCount!==10){ console.log(`✗ not all 200 ${okCount}/10`); pass=false; } else console.log(`✓ all 200 10/10`);
  const bad503=all.filter(r=> r.status===503); if(bad503.length){ console.log(`✗ 503 ${bad503.length}`); pass=false; } else console.log(`✓ no 503`);
  const over=all.filter(r=> r.ms>=30000); if(over.length){ console.log(`✗ per-req >=30s ${over.length}`); pass=false; } else console.log(`✓ per-req <30s (expiry 25s allowed)`);
  const wall=all.reduce((s,r)=> s+r.ms,0); console.log(` wall sum ${fmt(wall)} burst done`);
  const byStatus={}; all.forEach(r=>{ const k=String(r.status||"ERR"); byStatus[k]=(byStatus[k]||0)+1; });
  console.log("\n"+ "=".repeat(78));
  console.log(`SUMMARY ok=${pass?"YES":"NO"} total=10 ok=${okCount} byStatus ${Object.entries(byStatus).map(([k,v])=>`${k}:${v}`).join(" ")}`);
  console.log("=".repeat(78));
  if(jsonOut) console.log(JSON.stringify({ok:pass, base:globalThis.__EFFECTIVE_BASE, total:10, okCount, byStatus, results: all.map(r=>({status:r.status,ms:r.ms,npm:r.npm}))},null,2));
  process.exit(pass?0:1);
}
main().catch(e=>{ console.error(e); process.exit(1); });
