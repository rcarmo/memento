import { expect, test, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";

const evidence = "docs/evidence/graph";

test.beforeAll(async () => { await mkdir(evidence, { recursive: true }); });

async function renderedScales(page:Page, ids:string[]) {
  return page.evaluate((wanted)=>{const scene:any=(window as any).__mementoGraphScene;const scales:Record<string,number>={};for(const mesh of scene.meshes){const values=mesh.instanceMatrix?.array;if(!values)continue;mesh.userData.globalIndices.forEach((globalIndex:number,instanceIndex:number)=>{const id=scene.nodes[globalIndex]?.id;if(!wanted.includes(id))return;const offset=instanceIndex*16;scales[id]=Math.hypot(values[offset],values[offset+1],values[offset+2]);});}return scales;},ids);
}

test("loads WebGL graph, filters, selects and exports", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile, "desktop interaction is covered separately from tablet touch");
  const errors:string[]=[]; page.on("pageerror",e=>errors.push(String(e)));
  const started=Date.now(); await page.goto("/graph",{waitUntil:"networkidle"});
  await expect(page.locator("header strong")).toHaveText("Memento Visual Debugger");
  await expect(page.locator("canvas")).toBeVisible();
  if(browserName==="firefox" && await page.locator(".toast-error").count()){
    await expect(page.locator(".toast-error")).toContainText("WebGL2 is unavailable");
    expect(errors).toEqual([]);
    return;
  }
  await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0),{timeout:15000}).toBeGreaterThan(0);
  expect(Date.now()-started).toBeLessThan(5000);
  const spread=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;const points=scene.nodes.map((node:any)=>node.coarse_position);const nearest=points.map((point:any,index:number)=>Math.min(...points.filter((_:any,other:number)=>other!==index).map((other:any)=>Math.hypot(point.x-other.x,point.y-other.y,point.z-other.z)))).sort((a:number,b:number)=>a-b);const xs=points.map((point:any)=>point.x),ys=points.map((point:any)=>point.y);return{width:Math.max(...xs)-Math.min(...xs),height:Math.max(...ys)-Math.min(...ys),median:nearest[Math.floor(nearest.length/2)]};});
  expect(spread.width).toBeGreaterThan(8);
  expect(spread.height).toBeGreaterThan(8);
  expect(spread.median).toBeGreaterThan(1.2);
  await page.locator('input[placeholder*="title"]').fill("Memory 1");
  const canvasBox=await page.locator("canvas").boundingBox();expect(canvasBox).not.toBeNull();
  const center={x:Math.floor(canvasBox!.width/2),y:Math.floor(canvasBox!.height/2)};
  await page.locator("canvas").click({position:center});
  await page.locator('input[placeholder*="title"]').fill("");
  await page.locator("canvas").click({position:center});
  await page.mouse.wheel(0,-200);
  await page.locator("canvas").dragTo(page.locator("canvas"),{sourcePosition:{x:center.x-80,y:center.y-40},targetPosition:{x:center.x+40,y:center.y+20}});
  if(browserName==="chromium"){
    await page.screenshot({path:`${evidence}/overview-light.png`,fullPage:true});
    await page.emulateMedia({colorScheme:"dark"});
    await page.screenshot({path:`${evidence}/overview-dark.png`,fullPage:true});
  }
  expect(errors).toEqual([]);
});

test("uses available size metrics to produce visibly distinct rendered scales", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile || browserName!=="chromium","rendered metric sizing is asserted once in Chromium");
  await page.goto("/graph",{waitUntil:"networkidle"});
  await page.waitForFunction(()=>Boolean((window as any).__mementoGraphScene?.nodes?.length));
  const selector=page.locator('label:has-text("Size") select');
  await expect(selector.locator("option")).toHaveText(["combined_bytes","markdown_bytes","asset_bytes","explicit_in_degree","explicit_out_degree"]);
  const combined=await renderedScales(page,["node-0","node-1","node-8"]);
  await selector.selectOption("markdown_bytes");
  await expect.poll(()=>page.evaluate(()=>(window as any).__mementoGraphScene.settings.sizeMetric)).toBe("markdown_bytes");
  const markdown=await renderedScales(page,["node-0"]);
  expect(combined["node-0"]-markdown["node-0"]).toBeGreaterThan(.4);
  await selector.selectOption("asset_bytes");
  await expect.poll(()=>page.evaluate(()=>(window as any).__mementoGraphScene.settings.sizeMetric)).toBe("asset_bytes");
  const asset=await renderedScales(page,["node-0","node-1"]);
  expect(asset["node-0"]-asset["node-1"]).toBeGreaterThan(.5);
  await selector.selectOption("explicit_in_degree");
  await expect.poll(()=>page.evaluate(()=>(window as any).__mementoGraphScene.settings.sizeMetric)).toBe("explicit_in_degree");
  const degree=await renderedScales(page,["node-0","node-8"]);
  expect(degree["node-8"]-degree["node-0"]).toBeGreaterThan(.5);
  await expect(page.locator('label:has-text("Size") .selection-detail')).toContainText("Relative logarithmic scale");
});

