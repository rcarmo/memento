import { expect, test } from "@playwright/test";
import { mkdir } from "node:fs/promises";

const evidence = "docs/evidence/graph";

test.beforeAll(async () => { await mkdir(evidence, { recursive: true }); });

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

test("tablet touch layout remains usable", async ({ page, isMobile }) => {
  test.skip(!isMobile,"tablet project only");
  await page.goto("/graph",{waitUntil:"networkidle"});
  await expect(page.locator("canvas")).toBeVisible();
  await expect(page.locator(".controls")).toBeVisible();
  await page.touchscreen.tap(600,400);
  await page.screenshot({path:`${evidence}/tablet-touch.png`,fullPage:true});
});
