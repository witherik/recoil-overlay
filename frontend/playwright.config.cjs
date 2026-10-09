const { defineConfig } = require("@playwright/test");
module.exports = defineConfig({
  testDir: "./tests",
  use: {
    baseURL: "http://127.0.0.1:4178",
    channel: "chrome",
    headless: true,
    viewport: { width: 600, height: 632 }, // the fixed window size, see main.go
  },
  webServer: {
    command: "npm run dev -- --host 127.0.0.1 --port 4178 --strictPort",
    url: "http://127.0.0.1:4178",
    reuseExistingServer: false,
  },
  workers: 1,
});
