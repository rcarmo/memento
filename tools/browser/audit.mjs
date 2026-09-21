import { chromium, firefox, webkit } from 'playwright';
import { spawn } from 'node:child_process';
import { readFile, writeFile, mkdir, mkdtemp, rm } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
import path from 'node:path';
import { auditDiagnostics } from './diagnostics-audit.mjs';
const root=fileURLToPath(new URL('../../',import.meta.url));
const engine=process.env.UI_BROWSER||'chromium';
if (!['chromium','webkit','firefox'].includes(engine)) throw new Error('UI_BROWSER must be chromium, webkit or firefox');
const parent=path.join(root,'build/ui-audit',engine);await mkdir(parent,{recursive:true});
// Readiness files, exports, logs and screenshots belong to this invocation only.
const output=await mkdtemp(path.join(parent,'run-'));
const ready=path.join(output,'ready');
const child=spawn('go',['test','./internal/service','-run','^TestUIAuditServer$','-count=1','-v','-timeout=14m'],{cwd:root,env:{...process.env,CGO_ENABLED:'0',GOTOOLCHAIN:'go1.26.6',MEMENTO_UI_AUDIT_READY:ready},detached:process.platform!=='win32',stdio:['ignore','pipe','pipe']});
let logs='';child.stdout.on('data',b=>logs+=b);child.stderr.on('data',b=>logs+=b);
let exited=false;const exit=new Promise(resolve=>{child.on('error',error=>{logs+=error.stack;exited=true;resolve(-1)});child.on('exit',code=>{exited=true;resolve(code)})});
let browser,base,webgl2,stopping;const results=[];const delay=ms=>new Promise(r=>setTimeout(r,ms));
const terminate=signal=>{try{if(process.platform==='win32')child.kill(signal);else process.kill(-child.pid,signal)}catch(error){if(error.code!=='ESRCH')throw error}};
async function shutdown(){
 if(stopping)return stopping;
 stopping=(async()=>{
  await browser?.close().catch(()=>{});
  await writeFile(ready+'.stop','stop');
  let timer;
  const code=await Promise.race([exit,new Promise(resolve=>{timer=setTimeout(()=>{terminate('SIGTERM');resolve(-1)},10000)})]);clearTimeout(timer);
  if(code===-1&&!exited){await delay(500);terminate('SIGKILL')}
  await writeFile(path.join(output,'server.log'),logs);
  await rm(ready,{force:true});await rm(ready+'.stop',{force:true});
  if(code!==0){console.error(logs);process.exitCode=1}
 })();return stopping;
}
for(const signal of ['SIGINT','SIGTERM'])process.once(signal,()=>{process.exitCode=signal==='SIGINT'?130:143;void shutdown()});
async function eventually(fn, timeout=120000){const deadline=Date.now()+timeout;while(Date.now()<deadline){if(exited)throw new Error(logs);try{return await fn()}catch{}await delay(100)}throw new Error(`condition timed out after ${timeout}ms\n${logs}`)}
try {
 base=await eventually(()=>readFile(ready,'utf8'));
 browser=await ({chromium,firefox,webkit}[engine]).launch({headless:true,...(engine==='chromium'?{args:['--no-sandbox','--use-gl=angle','--use-angle=swiftshader','--enable-unsafe-swiftshader']}:{})});
 const lock=await readFile(path.join(root,'tools/browser/package-lock.json'));
 await writeFile(path.join(output,'environment.json'),JSON.stringify({engine,browserVersion:browser.version(),node:process.version,platform:process.platform,arch:process.arch,goToolchain:'go1.26.6',lockSHA256:createHash('sha256').update(lock).digest('hex')},null,2));
 const probe=await browser.newPage();webgl2=await probe.evaluate(()=>!!document.createElement('canvas').getContext('webgl2'));await probe.close();
 // Chromium/WebKit are required gates. Firefox is explicitly fallback/admin only.
 if(!webgl2&&engine!=='firefox')throw new Error(engine+' requires WebGL2; graph cases must not silently skip');
 async function check(name,fn){
  if(stopping)throw new Error('Browser audit interrupted');
  if(!webgl2&&!/WebGL unavailable|real admin authentication|admin buttons/.test(name)){results.push({name,status:'not_run',reason:engine+' headless environment has no WebGL2'});return}
  const context=await browser.newContext({viewport:{width:1440,height:1000},acceptDownloads:true,locale:'en-GB',timezoneId:'UTC',colorScheme:'dark'});
  await context.tracing.start({screenshots:true,snapshots:true,sources:true});
  const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
  let testTimer;
  try{await Promise.race([fn(page,context),new Promise((_,reject)=>{testTimer=setTimeout(()=>reject(new Error('Browser case exceeded 90 seconds')),90000)})]);assert.deepEqual(errors,[]);results.push({name,status:'passed'});console.log('PASS',name);await context.tracing.stop()}
  catch(e){results.push({name,status:'failed',error:e.stack});await page.locator('body').innerText().then(text=>writeFile(path.join(output,'failure-'+results.length+'.txt'),text)).catch(()=>{});await page.screenshot({path:path.join(output,'failure-'+results.length+'.png')}).catch(()=>{});await context.tracing.stop({path:path.join(output,'failure-'+results.length+'-trace.zip')}).catch(()=>{});console.error('FAIL',name,e.message)}
  finally{clearTimeout(testTimer);await context.close().catch(()=>{})}
 }
 // Wait for rendering tasks without assuming a machine-dependent delay.
 const rendered=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
 const graph=async(page)=>{await page.goto(base+'/graph');await page.waitForFunction(()=>window.__mementoGraphScene?.nodes.length===120)};
 const scene=async(page,fn)=>page.evaluate(fn);
 const nodes=async page=>scene(page,()=>window.__mementoGraphScene.nodes.length);
 const select=async(page,index=0)=>page.evaluate(i=>{const s=window.__mementoGraphScene;s.callbacks.select(s.nodes[i])},index);
 const stop=async page=>page.evaluate(()=>{const s=window.__mementoGraphScene;s.worker.postMessage({type:'stop'});s.layoutId++;s.cancelFocus()});
 const waitNodes=async(page,n)=>page.waitForFunction(n=>window.__mementoGraphScene.nodes.length===n,n);
 const fixtureNode=(id,n=0)=>({id,path:'/projects/'+id+'.md',title:'Fixture '+id,type:'project',status:'active',tags:['audit'],namespace:'/projects/',combined_bytes:100+n,coarse_position:{x:n*2,y:0,z:0},embedding:{status:'ready'},anomaly_ids:[]});
 await check('real Go startup, filtering, metrics, semantics, forces and viewport',async(page)=>{
  await graph(page);assert.equal(await nodes(page),120);assert.ok(await page.locator('.cluster-label').count()>=3);
  await page.locator('.controls label').filter({hasText:/^Type/}).locator('select').selectOption('skill');await waitNodes(page,40);await page.locator('.controls label').filter({hasText:/^Type/}).locator('select').selectOption('all');await waitNodes(page,120);
  for(const metric of ['markdown_bytes','asset_bytes','explicit_in_degree','explicit_out_degree','combined_bytes']){await page.locator('.controls label').filter({hasText:/^Size/}).locator('select').selectOption(metric);await page.waitForFunction(m=>window.__mementoGraphScene.settings.sizeMetric===m,metric)}
  await page.getByLabel('Show semantic layer',{exact:true}).check();await page.waitForFunction(()=>window.__mementoGraphScene.edges.some(e=>e.kind==='semantic_similarity'));
  await page.getByText('Forces',{exact:true}).click();const force=page.locator('label.slider').filter({hasText:/^explicit /}).locator('input');await force.fill('0.12');await force.dispatchEvent('input');await page.waitForFunction(()=>window.__mementoGraphScene.settings.forces.explicit===.12);
  await page.getByRole('button',{name:'Reset forces'}).click();await page.waitForFunction(()=>window.__mementoGraphScene.settings.forces.explicit===.06);
  const id=await scene(page,()=>window.__mementoGraphScene.layoutId);await page.locator('label.slider').filter({hasText:'Semantic opacity'}).locator('input').fill('0.2');await page.waitForFunction(()=>window.__mementoGraphScene.settings.semanticAlpha===.2);assert.equal(await scene(page,()=>window.__mementoGraphScene.layoutId),id);
  const canvas=page.locator('canvas');const box=await canvas.boundingBox();await page.mouse.move(box.x+box.width/2,box.y+box.height/2);const distance=await scene(page,()=>window.__mementoGraphScene.distance);await page.mouse.wheel(0,120);await page.waitForFunction(d=>window.__mementoGraphScene.distance>d,distance);
  await page.setViewportSize({width:1024,height:1366});await page.screenshot({path:path.join(output,'graph-tablet.png')});assert.ok((await canvas.boundingBox()).width>200);
 });
 await check('real Go inspector, references, search, tag filtering and deselection',async(page)=>{
  await graph(page);await select(page);await page.locator('.inspector h2').waitFor();await page.locator('.inspector h3').filter({hasText:'Explicit links'}).waitFor();await page.getByTestId('inspector-loading').waitFor({state:'hidden'});
  await page.locator('.inspector .tag').first().click();assert.equal(await page.getByPlaceholder('title, tags, full text').inputValue(),'audit');
  await page.getByRole('button',{name:'Overview',exact:true}).click();assert.equal(await page.getByPlaceholder('title, tags, full text').inputValue(),'');await page.locator('.inspector h2').waitFor({state:'hidden'});
  await page.getByPlaceholder('title, tags, full text').fill('Node 003');await page.locator('.search-results button').first().click();await page.locator('.inspector h2').filter({hasText:'Node 003'}).waitFor();
  await page.evaluate(()=>window.__mementoGraphScene.callbacks.select(null));await page.locator('.inspector h2').waitFor({state:'hidden'});
 });
 await check('real Go visibility and trash filters, refresh disabled in simulation',async(page)=>{
  await graph(page);await page.getByLabel('View as',{exact:false}).selectOption('projects-reader');await waitNodes(page,40);assert.ok((await scene(page,()=>window.__mementoGraphScene.nodes.map(n=>n.path))).every(p=>p.startsWith('/projects/')));
  for(const label of ['Refresh selected embedding','Refresh visible','Refresh all'])assert.equal(await page.getByRole('button',{name:label,exact:true}).isDisabled(),true);
  await page.getByLabel('View as',{exact:false}).selectOption('');await waitNodes(page,120);await page.getByLabel('Show Trash').check();await waitNodes(page,121);await page.getByLabel('Show Trash').uncheck();await waitNodes(page,120);
 });
 await check('real Go PNG SVG JSON exports, metadata boundaries and URL cleanup',async(page)=>{
  await graph(page);await page.getByRole('button',{name:'Export current graph'}).click();assert.equal(await page.getByLabel('Include bounded preview where allowed').count(),0);await page.evaluate(()=>{window.auditURLs=[];window.auditRevoked=[];const make=URL.createObjectURL,revoke=URL.revokeObjectURL;URL.createObjectURL=b=>{const u=make(b);window.auditURLs.push(u);return u};URL.revokeObjectURL=u=>{window.auditRevoked.push(u);revoke(u)}});
  for(const format of ['JSON','SVG','PNG']){
   if(!await page.locator('dialog').isVisible())await page.getByRole('button',{name:'Export current graph'}).click();
   const pending=page.waitForEvent('download');await page.locator('.format-grid button').filter({hasText:format}).click();const download=await pending;const dest=path.join(output,download.suggestedFilename());await download.saveAs(dest);const data=await readFile(dest);assert.ok(data.length>100);
   if(format==='JSON'){const doc=JSON.parse(data);assert.equal(doc.nodes.length,120);assert.equal(doc.settings.include_preview,undefined)}
   if(format==='SVG')assert.match(data.toString(),/<svg/);
   if(format==='PNG')assert.equal(data.subarray(1,4).toString(),'PNG');
  }
  await page.waitForFunction(()=>window.auditURLs.length===window.auditRevoked.length);
 });
 await check('aggregated cluster expansion, member navigation and disabled aggregate actions',async(page)=>{
  const cluster={id:'cluster:test',label:'Test cluster',namespace:'/projects/',member_count:2,type_counts:[['project',2]],combined_bytes:250,coarse_position:{x:0,y:0,z:0}};
  await page.route('**/graph/api/v1/overview',r=>r.fulfill({json:{mode:'aggregated',clusters:[cluster],cluster_edges:[],nodes:[],edges:[],diagnostics:[],revisions:{}}}));
  await page.route('**/graph/api/v1/clusters/*',r=>r.fulfill({json:{nodes:[fixtureNode('a'),fixtureNode('b',1)],edges:[],truncated:false}}));
  await page.route('**/graph/api/v1/embeddings/status',r=>r.fulfill({json:{available:true,alive:true,completed:0}}));
  await page.goto(base+'/graph');await waitNodes(page,1);assert.ok(await page.locator('.controls label').filter({hasText:/^Size/}).locator('select').locator('option[value=member_count]').count());assert.equal(await page.getByRole('button',{name:'Refresh visible',exact:true}).isDisabled(),true);
  await select(page);await waitNodes(page,2);await page.locator('.inspector summary').filter({hasText:'Cluster members (2)'}).waitFor();assert.equal(await page.locator('.controls label').filter({hasText:/^Size/}).locator('select').locator('option[value=member_count]').count(),0);
 });
 await check('large graph Points picking/hover and pinch/cancel never selects',async(page)=>{
  await graph(page);await stop(page);
  await page.evaluate(()=>{const s=window.__mementoGraphScene;const template=s.nodes[0];const nodes=Array.from({length:1001},(_,i)=>({...template,id:'large-'+i,title:'Large '+i,namespace:'',coarse_position:{x:i?30+i:0,y:0,z:0}}));s.setGraph(nodes,[],{forces:{repulsion:0,explicit:0}});s.worker.postMessage({type:'stop'});s.layoutId++;s.yaw=0;s.pitch=0;s.distance=16;s.target.set(0,0,0)});
  await rendered(page);const box=await page.locator('canvas').boundingBox();await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.locator('.node-label').waitFor({state:'visible'});assert.equal(await page.locator('.node-label').innerText(),'Large 0');
  // Replace selection callback only for gesture isolation; picking still uses real Three raycasts.
  await page.evaluate(()=>{const s=window.__mementoGraphScene;s.callbacks.select=n=>{window.auditPicked=n?.id;window.auditPicks=(window.auditPicks||0)+1}});
  await page.mouse.click(box.x+box.width/2,box.y+box.height/2);assert.equal(await page.evaluate(()=>window.auditPicked),'large-0');
  await page.evaluate(()=>{const s=window.__mementoGraphScene,c=s.canvas;c.setPointerCapture=()=>{};const ev=(type,id,x)=>c.dispatchEvent(new PointerEvent(type,{pointerId:id,clientX:x,clientY:200,pointerType:'touch'}));window.auditPicks=0;ev('pointerdown',1,100);ev('pointerdown',2,200);ev('pointermove',2,240);ev('pointerup',2,240);ev('pointerup',1,100);ev('pointerdown',3,100);ev('pointercancel',3,100);ev('pointerup',3,100)});
  assert.equal(await page.evaluate(()=>window.auditPicks),0);assert.equal(await scene(page,()=>window.__mementoGraphScene.pointers.size),0);
 });
 await check('selection errors end loading; slow navigation cannot revive cleared inspector',async(page)=>{
  await graph(page);let release;const held=new Promise(r=>release=r);
  await page.route('**/graph/api/v1/memories/*',async r=>{await held;await r.fulfill({status:404,json:{error:'fixture missing'}}).catch(()=>{})});
  await select(page);await page.getByTestId('inspector-loading').waitFor();release();await page.getByTestId('inspector-error').waitFor();await page.getByTestId('inspector-loading').waitFor({state:'hidden'});
  await page.unroute('**/graph/api/v1/memories/*');let release2;const held2=new Promise(r=>release2=r);
  let started2,finished2;const requested2=new Promise(r=>started2=r),handled2=new Promise(r=>finished2=r);
  await page.route('**/graph/api/v1/memories/*',async r=>{started2();await held2;await r.continue().catch(()=>{});finished2()});
  await page.getByPlaceholder('title, tags, full text').fill('Node 003');await page.locator('.search-results button').first().click();await requested2;await page.getByRole('button',{name:'Overview',exact:true}).click();release2();await handled2;await rendered(page);await page.locator('.inspector h2').waitFor({state:'hidden'});
 });
 await check('slow overview cannot overwrite principal simulation',async(page)=>{
  let first=true,release,finished;const held=new Promise(r=>release=r),handled=new Promise(r=>finished=r);
  await page.route('**/graph/api/v1/overview',async r=>{if(first){first=false;await held;await r.fulfill({json:{mode:'direct',nodes:[fixtureNode('LEAK')],edges:[],clusters:[],diagnostics:[],revisions:{}}}).catch(()=>{});finished()}else await r.continue()});
  await page.goto(base+'/graph');await page.getByLabel('View as',{exact:false}).selectOption('projects-reader');await waitNodes(page,40);release();await handled;await rendered(page);assert.equal(await nodes(page),40);assert.equal(await page.locator('.toast-error').count(),0);
 });
 await check('refresh selected/visible/full confirmation and error handling',async(page)=>{
  await page.route('**/graph/api/v1/embeddings/status',r=>r.fulfill({json:{available:true,alive:true,completed:0}}));const requests=[];
  await page.route('**/graph/api/v1/embeddings/refresh',r=>{requests.push(r.request().postDataJSON());return r.fulfill({status:202,json:{available:true,alive:true,pending:true,completed:0}})});
  await graph(page);await select(page);await page.getByRole('button',{name:'Refresh selected embedding'}).click();assert.equal(requests.at(-1).scope,'selected');assert.equal(requests.at(-1).concept_ids.length,1);
  await page.getByRole('button',{name:'Refresh visible',exact:true}).click();assert.equal(requests.at(-1).concept_ids.length,120);
  page.once('dialog',d=>d.dismiss());await page.getByRole('button',{name:'Refresh all',exact:true}).click();assert.equal(requests.length,2);
  page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Refresh all',exact:true}).click();assert.equal(requests.at(-1).confirm_full,true);
  await page.unroute('**/graph/api/v1/embeddings/refresh');await page.route('**/graph/api/v1/embeddings/refresh',r=>r.fulfill({status:503,json:{error:'worker unavailable'}}));await page.getByRole('button',{name:'Refresh visible',exact:true}).click();await page.locator('.toast-error').filter({hasText:'worker unavailable'}).waitFor();
 });
 await check('empty graph supports pointer hover, click and export controls',async(page)=>{
  await page.route('**/graph/api/v1/overview',r=>r.fulfill({json:{mode:'direct',nodes:[],edges:[],clusters:[],cluster_edges:[],diagnostics:[],revisions:{}}}));
  await page.goto(base+'/graph');await page.waitForFunction(()=>window.__mementoGraphScene&&window.__mementoGraphScene.nodes.length===0);
  const box=await page.locator('canvas').boundingBox();await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.mouse.click(box.x+box.width/2,box.y+box.height/2);assert.equal(await page.locator('.node-label').isVisible(),false);
  await page.getByRole('button',{name:'Export current graph'}).click();for(const name of ['JSON','SVG'])assert.equal(await page.locator('.format-grid button').filter({hasText:name}).isDisabled(),true);
 });
 await check('scene disposal stops animation and removes overlay DOM',async(page)=>{await graph(page);await page.evaluate(()=>window.__mementoGraphScene.dispose());assert.equal(await page.locator('.node-label,.cluster-label').count(),0);const frame=await scene(page,()=>window.__mementoGraphScene.lastFrame);await rendered(page);assert.equal(await scene(page,()=>window.__mementoGraphScene.lastFrame),frame)});
 await check('WebGL unavailable produces a visible diagnostic',async(page)=>{await page.addInitScript(()=>{const get=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){return type==='webgl2'?null:get.call(this,type,...args)}});await page.goto(base+'/graph');await page.locator('.toast-error').filter({hasText:'WebGL2 is unavailable'}).waitFor()});
 await check('real admin authentication, principal lifecycle, credential secrecy and activity',async(page)=>{
  await page.goto(base+'/admin');await page.locator('#token').fill('ui-reader-token');await page.locator('#login').click();await page.locator('#loginError').filter({hasText:'admin bearer credential required'}).waitFor();
  await page.locator('#token').fill('ui-admin-token');await page.locator('#login').click();await page.locator('#content .card').first().waitFor();assert.equal(await page.locator('#token').inputValue(),'');
  await page.getByRole('button',{name:'Create',exact:true}).click();await page.locator('#name').fill('ui-created');await page.locator('#preset').selectOption('reader');await page.locator('#create').click();await page.locator('#secret').waitFor({state:'visible'});const token=(await page.locator('#secretValue').innerText()).trim();assert.ok(token.length>20);
  await page.locator('#closeSecret').click();assert.equal(await page.locator('#secretValue').innerText(),'');
  const api=async(action,body={})=>page.evaluate(async({action,body})=>{const r=await fetch('/admin/api/principals/ui-created/'+action,{method:'POST',headers:{Authorization:'Bearer ui-admin-token','Content-Type':'application/json'},body:JSON.stringify(body)});return {status:r.status,body:await r.json()}},{action,body});
  assert.equal((await api('update',{roles:['reader'],read_prefixes:['/projects/'],write_prefixes:[]})).status,200);
  assert.equal((await api('disable')).status,200);assert.equal((await api('enable')).status,200);
  const rotated=await api('rotate');assert.equal(rotated.status,200);assert.notEqual(rotated.body.credential,token);
  assert.equal((await api('revoke')).status,200);assert.equal((await api('disable')).status,200);assert.equal((await api('delete')).status,200);
  await page.getByRole('button',{name:'Activity',exact:true}).click();await page.locator('#content').filter({hasText:'ui-created'}).waitFor();assert.ok(!(await page.locator('body').innerText()).includes(token));
  assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
  await page.locator('#lock').click();assert.equal(await page.locator('#content').innerText(),'');
  await page.screenshot({path:path.join(output,'admin-locked.png')});
 });

 await check('admin buttons, errors, rename, rotation, copy, Escape and stale session responses',async(page,context)=>{
  if(engine==='chromium')await context.grantPermissions(['clipboard-read','clipboard-write']);
  else await page.addInitScript(()=>Object.defineProperty(navigator,'clipboard',{value:{writeText:async value=>{window.auditClipboard=value},readText:async()=>window.auditClipboard}}));
  await page.goto(base+'/admin');await page.locator('#token').fill('ui-admin-token');await page.locator('#login').click();await page.locator('#content .card').first().waitFor();
  await page.getByRole('button',{name:'Create',exact:true}).click();await page.locator('#name').fill('ui-buttons');await page.locator('#create').click();await page.locator('#secret').waitFor({state:'visible'});const secret=await page.locator('#secretValue').innerText();
  await page.locator('#copy').click();assert.ok((await page.evaluate(()=>navigator.clipboard.readText())).includes(secret));
  await page.keyboard.press('Escape');await page.waitForFunction(()=>document.querySelector('#secretValue').textContent==='');
  await page.getByRole('button',{name:'Principals',exact:true}).click();
  const card=()=>page.locator('#content article').filter({has:page.locator('strong').filter({hasText:/^ui-buttons$/})});await card().waitFor();
  for(const action of ['Disable','Enable']){page.once('dialog',d=>d.accept());await card().getByRole('button',{name:action,exact:true}).click();await card().getByRole('button',{name:action==='Disable'?'Enable':'Disable',exact:true}).waitFor()}
  const prompts=['reader','/projects/',''];const onPrompt=async d=>d.accept(prompts.shift());page.on('dialog',onPrompt);const updated=page.waitForResponse(r=>r.url().endsWith('/admin/api/principals/ui-buttons/update')&&r.request().method()==='POST');await card().getByRole('button',{name:'Edit access'}).click();assert.equal((await updated).status(),200);page.off('dialog',onPrompt);assert.equal(prompts.length,0);
  await page.route('**/admin/api/principals/ui-buttons/rotate',r=>r.fulfill({status:400,json:{error:'fixture rotate blocked'}}));page.once('dialog',d=>d.accept());await card().getByRole('button',{name:'Rotate credential'}).click();await page.locator('#actionError').filter({hasText:'fixture rotate blocked'}).waitFor();await page.unroute('**/admin/api/principals/ui-buttons/rotate');
  page.once('dialog',d=>d.accept());await card().getByRole('button',{name:'Rotate credential'}).click();await page.locator('#secret').waitFor({state:'visible'});assert.notEqual(await page.locator('#secretValue').innerText(),secret);await page.locator('#closeSecret').click();
  const renameDialogs=d=>d.accept(d.type()==='prompt'?'ui-renamed':undefined);page.on('dialog',renameDialogs);await card().getByRole('button',{name:'Rename',exact:true}).click();await page.locator('#content strong').filter({hasText:/^ui-renamed$/}).waitFor();page.off('dialog',renameDialogs);
  let release,finished,started;const held=new Promise(r=>release=r),handled=new Promise(r=>finished=r),requested=new Promise(r=>started=r);await page.route('**/admin/api/activity',async r=>{started();await held;await r.continue().catch(()=>{});finished()});await page.getByRole('button',{name:'Activity',exact:true}).click();await requested;await page.locator('#lock').click();release();await handled;await rendered(page);assert.equal(await page.locator('#content').innerText(),'');assert.equal(await page.locator('#secretValue').innerText(),'');
 });
 await check('search reply cannot repopulate results after changing principal',async(page)=>{
  await graph(page);let release;const held=new Promise(r=>release=r);let started,finished;const requested=new Promise(r=>started=r),handled=new Promise(r=>finished=r);
  await page.route('**/graph/api/v1/search',async r=>{started();await held;await r.fulfill({json:{results:[{id:'private',path:'/private/leak.md',title:'Private leak'}]}}).catch(()=>{});finished()});
  await page.getByPlaceholder('title, tags, full text').fill('private');await requested;await page.getByLabel('View as',{exact:false}).selectOption('projects-reader');await waitNodes(page,40);release();await handled;await rendered(page);assert.equal(await page.locator('.search-results').count(),0);
 });
 await check('refresh completion preserves semantic and filter settings',async(page)=>{
  await page.clock.install();
  let completed=0;await page.route('**/graph/api/v1/embeddings/status',r=>r.fulfill({json:{available:true,alive:true,completed,embedding_revision:'partial'}}));
  await graph(page);await page.getByLabel('Show semantic layer',{exact:true}).check();await page.locator('.controls label').filter({hasText:/^Type/}).locator('select').selectOption('skill');await waitNodes(page,40);completed=1;
  const reload=page.waitForResponse(r=>r.url().endsWith('/api/v1/overview'));await page.clock.fastForward(15001);await reload;await waitNodes(page,40);assert.equal(await page.getByLabel('Show semantic layer',{exact:true}).isChecked(),true);await page.waitForFunction(()=>window.__mementoGraphScene.edges.some(e=>e.kind==='semantic_similarity'));
 });
 await check('diagnostic scope, target navigation, retry, versions and alignment',async(page)=>{await page.clock.install();await graph(page);await auditDiagnostics(page)});
 await check('API failure, retry and error dismissal',async(page)=>{
  await page.route('**/graph/api/v1/overview',r=>r.fulfill({status:500,json:{error:'snapshot offline'}}));await page.goto(base+'/graph');await page.locator('.toast-error').filter({hasText:'snapshot offline'}).waitFor();await page.locator('.toast-error').click();assert.equal(await page.locator('.toast-error').count(),0);await page.unroute('**/graph/api/v1/overview');await page.getByRole('button',{name:'Overview',exact:true}).click();await waitNodes(page,120);
 });
 console.log(JSON.stringify({passed:results.filter(r=>r.status==='passed').length,failed:results.filter(r=>r.status==='failed').length,notRun:results.filter(r=>r.status==='not_run').length}));
 if(results.some(r=>r.status==='failed'))process.exitCode=1;
 }catch(error){results.push({name:'audit setup/runtime',status:'failed',error:error.stack});console.error(error);process.exitCode=1}
finally{
 await shutdown();
 await writeFile(path.join(output,'results.json'),JSON.stringify({engine,webgl2,base,results},null,2));
 console.log('UI artifacts: '+path.relative(root,output));
}
