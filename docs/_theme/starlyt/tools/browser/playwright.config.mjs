import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  outputDir: '.artifacts/test-results',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 30_000,
  expect: { timeout: 5_000 },
  reporter: [
    ['line'],
    ['json', { outputFile: '.artifacts/results.json' }],
  ],
  use: {
    baseURL: 'http://127.0.0.1:4444',
    locale: 'en-US',
    timezoneId: 'UTC',
    deviceScaleFactor: 1,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: {
    url: 'http://127.0.0.1:4444/',
    command: `python -m http.server 4444 --bind 127.0.0.1 --directory ${JSON.stringify(join(tmpdir(), 'starlyt-browser-acceptance', 'site-output'))}`,
    timeout: 30_000,
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
    { name: 'firefox', use: { browserName: 'firefox' } },
    { name: 'webkit', use: { browserName: 'webkit' } },
  ],
});
