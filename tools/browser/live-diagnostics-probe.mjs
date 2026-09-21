import {chromium} from 'playwright';
import fs from 'node:fs/promises';
const out='/workspace/tmp/memento-link-audit';
const browser=await chromium.launch({headless:true,args:['--no-sandbox','--use-gl=angle','--use-angle=swiftshader','--enable-webgl','--disable-web-security']});
try {
 const page=await browser.newPage({viewport:{width:1500,height:1050}});
 const events=[];page.on('pageerror',e=>events.push({type:'js',message:e.message}));page.on('response',async r=>{if(r.url().includes('/api/v1/memories/'))events.push({type:'detail',status:r.status(),url:r.url(),body:await r.text()})});
 await page.goto('http://192.168.1.250:18081/graph',{waitUntil:'domcontentloaded',timeout:60000});
 await page.waitForFunction(()=>window.__mementoGraphScene?.nodes?.length>0,{timeout:60000});
 await page.getByPlaceholder('title, tags, full text').fill('Website Device Screenshots');
 await page.locator('.search-results button').filter({hasText:'/skills/blogging/website-device-screenshots.md'}).click({timeout:30000});
 await page.waitForTimeout(7000);
 await fs.writeFile(out+'/browser-state.txt',await page.locator('body').innerText());
 await fs.writeFile(out+'/browser-events.json',JSON.stringify(events,null,2));
 await page.screenshot({path:out+'/browser-selected.png'});
 console.log((await page.locator('.inspector').innerText()).slice(0,1800));
 console.log(JSON.stringify({events:events.map(e=>({...e,body:e.body?.slice(0,120)})),diagnostics:await page.locator('.diagnostics li').count(),selection:await page.evaluate(()=>window.__mementoGraphScene?.selectedId)}));
} finally{await browser.close()}
