#!/usr/bin/env node
'use strict';

const crypto = require('node:crypto');
const { execFile } = require('node:child_process');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { promisify } = require('node:util');

const { fixtureManifest } = require('./fixture-manifest.js');
const { stagePackage } = require('./stage-package.js');

const execFileAsync = promisify(execFile);
const expectedFiles = [
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
].sort();

async function checkPack() {
  const packageRoot = path.resolve(__dirname, '..');
  const repositoryRoot = path.resolve(packageRoot, '..', '..');
  const npmVersion = (await execFileAsync('npm', ['--version'])).stdout.trim();
  if (npmVersion !== '11.16.0') {
    throw new Error(`npm pack reproducibility requires npm 11.16.0, got ${npmVersion}`);
  }
  const temporaryRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-npm-pack-'));
  try {
    const version = (await fs.readFile(path.join(repositoryRoot, 'VERSION'), 'utf8')).trim();
    const checksums = path.join(temporaryRoot, 'checksums.txt');
    await fs.writeFile(checksums, fixtureManifest(version));
    const outputs = [];
    for (const name of ['first', 'second']) {
      const stage = path.join(temporaryRoot, `${name}-stage`);
      const destination = path.join(temporaryRoot, `${name}-pack`);
      await fs.mkdir(destination);
      await stagePackage({ packageRoot, repositoryRoot, output: stage, checksums });
      const result = await execFileAsync('npm', ['pack', stage, '--ignore-scripts', '--json', '--pack-destination', destination], {
        maxBuffer: 4 * 1024 * 1024,
      });
      const metadata = JSON.parse(result.stdout)[0];
      const files = metadata.files.map((entry) => entry.path).sort();
      if (JSON.stringify(files) !== JSON.stringify(expectedFiles)) {
        throw new Error(`npm pack contents differ from allowlist: ${JSON.stringify(files)}`);
      }
      outputs.push(await fs.readFile(path.join(destination, metadata.filename)));
    }
    const firstDigest = crypto.createHash('sha256').update(outputs[0]).digest('hex');
    const secondDigest = crypto.createHash('sha256').update(outputs[1]).digest('hex');
    if (firstDigest !== secondDigest) {
      throw new Error(`npm tarball is not reproducible: ${firstDigest} != ${secondDigest}`);
    }
    return firstDigest;
  } finally {
    await fs.rm(temporaryRoot, { recursive: true, force: true });
  }
}

function parseArguments(arguments_) {
  if (arguments_.length !== 0) {
    throw new Error('usage: check-pack');
  }
}

if (require.main === module) {
  Promise.resolve()
    .then(() => parseArguments(process.argv.slice(2)))
    .then(checkPack)
    .then((digest) => process.stdout.write(`reproducible npm tarball sha256 ${digest}\n`))
    .catch((error) => {
      process.stderr.write(`qweather-cli npm pack check failed: ${error.message}\n`);
      process.exitCode = 1;
    });
}

module.exports = { checkPack };