test("offers member count only while viewing aggregate nodes", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile || browserName!=="chromium","aggregate metric availability is asserted once in Chromium");
  await page.route("**/graph/api/v1/overview",async route=>{const response=await route.fetch();const payload=await response.json();const clusters=[
    {id:"cluster-small",label:"Small namespace",namespace:"/small/",member_count:1,markdown_bytes:100,asset_bytes:0,combined_bytes:100,explicit_in_degree:1,explicit_out_degree:0,type_counts:[["project",1]],coarse_position:{x:-3,y:0,z:0}},
    {id:"cluster-large",label:"Large namespace",namespace:"/large/",member_count:100,markdown_bytes:10000,asset_bytes:1000,combined_bytes:11000,explicit_in_degree:8,explicit_out_degree:9,type_counts:[["project",100]],coarse_position:{x:3,y:0,z:0}},
  ];await route.fulfill({response,json:{...payload,mode:"aggregated",nodes:[],edges:[],clusters,cluster_edges:[],truncated:true}});});
  await page.goto("/graph",{waitUntil:"networkidle"});
  const selector=page.locator('label:has-text("Size") select');
  await expect(selector.locator('option[value="member_count"]')).toHaveCount(1);
  await selector.selectOption("member_count");
  await expect.poll(()=>page.evaluate(()=>(window as any).__mementoGraphScene.settings.sizeMetric)).toBe("member_count");
  const aggregate=await renderedScales(page,["cluster-small","cluster-large"]);
  expect(aggregate["cluster-large"]-aggregate["cluster-small"]).toBeGreaterThan(.5);
  await page.evaluate(()=>(window as any).__mementoGraphScene.callbacks.select((window as any).__mementoGraphScene.nodes[0]));
  await expect(selector.locator('option[value="member_count"]')).toHaveCount(0);
  await expect(selector).toHaveValue("combined_bytes");
  await expect.poll(()=>page.evaluate(()=>(window as any).__mementoGraphScene.settings.sizeMetric)).toBe("combined_bytes");
});

test("keeps picking bounds synchronized after worker layout", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile || browserName!=="chromium","instanced-mesh picking is asserted once in Chromium");
  await page.goto("/graph",{waitUntil:"domcontentloaded"});
  await page.waitForFunction(()=>Boolean((window as any).__mementoGraphScene?.nodes?.length));
  await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;const box=scene.canvas.getBoundingClientRect();scene.hit({clientX:box.left+box.width/2,clientY:box.top+box.height/2});});
  await page.waitForTimeout(2500);
  const result=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;const Vector3=scene.camera.position.constructor;const outside:any[]=[];for(const mesh of scene.meshes){const sphere=mesh.boundingSphere;for(const globalIndex of mesh.userData.globalIndices){const node=scene.nodes[globalIndex],p=node.coarse_position,point=new Vector3(p.x,p.y,p.z);if(!sphere||point.distanceTo(sphere.center)>sphere.radius+scene.radius(node))outside.push({id:node.id,title:node.title});}}return{outside,nodes:scene.nodes.length};});
  expect(result.nodes).toBeGreaterThan(0);
  expect(result.outside).toEqual([]);
});

