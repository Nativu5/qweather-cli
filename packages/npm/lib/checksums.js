'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');

const CHECKSUM_LINE = /^([0-9a-f]{64})  ([^/\\\r\n]+)$/;

function parseManifest(manifest) {
  if (typeof manifest !== 'string' || !manifest.endsWith('\n')) {
    throw new Error('malformed checksum manifest: expected UTF-8 text with one final newline');
  }
  const lines = manifest.slice(0, -1).split('\n');
  if (lines.length === 0 || lines.some((line) => line.length === 0)) {
    throw new Error('malformed checksum manifest: empty lines are not allowed');
  }
  const entries = new Map();
  let previous = '';
  for (const line of lines) {
    const match = CHECKSUM_LINE.exec(line);
    if (!match) {
      throw new Error('malformed checksum manifest line');
    }
    const [, checksum, name] = match;
    if (entries.has(name)) {
      throw new Error(`duplicate checksum for ${name}`);
    }
    if (previous && name <= previous) {
      throw new Error('malformed checksum manifest: filenames are not bytewise sorted');
    }
    entries.set(name, checksum);
    previous = name;
  }
  return entries;
}

function expectedChecksum(manifest, asset) {
  const checksum = parseManifest(manifest).get(asset);
  if (!checksum) {
    throw new Error(`checksum manifest does not contain ${asset}`);
  }
  return checksum;
}

function verifyChecksum(bytes, expected) {
  if (!/^[0-9a-f]{64}$/.test(expected)) {
    throw new Error('expected checksum is not lowercase SHA256');
  }
  const actual = crypto.createHash('sha256').update(bytes).digest('hex');
  const matches = crypto.timingSafeEqual(Buffer.from(actual, 'ascii'), Buffer.from(expected, 'ascii'));
  if (!matches) {
    throw new Error(`checksum mismatch: expected ${expected}, got ${actual}`);
  }
}

async function verifyFileChecksum(path, expected) {
  const hash = crypto.createHash('sha256');
  const stream = fs.createReadStream(path);
  for await (const chunk of stream) {
    hash.update(chunk);
  }
  const actual = hash.digest('hex');
  const matches = crypto.timingSafeEqual(Buffer.from(actual, 'ascii'), Buffer.from(expected, 'ascii'));
  if (!matches) {
    throw new Error(`checksum mismatch: expected ${expected}, got ${actual}`);
  }
}

module.exports = { expectedChecksum, parseManifest, verifyChecksum, verifyFileChecksum };
