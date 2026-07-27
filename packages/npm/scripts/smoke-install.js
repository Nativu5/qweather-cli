#!/usr/bin/env node
'use strict';

const { execFile } = require('node:child_process');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const tar = require('tar');
const { promisify } = require('node:util');

const { selectAsset } = require('../lib/platform.js');
const { fixtureManifest } = require('./fixture-manifest.js');
const { stagePackage } = require('./stage-package.js');

const execFileAsync = promisify(execFile);

async function smokeInstall(binary) {
  if (process.platform === 'win32') {
    throw new Error('the deterministic local-fixture smoke runs on Linux or macOS');
  }
  const packageRoot = path.resolve(__dirname, '..');
  const repositoryRoot = path.resolve(packageRoot, '..', '..');
  const version = (await fs.readFile(path.join(repositoryRoot, 'VERSION'), 'utf8')).trim();
  const selected = selectAsset(version);
  const temporaryRoot = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-npm-smoke-'));
  try {
    const archiveRoot = path.join(temporaryRoot, 'archive-root');
    const payloadRoot = path.join(archiveRoot, selected.root);
    await fs.mkdir(payloadRoot, { recursive: true });
    await fs.copyFile(binary, path.join(payloadRoot, selected.binary));
    await fs.copyFile(path.join(repositoryRoot, 'LICENSE'), path.join(payloadRoot, 'LICENSE'));
    await fs.copyFile(path.join(repositoryRoot, 'README.md'), path.join(payloadRoot, 'README.md'));
    const archive = path.join(temporaryRoot, selected.asset);
    await tar.c({ cwd: archiveRoot, file: archive, gzip: true, portable: true }, [
      `${selected.root}/${selected.binary}`,
      `${selected.root}/LICENSE`,
      `${selected.root}/README.md`,
    ]);

    const checksums = path.join(temporaryRoot, 'checksums.txt');
    const selectedBytes = await fs.readFile(archive);
    await fs.writeFile(checksums, fixtureManifest(version, new Map([[selected.asset, selectedBytes]])));
    const stage = path.join(temporaryRoot, 'stage');
    await stagePackage({ packageRoot, repositoryRoot, output: stage, checksums });
    const packDirectory = path.join(temporaryRoot, 'pack');
    await fs.mkdir(packDirectory);
    const packResult = await execFileAsync('npm', ['pack', stage, '--ignore-scripts', '--json', '--pack-destination', packDirectory], {
      maxBuffer: 4 * 1024 * 1024,
    });
    const tarball = path.join(packDirectory, JSON.parse(packResult.stdout)[0].filename);
    const project = path.join(temporaryRoot, 'project');
    await fs.mkdir(project);
    await fs.writeFile(path.join(project, 'package.json'), '{"private":true}\n');
    await execFileAsync('npm', ['install', '--ignore-scripts', '--no-audit', '--no-fund', tarball], {
      cwd: project,
      maxBuffer: 4 * 1024 * 1024,
    });

    const installedRoot = path.join(project, 'node_modules', 'qweather-cli');
    const installScript = [
      "const fs = require('node:fs/promises');",
      "const { install } = require(process.argv[1]);",
      'install({ packageRoot: process.argv[2], download: (_url, destination) => fs.copyFile(process.argv[3], destination) })',
      "  .catch((error) => { console.error(error.message); process.exitCode = 1; });",
    ].join(' ');
    await execFileAsync(process.execPath, ['-e', installScript, path.join(installedRoot, 'lib', 'install.js'), installedRoot, archive]);
    const shim = path.join(project, 'node_modules', '.bin', process.platform === 'win32' ? 'qweather.cmd' : 'qweather');
    const result = await execFileAsync(shim, ['version', '--output', 'json']);
    const metadata = JSON.parse(result.stdout);
    if (metadata.version !== version || metadata.goVersion !== 'go1.26.5' || !/^[0-9a-f]{40}$/.test(metadata.commit) || !/Z$/.test(metadata.buildTime)) {
      throw new Error(`installed version metadata is invalid: ${result.stdout.trim()}`);
    }
    return metadata;
  } finally {
    await fs.rm(temporaryRoot, { recursive: true, force: true });
  }
}

function parseArguments(arguments_) {
  if (arguments_.length !== 2 || arguments_[0] !== '--binary') {
    throw new Error('usage: smoke-install --binary <qweather>');
  }
  return path.resolve(arguments_[1]);
}

if (require.main === module) {
  Promise.resolve()
    .then(() => parseArguments(process.argv.slice(2)))
    .then(smokeInstall)
    .then((metadata) => process.stdout.write(`installed qweather ${metadata.version} from local npm fixture\n`))
    .catch((error) => {
      process.stderr.write(`qweather-cli npm install smoke failed: ${error.message}\n`);
      process.exitCode = 1;
    });
}

module.exports = { smokeInstall };
