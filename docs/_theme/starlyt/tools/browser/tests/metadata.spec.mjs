import { readFileSync } from 'node:fs';
import { expect, test } from '@playwright/test';
import revisions from '../browser-revisions.json' with { type: 'json' };

const packageMetadata = JSON.parse(
  readFileSync(new URL('../node_modules/@playwright/test/package.json', import.meta.url), 'utf8'),
);
const bundledBrowsers = JSON.parse(
  readFileSync(new URL('../node_modules/playwright-core/browsers.json', import.meta.url), 'utf8'),
).browsers;

test('pins every maintained browser engine', () => {
  expect(packageMetadata.version).toBe(revisions.playwright);
  expect(revisions.browsers.map(({ name }) => name)).toEqual([
    'chromium',
    'firefox',
    'webkit',
  ]);
  for (const browser of revisions.browsers) {
    expect(bundledBrowsers.find(({ name }) => name === browser.name)).toMatchObject({
      revision: browser.revision,
      browserVersion: browser.version,
    });
  }
});
