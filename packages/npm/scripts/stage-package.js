#!/usr/bin/env node
'use strict';

const fs = require('node:fs/promises');
const path = require('node:path');

const { parseManifest } = require('../lib/checksums.js');
const { selectAsset } = require('../lib/platform.js');

async function stagePackage(options) {
  const packageRoot = path.resolve(options.packageRoot);
  const repositoryRoot = path.resolve(options.repositoryRoot);
  const output = path.resolve(options.output);
  const checksums = path.resolve(options.checksums);
  try {
    await fs.stat(output);
    throw new Error(`package staging output already exists: ${output}`);
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }

  const version = (await fs.readFile(path.join(repositoryRoot, 'VERSION'), 'utf8')).trim();
  const packageJSON = JSON.parse(await fs.readFile(path.join(packageRoot, 'package.json'), 'utf8'));
  const shrinkwrap = JSON.parse(await fs.readFile(path.join(packageRoot, 'npm-shrinkwrap.json'), 'utf8'));
  if (packageJSON.version !== version || shrinkwrap.version !== version) {
    throw new Error(`version drift: VERSION=${version}, package=${packageJSON.version}, shrinkwrap=${shrinkwrap.version}`);
  }
  const manifestText = await fs.readFile(checksums, 'utf8');
  validateReleaseManifest(parseManifest(manifestText), version);

  await fs.mkdir(output, { recursive: true, mode: 0o755 });
  await fs.cp(path.join(packageRoot, 'bin'), path.join(output, 'bin'), { recursive: true, preserveTimestamps: false });
  await fs.cp(path.join(packageRoot, 'lib'), path.join(output, 'lib'), { recursive: true, preserveTimestamps: false });
  await copyFile(packageRoot, output, 'install.js');
  await copyFile(packageRoot, output, 'package.json');
  await copyFile(packageRoot, output, 'npm-shrinkwrap.json');
  await fs.copyFile(checksums, path.join(output, 'checksums.txt'));
  await fs.copyFile(path.join(repositoryRoot, 'LICENSE'), path.join(output, 'LICENSE'));
  await fs.copyFile(path.join(repositoryRoot, 'README.md'), path.join(output, 'README.md'));
}

function validateReleaseManifest(entries, version) {
  const expected = new Set([
    selectAsset(version, 'darwin', 'arm64').asset,
    selectAsset(version, 'darwin', 'x64').asset,
    selectAsset(version, 'linux', 'arm64').asset,
    selectAsset(version, 'linux', 'x64').asset,
    selectAsset(version, 'win32', 'arm64').asset,
    selectAsset(version, 'win32', 'x64').asset,
  ]);
  if (entries.size !== expected.size) {
    throw new Error(`checksums.txt must contain exactly ${expected.size} release archives`);
  }
  for (const name of expected) {
    if (!entries.has(name)) {
      throw new Error(`checksums.txt is missing ${name}`);
    }
  }
}

async function copyFile(sourceRoot, outputRoot, name) {
  await fs.copyFile(path.join(sourceRoot, name), path.join(outputRoot, name));
}

function parseArguments(arguments_) {
  const values = {};
  for (let index = 0; index < arguments_.length; index += 2) {
    const flag = arguments_[index];
    const value = arguments_[index + 1];
    if (!flag?.startsWith('--') || value === undefined) {
      throw new Error('usage: stage-package --checksums <file> --output <directory>');
    }
    values[flag.slice(2)] = value;
  }
  if (!values.checksums || !values.output) {
    throw new Error('usage: stage-package --checksums <file> --output <directory>');
  }
  return values;
}

if (require.main === module) {
  const packageRoot = path.resolve(__dirname, '..');
  const repositoryRoot = path.resolve(packageRoot, '..', '..');
  Promise.resolve()
    .then(() => parseArguments(process.argv.slice(2)))
    .then((arguments_) => stagePackage({ packageRoot, repositoryRoot, ...arguments_ }))
    .catch((error) => {
      process.stderr.write(`qweather-cli package staging failed: ${error.message}\n`);
      process.exitCode = 1;
    });
}

module.exports = { stagePackage, validateReleaseManifest };
