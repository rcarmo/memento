import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/browser",
  timeout: 45_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  reporter: [["list"], ["json", { outputFile: "build/graph-playwright-report.json" }]],
  use: {
    baseURL: "http://127.0.0.1:18765",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    viewport: { width: 1440, height: 900 },
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "firefox", use: { ...devices["Desktop Firefox"] } },
    { name: "webkit", use: { ...devices["Desktop Safari"] } },
    { name: "tablet", use: { ...devices["iPad Pro 11"] } },
  ],
  webServer: {
    command: "bun tools/graph_fixture_server.ts",
    url: "http://127.0.0.1:18765/graph",
    reuseExistingServer: true,
    timeout: 30_000,
  },
});
