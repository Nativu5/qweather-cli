'use strict';

const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const tar = require('tar');
const test = require('node:test');

const { install } = require('../lib/install.js');

async function createPackageFixture(directory) {
  const packageRoot = path.join(directory, 'package');
  await fs.mkdir(packageRoot, { recursive: true });
  await fs.writeFile(path.join(packageRoot, 'package.json'), JSON.stringify({ name: 'qweather-cli', version: '1.2.3' }));

  const root = 'qweather-cli_1.2.3_linux_amd64';
  const source = path.join(directory, 'archive-source');
  await fs.mkdir(path.join(source, root), { recursive: true });
  const binary = '#!/usr/bin/env node\nconsole.log(JSON.stringify({version:"1.2.3"}));\n';
  await fs.writeFile(path.join(source, root, 'qweather'), binary, { mode: 0o755 });
  await fs.writeFile(path.join(source, root, 'LICENSE'), 'license\n');
  await fs.writeFile(path.join(source, root, 'README.md'), 'readme\n');
  const archive = path.join(directory, 'qweather-cli_1.2.3_linux_amd64.tar.gz');
  await tar.c({ cwd: source, file: archive, gzip: true, portable: true }, [
    `${root}/qweather`,
    `${root}/LICENSE`,
    `${root}/README.md`,
  ]);
  const digest = crypto.createHash('sha256').update(await fs.readFile(archive)).digest('hex');
  await fs.writeFile(path.join(packageRoot, 'checksums.txt'), `${digest}  ${path.basename(archive)}\n`);
  return { archive, packageRoot };
}

test('install downloads, verifies, extracts, and atomically installs the matching binary', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-install-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const fixture = await createPackageFixture(directory);
  const urls = [];

  const result = await install({
    packageRoot: fixture.packageRoot,
    platform: 'linux',
    arch: 'x64',
    async download(url, destination) {
      urls.push(url);
      await fs.copyFile(fixture.archive, destination);
    },
  });

  assert.equal(result.reused, false);
  assert.equal(urls[0], 'https://github.com/Nativu5/qweather-cli/releases/download/v1.2.3/qweather-cli_1.2.3_linux_amd64.tar.gz');
  const binary = path.join(fixture.packageRoot, 'libexec', 'qweather');
  assert.match(await fs.readFile(binary, 'utf8'), /version:"1\.2\.3"/);
});

test('install reuses a valid same-version binary without network I/O', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-install-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const fixture = await createPackageFixture(directory);
  const libexec = path.join(fixture.packageRoot, 'libexec');
  await fs.mkdir(libexec);
  const binary = path.join(libexec, 'qweather');
  await fs.writeFile(binary, '#!/usr/bin/env node\nconsole.log(JSON.stringify({version:"1.2.3"}));\n', { mode: 0o755 });

  const result = await install({
    packageRoot: fixture.packageRoot,
    platform: 'linux',
    arch: 'x64',
    async download() {
      throw new Error('network must not be used');
    },
  });

  assert.deepEqual(result, { binary, reused: true });
});

test('install failure removes temporary files and preserves an existing binary', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-install-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const fixture = await createPackageFixture(directory);
  const libexec = path.join(fixture.packageRoot, 'libexec');
  await fs.mkdir(libexec);
  const binary = path.join(libexec, 'qweather');
  const existing = '#!/usr/bin/env node\nconsole.log(JSON.stringify({version:"0.9.0"}));\n';
  await fs.writeFile(binary, existing, { mode: 0o755 });

  await assert.rejects(install({
    packageRoot: fixture.packageRoot,
    platform: 'linux',
    arch: 'x64',
    async download(_url, destination) {
      await fs.writeFile(destination, 'wrong archive bytes');
    },
  }), /checksum mismatch/);

  assert.equal(await fs.readFile(binary, 'utf8'), existing);
  assert.deepEqual((await fs.readdir(libexec)).sort(), ['qweather']);
});