test("highlights selected inbound and outbound links directionally", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile || browserName!=="chromium","selected edge rendering is asserted once in Chromium");
  await page.goto("/graph",{waitUntil:"networkidle"});
  await page.waitForFunction(()=>Boolean((window as any).__mementoGraphScene?.nodes?.length));
  const overlay=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;const incoming=new Set(scene.edges.map((edge:any)=>edge.target)),outgoing=new Set(scene.edges.map((edge:any)=>edge.source));const node=scene.nodes.find((candidate:any)=>incoming.has(candidate.id)&&outgoing.has(candidate.id));if(!node)return null;scene.focus(node);const first=scene.selectedEdgeGroup.children[0],geometry=first?.geometry,material=first?.material;scene.focus(node);return{selected:node.id,incoming:scene.selectedEdgeGroup.userData.incoming,outgoing:scene.selectedEdgeGroup.userData.outgoing,replaced:Boolean(first&&scene.selectedEdgeGroup.children[0]!==first),disposed:Boolean(geometry?.index===null&&material?.program===undefined),children:scene.selectedEdgeGroup.children.map((mesh:any)=>({direction:mesh.userData.direction,color:mesh.material.color.getHex(),radius:mesh.geometry.parameters.radius}))};});
  expect(overlay).not.toBeNull();
  expect(overlay!.incoming).toBeGreaterThan(0);
  expect(overlay!.outgoing).toBeGreaterThan(0);
  expect(overlay!.replaced).toBe(true);
  expect(overlay!.children.filter((edge:any)=>edge.direction==="incoming").every((edge:any)=>edge.color===0x55aee8&&edge.radius>.01)).toBe(true);
  expect(overlay!.children.filter((edge:any)=>edge.direction==="outgoing").every((edge:any)=>edge.color===0xe66b5b&&edge.radius>.01)).toBe(true);
  await expect(page.locator(".legend")).toContainText("selected incoming");
  await expect(page.locator(".legend")).toContainText("selected outgoing");
});

test("transient embedding diagnostics do not ring every node", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile || browserName!=="chromium","halo filtering is asserted once in Chromium");
  await page.goto("/graph",{waitUntil:"networkidle"});
  await page.waitForFunction(()=>Boolean((window as any).__mementoGraphScene?.nodes?.length));
  const halos=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;scene.selectedId=null;for(const node of scene.nodes)node.anomaly_ids=[];scene.nodes[0].anomaly_ids=[`diagnostic:embedding_stale:${scene.nodes[0].id}`];scene.nodes[1].anomaly_ids=[`diagnostic:embedding_missing:${scene.nodes[1].id}`];scene.nodes[2].anomaly_ids=[`diagnostic:orphan:${scene.nodes[2].id}`];scene.nodes[3].anomaly_ids=[`diagnostic:size_outlier:${scene.nodes[3].id}`];scene.drawHalos();return scene.haloGroup.children.length;});
  expect(halos).toBe(1);
});

test("semantic layer is default-off and selected nodes reveal bounded neighbours", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile || browserName!=="chromium","semantic controls are asserted once in Chromium");
  await page.goto("/graph",{waitUntil:"networkidle"});
  await page.waitForFunction(()=>Boolean((window as any).__mementoGraphScene?.nodes?.length));
  const initial=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;return scene.edges.filter((edge:any)=>edge.kind==="semantic_similarity").length;});
  expect(initial).toBe(0);
  await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;scene.callbacks.select(scene.nodes[7]);});
  await expect.poll(async()=>page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;return scene.edges.filter((edge:any)=>edge.kind==="semantic_similarity").length;})).toBeGreaterThan(0);
  const selectedCount=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;return scene.edges.filter((edge:any)=>edge.kind==="semantic_similarity").length;});
  expect(selectedCount).toBeLessThanOrEqual(5);
  const selectedRendering=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;return{semanticTubes:scene.selectedEdgeGroup.userData.semantic||0,dashedColours:scene.edgeGroup.children.filter((child:any)=>child.material?.type==="LineDashedMaterial").map((child:any)=>child.material.color.getHex()),selectedColours:scene.selectedEdgeGroup.children.filter((child:any)=>child.userData.direction==="semantic").map((child:any)=>child.material.color.getHex())};});
  expect(selectedRendering.semanticTubes).toBeGreaterThan(0);
  expect(selectedRendering.dashedColours.length).toBeGreaterThan(0);
  expect(selectedRendering.dashedColours.every((colour:number)=>[0x3f9f68,0x50b878,0x60c870].includes(colour))).toBe(true);
  expect(selectedRendering.selectedColours.every((colour:number)=>colour===0x60c870)).toBe(true);
  await page.locator('label:has-text("Show semantic layer") input').check();
  await expect.poll(async()=>page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;return scene.edges.filter((edge:any)=>edge.kind==="semantic_similarity").length;})).toBeGreaterThan(selectedCount);
  await expect(page.locator(".inspector")).toContainText("Semantic neighbours");
  await expect(page.locator(".inspector")).toContainText("cosine");
});

