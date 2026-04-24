// @ts-check
const { defineConfig, devices } = require("@playwright/test");

module.exports = defineConfig({
  testDir: ".",
  timeout: 30_000,
  reporter: "list",
  use: {
    baseURL: process.env.BASE_URL || "http://localhost:5173",
    trace: "off",
    // Disable any HTTP proxy — Chromium otherwise inherits NO_PROXY rules
    // unreliably and routes localhost through whatever proxy is configured,
    // causing 60s timeouts on local dev.
    bypassCSP: true,
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
});
