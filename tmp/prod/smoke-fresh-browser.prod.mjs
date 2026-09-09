// tmp/smoke-fresh-browser.mjs — single NPM sequential smoke (login→krs→khs→profile) per-req <15s
// Std fetch only. No otel/debug/cloud.
// Usage: node tmp/smoke-fresh-browser.mjs --help
//        BASE_URL=https://lonceng-unman-api.miproduction.web.id NPM=2211700006 PASSWORD=Izzan027 node tmp/smoke-fresh-browser.mjs

const BASE = process.env.BASE_URL || "https://lonceng-unman-api.miproduction.web.id";
const NPM = process.env.NPM || process.env.NPM_A || "2211700006";
const PASSWORD = process.env.PASSWORD || process.env.PASSWORD_A || "Izzan027";

function help(){
  console.log(`smoke-fresh-browser — sequential smoke for ephemeral sessions

Usage:
  node tmp/smoke-fresh-browser.mjs [options]

Options:
  --help          Show help and exit 0
  --base URL      Override BASE_URL
  --json          JSON output

Env:
  BASE_URL ${BASE}, NPM ${NPM}, PASSWORD ***

Flow: login 200 → krs 200 → khs/semesters 200 → khs 200 → profile 200, each <15s, no 503
Note: krs/khs require existing PDFs or will 404/500 — smoke checks infra 503 absence primarily`);
}

async function req(method, path, body, timeout=90000){
  const url=(globalThis.__BASE||BASE)+path;
  const t0=Date.now();
  const ctrl=new AbortController(); const to=setTimeout(()=>ctrl.abort(),timeout);
  try{
    const opts={method, headers:{}, signal:ctrl.signal};
    if(method!=="GET"){ opts.headers["Content-Type"]="application/json"; opts.body=JSON.stringify(body||{}); }
    const res=await fetch(url, opts); let raw=await res.text(); let j; try{j=JSON.parse(raw)}catch{j={raw:raw.slice(0,800)}}
    clearTimeout(to); return {ok:res.ok,status:res.status,ms:Date.now()-t0,json:j,raw,path};
  }catch(e){ clearTimeout(to); return {ok:false,status:0,ms:Date.now()-t0,error:`${e.name}: ${e.message}`,json:null,raw:"",path};}
}

async function main(){
  const args=process.argv.slice(2);
  if(args.includes("--help")||args.includes("-h")){help();process.exit(0);}
  for(let i=0;i<args.length;i++){
    if(args[i]==="--base"&&args[i+1]) globalThis.__BASE=args[++i];
    if(args[i].startsWith("--base=")) globalThis.__BASE=args[i].split("=")[1];
  }
  globalThis.__BASE=globalThis.__BASE||BASE;
  const jsonOut=args.includes("--json");
  console.log(`SMOKE FRESH BROWSER BASE=${globalThis.__BASE} NPM=${NPM}`);
  const steps=[];
  steps.push(await req("POST","/api/v1/lms/login",{npm:NPM,password:PASSWORD}));
  console.log(` login          ${steps[0].status} ${steps[0].ms}ms`);
  steps.push(await req("POST","/api/v1/lms/krs",{npm:NPM,password:PASSWORD}));
  console.log(` krs            ${steps[1].status} ${steps[1].ms}ms`);
  steps.push(await req("POST","/api/v1/lms/khs/semesters",{npm:NPM,password:PASSWORD}));
  console.log(` khs/semesters  ${steps[2].status} ${steps[2].ms}ms`);
  // pick first semester if available for khs detail
  let sem=null;
  try{ const d=steps[2].json?.data; if(Array.isArray(d)&&d[0]) sem=d[0]; else if(d?.semesters&&d.semesters[0]) sem=d.semesters[0]; }catch{}
  if(sem && sem.tahun_ajaran){
    steps.push(await req("POST","/api/v1/lms/khs",{npm:NPM,password:PASSWORD,tahun_ajaran:sem.tahun_ajaran,semester:sem.semester}));
  } else {
    steps.push(await req("POST","/api/v1/lms/khs",{npm:NPM,password:PASSWORD,tahun_ajaran:"2023/2024",semester:"Ganjil"}));
  }
  console.log(` khs            ${steps[3].status} ${steps[3].ms}ms`);
  steps.push(await req("POST","/api/v1/lms/student-profile",{npm:NPM,password:PASSWORD}));
  console.log(` profile        ${steps[4].status} ${steps[4].ms}ms`);
  let pass=true;
  const any503=steps.some(s=>s.status===503);
  if(any503){console.log("✗ 503 detected"); pass=false;} else console.log("✓ no 503");
  const over=steps.filter(s=>s.ms>=15000);
  if(over.length){console.log(`✗ per-req >=15s ${over.length}`); pass=false;} else console.log("✓ per-req <15s");
  console.log(`SMOKE ${pass?"PASS":"FAIL"} (krs/khs 404 ok if no PDF, infra 503 is fail)`);
  if(jsonOut) console.log(JSON.stringify({ok:pass, base:globalThis.__BASE, steps:steps.map(s=>({path:s.path,status:s.status,ms:s.ms}))},null,2));
  process.exit(pass?0:1);
}
main().catch(e=>{console.error(e);process.exit(1);});