test("focuses smoothly without rotating and reveals FTS search results", async ({ page, browserName,isMobile }) => {
  test.skip(isMobile || browserName!=="chromium","camera interpolation is asserted once in Chromium");
  await page.goto("/graph",{waitUntil:"networkidle"});
  await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0),{timeout:15000}).toBeGreaterThan(0);
  const focus=await page.evaluate(async()=>{const scene:any=(window as any).__mementoGraphScene;const node=scene.nodes[20];scene.yaw=1.2;scene.pitch=.3;const before={target:scene.target.toArray(),yaw:scene.yaw,pitch:scene.pitch};scene.focus(node);const immediate={target:scene.target.toArray(),active:!!scene.focusTween};await new Promise(resolve=>setTimeout(resolve,260));const middle={target:scene.target.toArray(),active:!!scene.focusTween};await new Promise(resolve=>setTimeout(resolve,400));const end={target:scene.target.toArray(),yaw:scene.yaw,pitch:scene.pitch,active:!!scene.focusTween,position:[node.coarse_position.x,node.coarse_position.y,node.coarse_position.z]};return{before,immediate,middle,end};});
  expect(focus.immediate.target).toEqual(focus.before.target);
  expect(focus.middle.target).not.toEqual(focus.before.target);
  expect(focus.end.target).toEqual(focus.end.position);
  expect(focus.end.yaw).toBe(focus.before.yaw);
  expect(focus.end.pitch).toBe(focus.before.pitch);
  expect(focus.end.active).toBe(false);
  await page.locator('.search-control input').fill("bounded Markdown preview");
  await expect(page.locator('.search-results button')).toHaveCount(20);
  await page.locator('.search-results button').nth(10).click();
  await expect(page.locator('.search-control input')).toHaveValue("");
  await expect(page.locator('.inspector')).toContainText("Memory");
  const revealed=await page.evaluate(()=>{const scene:any=(window as any).__mementoGraphScene;return{nodes:scene.nodes.length,selectedId:scene.selectedId};});
  expect(revealed.nodes).toBeGreaterThan(0);
  expect(revealed.selectedId).toBeTruthy();
});

test("simulates principal namespace visibility without becoming an auth boundary", async ({ page, browserName, isMobile }) => {
  test.skip(isMobile||browserName!=="chromium","principal simulation is asserted once in Chromium");
  const overviewHeaders:string[]=[];
  page.on("request",request=>{if(request.url().endsWith("/graph/api/v1/overview"))overviewHeaders.push(request.headers()["x-memento-simulated-principal"]||"");});
  await page.goto("/graph",{waitUntil:"networkidle"});
  const selector=page.locator('label:has-text("View as") select');
  await expect(selector).toHaveValue("");
  await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0)).toBe(120);
  await selector.selectOption("projects-reader");
  await expect(page.locator("header .warning")).toHaveText("Simulated visibility — not an authorization boundary");
  await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0)).toBe(20);
  expect(overviewHeaders.at(-1)).toBe("projects-reader");
  await page.locator('.search-control input').fill("bounded Markdown preview");
  await expect(page.locator('.search-results button')).toHaveCount(20);
  const paths=await page.locator('.search-results button code').allTextContents();
  expect(paths.every(path=>path.startsWith("/projects/"))).toBe(true);
  await selector.selectOption("shared-reader");
  await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0)).toBe(40);
  await selector.selectOption("");
  await expect(page.locator("header .warning")).toHaveText("Unauthenticated -- trusted networks only");
  await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0)).toBe(120);
  expect(overviewHeaders.at(-1)).toBe("");
});

test("tablet touch layout remains usable", async ({ page, isMobile }) => {
  test.skip(!isMobile,"tablet project only");
  await page.goto("/graph",{waitUntil:"networkidle"});
  await expect(page.locator("canvas")).toBeVisible();
  await expect(page.locator(".controls")).toBeVisible();
  await page.touchscreen.tap(600,400);
  await page.screenshot({path:`${evidence}/tablet-touch.png`,fullPage:true});
});
