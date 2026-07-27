'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');

const { fixtureManifest } = require('../scripts/fixture-manifest.js');
const { stagePackage } = require('../scripts/stage-package.js');

const packageRoot = path.resolve(__dirname, '..');
const repositoryRoot = path.resolve(packageRoot, '..', '..');

test('npm metadata stays synchronized with the root version and fixed toolchain contract', async () => {
  const version = (await fs.readFile(path.join(repositoryRoot, 'VERSION'), 'utf8')).trim();
  const packageJSON = JSON.parse(await fs.readFile(path.join(packageRoot, 'package.json'), 'utf8'));
  const shrinkwrap = JSON.parse(await fs.readFile(path.join(packageRoot, 'npm-shrinkwrap.json'), 'utf8'));

  assert.equal(packageJSON.version, version);
  assert.equal(shrinkwrap.version, version);
  assert.equal(packageJSON.engines.node, '>=22.11.0');
  assert.equal(packageJSON.packageManager, 'npm@11.16.0');
  assert.deepEqual(packageJSON.dependencies, {
    tar: '7.5.22',
    undici: '7.29.0',
    yauzl: '3.4.0',
  });
  assert.deepEqual(shrinkwrap.packages[''].dependencies, packageJSON.dependencies);
  assert.equal(shrinkwrap.packages[''].engines.node, packageJSON.engines.node);
  assert.ok(packageJSON.files.includes('npm-shrinkwrap.json'));
});

test('stagePackage produces the allowlisted publication tree with the embedded manifest', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-package-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const output = path.join(directory, 'stage');
  const version = (await fs.readFile(path.join(repositoryRoot, 'VERSION'), 'utf8')).trim();
  const checksums = path.join(directory, 'checksums.txt');
  await fs.writeFile(checksums, fixtureManifest(version));
  await stagePackage({
    packageRoot,
    repositoryRoot,
    output,
    checksums,
  });

  const files = await listFiles(output);
  const expected = [
    'LICENSE',
    'README.md',
    'bin/qweather.js',
    'checksums.txt',
    'install.js',
    'lib/checksums.js',
    'lib/download.js',
    'lib/extract.js',
    'lib/install.js',
    'lib/platform.js',
    'npm-shrinkwrap.json',
    'package.json',
  ];
  assert.deepEqual(files.sort(), expected.sort());
});

async function listFiles(root) {
  const result = [];
  async function visit(directory) {
    const entries = await fs.readdir(directory, { withFileTypes: true });
    entries.sort((left, right) => left.name.localeCompare(right.name));
    for (const entry of entries) {
      const fullPath = path.join(directory, entry.name);
      if (entry.isDirectory()) await visit(fullPath);
      else result.push(path.relative(root, fullPath).split(path.sep).join('/'));
    }
  }
  await visit(root);
  return result;
}
