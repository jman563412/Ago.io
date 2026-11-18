import { defineConfig, devices } from "@playwright/test"

// Its own port, so tests can run while the dev server is up on 8090
const port = 8092

export default defineConfig({
  testDir: "./tests",
  // One server for the whole run, so tests take turns on the same map
  workers: 1,
  fullyParallel: false,
  reporter: "list",
  use: {
    baseURL: `http://localhost:${port}`,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  // Starts the Go server before the tests and stops it after.
  // Runs from the repo root, which the server needs to find ./static
  webServer: {
    command: "go run .",
    url: `http://localhost:${port}`,
    env: { PORT: String(port) },
    reuseExistingServer: false,
    timeout: 60_000,
  },
})
