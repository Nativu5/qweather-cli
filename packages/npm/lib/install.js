'use strict';

const { execFile } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');
const { promisify } = require('node:util');

const { expectedChecksum, verifyFileChecksum } = require('./checksums.js');
const { downloadToFile } = require('./download.js');
const { extractArchive } = require('./extract.js');
const { selectAsset } = require('./platform.js');

const execFileAsync = promisify(execFile);

async function install(options = {}) {
  const packageRoot = options.packageRoot || path.resolve(__dirname, '..');
  const platform = options.platform || process.platform;
  const arch = options.arch || process.arch;
  const download = options.download || downloadToFile;
  const extract = options.extract || extractArchive;
  const packageJSON = JSON.parse(await fs.readFile(path.join(packageRoot, 'package.json'), 'utf8'));
  const version = packageJSON.version;
  const selected = selectAsset(version, platform, arch);
  const libexec = path.join(packageRoot, 'libexec');
  const binaryPath = path.join(libexec, selected.binary);

  if (await hasExpectedVersion(binaryPath, version, platform)) {
    return { binary: binaryPath, reused: true };
  }

  await fs.mkdir(libexec, { recursive: true, mode: 0o755 });
  const temporaryDirectory = await fs.mkdtemp(path.join(libexec, '.qweather-install-'));
  try {
    const archivePath = path.join(temporaryDirectory, selected.asset);
    const manifest = await fs.readFile(path.join(packageRoot, 'checksums.txt'), 'utf8');
    const checksum = expectedChecksum(manifest, selected.asset);
    const url = `https://github.com/Nativu5/qweather-cli/releases/download/v${version}/${selected.asset}`;
    await download(url, archivePath, options.downloadOptions);
    await verifyFileChecksum(archivePath, checksum);
    const extracted = await extract(archivePath, path.join(temporaryDirectory, 'extract'), {
      root: selected.root,
      binary: selected.binary,
      maxBytes: options.maxExtractedBytes,
    });
    if (platform !== 'win32') {
      await fs.chmod(extracted, 0o755);
    }
    if (platform === 'win32') {
      await fs.rm(binaryPath, { force: true });
    }
    await fs.rename(extracted, binaryPath);
    return { binary: binaryPath, reused: false };
  } finally {
    await fs.rm(temporaryDirectory, { recursive: true, force: true });
  }
}

async function hasExpectedVersion(binaryPath, expectedVersion, platform) {
  try {
    const status = await fs.stat(binaryPath);
    if (!status.isFile()) return false;
    if (platform !== 'win32' && (status.mode & 0o111) === 0) return false;
    const result = await execFileAsync(binaryPath, ['version', '--output', 'json'], {
      timeout: 10_000,
      maxBuffer: 1024 * 1024,
      windowsHide: true,
    });
    const parsed = JSON.parse(result.stdout);
    return parsed.version === expectedVersion;
  } catch {
    return false;
  }
}

module.exports = { hasExpectedVersion, install };
