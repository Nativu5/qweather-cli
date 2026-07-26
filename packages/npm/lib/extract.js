'use strict';

const fs = require('node:fs');
const fsPromises = require('node:fs/promises');
const path = require('node:path');
const { pipeline } = require('node:stream/promises');
const tar = require('tar');
const yauzl = require('yauzl');

const DEFAULT_MAX_EXTRACTED_BYTES = 128 * 1024 * 1024;

async function extractArchive(archivePath, destination, options) {
  const { root, binary } = options;
  const maxBytes = options.maxBytes || DEFAULT_MAX_EXTRACTED_BYTES;
  await fsPromises.mkdir(destination, { recursive: true, mode: 0o700 });
  if (archivePath.endsWith('.zip')) {
    const entries = await inspectZip(archivePath);
    validateEntries(entries, root, binary, maxBytes);
    await extractZip(archivePath, destination);
  } else if (archivePath.endsWith('.tar.gz')) {
    const entries = await inspectTar(archivePath);
    validateEntries(entries, root, binary, maxBytes);
    await tar.x({
      cwd: destination,
      file: archivePath,
      preservePaths: false,
      strict: true,
    });
  } else {
    throw new Error(`unsupported archive format for ${archivePath}`);
  }
  const binaryPath = path.join(destination, root, binary);
  const status = await fsPromises.lstat(binaryPath);
  if (!status.isFile()) {
    throw new Error('extracted qweather binary is not a regular file');
  }
  if (process.platform !== 'win32') {
    await fsPromises.chmod(binaryPath, 0o755);
  }
  return binaryPath;
}

async function inspectTar(archivePath) {
  const entries = [];
  await tar.t({
    file: archivePath,
    onReadEntry(entry) {
      entries.push({ name: entry.path, size: entry.size, type: entry.type });
    },
  });
  return entries;
}

function inspectZip(archivePath) {
  return new Promise((resolve, reject) => {
    yauzl.open(archivePath, { lazyEntries: true, validateEntrySizes: true }, (openError, zipFile) => {
      if (openError) {
        reject(openError);
        return;
      }
      const entries = [];
      zipFile.once('error', reject);
      zipFile.on('entry', (entry) => {
        const unixMode = (entry.externalFileAttributes >>> 16) & 0xffff;
        const fileType = unixMode & 0o170000;
        entries.push({
          name: entry.fileName,
          size: entry.uncompressedSize,
          type: fileType === 0o120000 ? 'SymbolicLink' : 'File',
        });
        zipFile.readEntry();
      });
      zipFile.once('end', () => resolve(entries));
      zipFile.readEntry();
    });
  });
}

function validateEntries(entries, root, binary, maxBytes) {
  const expected = new Set([
    `${root}/${binary}`,
    `${root}/LICENSE`,
    `${root}/README.md`,
  ]);
  if (entries.length !== expected.size) {
    throw new Error(`archive must contain exactly three files; found ${entries.length}`);
  }
  let total = 0;
  const seen = new Set();
  for (const entry of entries) {
    validateEntryName(entry.name);
    if (!expected.has(entry.name)) {
      throw new Error(`unexpected archive entry ${entry.name}`);
    }
    if (seen.has(entry.name)) {
      throw new Error(`duplicate archive entry ${entry.name}`);
    }
    if (entry.type !== 'File' && entry.type !== 'OldFile' && entry.type !== 'ContiguousFile') {
      throw new Error(`archive entry ${entry.name} is not a regular file`);
    }
    if (!Number.isSafeInteger(entry.size) || entry.size < 0) {
      throw new Error(`archive entry ${entry.name} has an invalid size`);
    }
    total += entry.size;
    if (total > maxBytes) {
      throw new Error(`archive exceeds the ${maxBytes}-byte extracted size limit`);
    }
    seen.add(entry.name);
  }
  for (const name of expected) {
    if (!seen.has(name)) {
      throw new Error(`archive is missing ${name}`);
    }
  }
}

function validateEntryName(name) {
  if (!name || name.includes('\0') || name.includes('\\') || name.startsWith('/')) {
    throw new Error(`unsafe archive entry ${JSON.stringify(name)}`);
  }
  const segments = name.split('/');
  if (segments.some((segment) => segment === '' || segment === '.' || segment === '..')) {
    throw new Error(`unsafe archive entry ${JSON.stringify(name)}`);
  }
}

function extractZip(archivePath, destination) {
  return new Promise((resolve, reject) => {
    yauzl.open(archivePath, { lazyEntries: true, validateEntrySizes: true }, (openError, zipFile) => {
      if (openError) {
        reject(openError);
        return;
      }
      let failed = false;
      const fail = (error) => {
        if (failed) return;
        failed = true;
        zipFile.close();
        reject(error);
      };
      zipFile.once('error', fail);
      zipFile.on('entry', async (entry) => {
        try {
          const target = path.join(destination, ...entry.fileName.split('/'));
          await fsPromises.mkdir(path.dirname(target), { recursive: true, mode: 0o700 });
          const input = await openZipReadStream(zipFile, entry);
          const output = fs.createWriteStream(target, { flags: 'wx', mode: 0o600 });
          await pipeline(input, output);
          zipFile.readEntry();
        } catch (error) {
          fail(error);
        }
      });
      zipFile.once('end', () => {
        if (!failed) resolve();
      });
      zipFile.readEntry();
    });
  });
}

function openZipReadStream(zipFile, entry) {
  return new Promise((resolve, reject) => {
    zipFile.openReadStream(entry, (error, stream) => {
      if (error) reject(error);
      else resolve(stream);
    });
  });
}

module.exports = { extractArchive, validateEntries };
