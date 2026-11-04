import { defineConfig } from '@playwright/test'

import { demo } from './e2e/user.ts'

// The docs' screenshots (make screenshots): e2e/docs-screenshots.shot.ts
// signs in as a demo user of made-up data and saves each screen to
// docs/screenshots. They are not tests: make e2e leaves them, as its
// files end in .spec.ts.
const port = 18_090

export default defineConfig({
  testDir: 'e2e',
  testMatch: '*.shot.ts',
  workers: 1,
  reporter: 'list',
  timeout: 120_000,
  use: {
    baseURL: `http://127.0.0.1:${String(port)}`,
    serviceWorkers: 'block',
    locale: 'en-GB',
    timezoneId: 'Europe/Zagreb',
  },
  webServer: {
    command: `../scripts/screenshots-server.sh ../.e2e/tdeemon ${String(port)} ../.e2e/shots`,
    url: `http://127.0.0.1:${String(port)}/healthz`,
    env: { SHOTS_EMAIL: demo.email, SHOTS_PASSWORD: demo.password },
    reuseExistingServer: false,
  },
})
