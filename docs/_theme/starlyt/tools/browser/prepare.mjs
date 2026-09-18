import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { arch, platform, release, tmpdir } from 'node:os';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import revisions from './browser-revisions.json' with { type: 'json' };
import { installations, modes, pages, viewports } from './matrix.mjs';

const browserRoot = dirname(fileURLToPath(import.meta.url));
const repository = resolve(browserRoot, '../..');
const artifacts = join(browserRoot, '.artifacts');
const workspace = join(tmpdir(), 'starlyt-browser-acceptance');
const source = join(workspace, 'consumer');
const output = join(workspace, 'site-output');
const theme = join(source, '_theme', 'starlyt');
const jigyll = process.env.JIGYLL || 'jigyll';

rmSync(artifacts, { recursive: true, force: true });
rmSync(workspace, { recursive: true, force: true });
mkdirSync(artifacts, { recursive: true });
mkdirSync(source, { recursive: true });
cpSync(join(repository, 'fixtures', 'content'), source, { recursive: true });
cpSync(repository, theme, {
  recursive: true,
  filter(path) {
    const name = relative(repository, path).split(sep).join('/');
    return !(
      name === '.git' || name.startsWith('.git/') ||
      name === '.scratch' || name.startsWith('.scratch/') ||
      name === '.poc' || name.startsWith('.poc/') ||
      name === '_site' || name.startsWith('_site/') ||
      name === 'tools/browser/node_modules' || name.startsWith('tools/browser/node_modules/') ||
      name === 'tools/browser/.artifacts' || name.startsWith('tools/browser/.artifacts/')
    );
  },
});

const config = (baseurl) => `title: Starlyt Docs
url: http://127.0.0.1:4444
baseurl: "${baseurl}"
theme: starlyt
permalink: pretty
navigation:
  - label: Overview
    link: /
  - label: Guides
    items:
      - label: Long guide
        link: /guides/long-guide/
  - label: Reference
    items:
      - label: Code examples
        link: /reference/code-examples/
defaults:
  - scope: {path: ""}
    values: {layout: default}
`;

const run = (command, args, options = {}) =>
  execFileSync(command, args, { cwd: repository, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], ...options }).trim();

writeFileSync(join(source, '_config.yml'), config(''));
run(jigyll, ['build', '--source', source, '--destination', output]);
writeFileSync(join(source, '_config.yml'), config('/docs'));
run(jigyll, ['build', '--source', source, '--destination', join(output, 'docs')]);

const themeDiff = execFileSync(
  'git',
  [
    'diff',
    '--binary',
    'HEAD',
    '--',
    '_config.yml',
    '_data',
    '_includes',
    '_layouts',
    '_sass',
    'assets',
  ],
  { cwd: repository },
);
const packageLock = readFileSync(join(browserRoot, 'package-lock.json'));
const environment = {
  generatedAt: new Date().toISOString(),
  scope: 'Pinned Playwright browser binaries on Linux only; excludes Edge, real Safari, mobile devices, and accessibility certification.',
  playwright: revisions.playwright,
  browsers: revisions.browsers,
  platform: { platform: platform(), release: release(), arch: arch() },
  theme: {
    commit: run('git', ['rev-parse', 'HEAD']),
    workingTreeSha256: createHash('sha256').update(themeDiff).digest('hex'),
  },
  jigyll: run(jigyll, ['version']),
  packageLockSha256: createHash('sha256').update(packageLock).digest('hex'),
  installations,
  pages,
  modes,
  viewports,
};
writeFileSync(join(artifacts, 'environment.json'), `${JSON.stringify(environment, null, 2)}\n`);
console.log(`Built root and /docs consumers in ${output}`);
