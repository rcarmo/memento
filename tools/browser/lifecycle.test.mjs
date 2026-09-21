import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { readFile, access } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root=fileURLToPath(new URL('../../',import.meta.url));
test('interruption stops the disposable Go fixture and writes failure evidence', {timeout:120000}, async()=>{
  const child=spawn(process.execPath,['tools/browser/audit.mjs'],{
    cwd:root,env:{...process.env,UI_BROWSER:'chromium'},stdio:['ignore','pipe','pipe']
  });
  let output='',interrupted=false;
  child.stdout.on('data',data=>{
    output+=data;
    if(!interrupted&&output.includes('PASS ')){interrupted=true;child.kill('SIGTERM')}
  });
  child.stderr.on('data',data=>output+=data);
  const watchdog=setTimeout(()=>child.kill('SIGKILL'),100000);
  let code;
  try{code=await new Promise((resolve,reject)=>{child.once('exit',resolve);child.once('error',reject)})}
  finally{clearTimeout(watchdog)}
  assert(interrupted,output);
  assert.notEqual(code,0,'an interrupted audit must fail');
  const relative=/UI artifacts: (build\/ui-audit\/chromium\/run-[^\r\n]+)/.exec(output)?.[1];
  assert(relative,output);
  const directory=path.join(root,relative);
  const result=JSON.parse(await readFile(path.join(directory,'results.json'),'utf8'));
  assert(result.results.some(r=>r.status==='failed'),'interruption must be recorded');
  assert.match(await readFile(path.join(directory,'server.log'),'utf8'),/--- PASS: TestUIAuditServer/,'fixture must exit gracefully');
  await assert.rejects(access(path.join(directory,'ready')));
  await assert.rejects(access(path.join(directory,'ready.stop')));
  await assert.rejects(fetch(result.base+'/graph',{signal:AbortSignal.timeout(3000)}),'fixture socket must close');
});
