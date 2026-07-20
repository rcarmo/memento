import { expect, test } from "@playwright/test";
import { mkdir } from "node:fs/promises";

const evidence="docs/evidence/graph";
test.beforeAll(async()=>mkdir(evidence,{recursive:true}));

test("selected memory inspector screenshot",async({page,browserName,isMobile})=>{
 test.skip(browserName!=="chromium"||isMobile,"screenshot baseline");
 await page.goto("/graph",{waitUntil:"networkidle"});
 await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0)).toBeGreaterThan(0);
 await page.evaluate(()=>{const scene=(window as any).__mementoGraphScene;scene.callbacks.select(scene.nodes[0]);});
 await expect(page.locator(".inspector h2")).toBeVisible();
 await page.screenshot({path:`${evidence}/selected-inspector.png`,fullPage:true});
});

test("aggregated overview and cluster expansion screenshots",async({page,browserName,isMobile})=>{
 test.skip(browserName!=="chromium"||isMobile,"screenshot baseline");
 await page.route("**/graph/api/v1/overview",async route=>{const response=await route.fetch();const payload=await response.json();const clusters=Array.from({length:12},(_,i)=>({id:`cluster-${i}`,label:`Namespace ${i}`,namespace:`/namespace-${i}/`,member_count:834,markdown_bytes:250000,asset_bytes:50000,combined_bytes:300000,explicit_in_degree:20,explicit_out_degree:20,broken_link_count:i===0?2:0,orphan_count:0,type_counts:[["project",10]],status_counts:[["active",10]],coarse_position:{x:Math.cos(i/12*Math.PI*2)*6,y:Math.sin(i/12*Math.PI*2)*6,z:(i%3)-1}}));const cluster_edges=clusters.map((c,i)=>({id:`ce-${i}`,source:c.id,target:clusters[(i+1)%clusters.length].id,explicit_edge_count:5,canonical:true}));await route.fulfill({json:{...payload,mode:"aggregated",nodes:[],edges:[],clusters,cluster_edges,memberships:[],truncated:true,metrics:{...payload.metrics,memory_count:10000}}});});
 await page.goto("/graph",{waitUntil:"networkidle"});
 await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0)).toBe(12);
 await page.screenshot({path:`${evidence}/aggregated-overview.png`,fullPage:true});
 await page.evaluate(()=>{const scene=(window as any).__mementoGraphScene;scene.callbacks.select(scene.nodes[0]);});
 await expect.poll(async()=>Number((await page.locator(".perf dd").first().textContent())||0)).toBeGreaterThan(12);
 await page.screenshot({path:`${evidence}/expanded-cluster.png`,fullPage:true});
});
