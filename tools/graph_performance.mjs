import { chromium } from "playwright";
import { mkdir, writeFile } from "node:fs/promises";

const browser=await chromium.launch({headless:true});
const page=await browser.newPage({viewport:{width:1440,height:900}});
const started=performance.now();
await page.goto("http://127.0.0.1:18766/graph",{waitUntil:"networkidle"});
await page.waitForFunction(()=>Number(document.querySelectorAll(".perf dd")[0]?.textContent||0)>=2000,{timeout:15000});
const firstUseful=performance.now()-started;
const samples=[];
for(let i=0;i<10;i++){await page.waitForTimeout(400);samples.push(await page.evaluate(()=>Number(document.querySelectorAll(".perf dd")[4]?.textContent||0)));}
samples.sort((a,b)=>a-b);const median=(samples[4]+samples[5])/2;
const metrics=await page.evaluate(()=>({heap:performance.memory?.usedJSHeapSize||null,nodes:Number(document.querySelectorAll(".perf dd")[0]?.textContent||0),edges:Number(document.querySelectorAll(".perf dd")[1]?.textContent||0),culled_edges:Number(document.querySelectorAll(".perf dd")[2]?.textContent||0),lod:document.querySelectorAll(".perf dd")[3]?.textContent||"",fetch_ms:Number((document.querySelectorAll(".perf dd")[5]?.textContent||"0").split(" ")[0]),response_bytes:Number(document.querySelectorAll(".perf dd")[6]?.textContent||0)}));
const report={schema_version:1,environment:"headless Chromium fixture",target_fps:30,ci_floor_fps:28,first_useful_paint_ms:Math.round(firstUseful),fps_samples:samples,fps_median:median,...metrics};
await mkdir("docs/evidence/graph",{recursive:true});await writeFile("docs/evidence/graph/performance-2000.json",JSON.stringify(report,null,2)+"\n");if(median>=28)await page.screenshot({path:"docs/evidence/graph/performance-2000.png"});
console.log(JSON.stringify(report));
await browser.close();
if(metrics.nodes!==2000||median<28)process.exit(1);
