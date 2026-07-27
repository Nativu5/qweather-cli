'use strict';

const crypto = require('node:crypto');

const { selectAsset } = require('../lib/platform.js');

function fixtureManifest(version, bytesByAsset = new Map()) {
  const names = [
    selectAsset(version, 'darwin', 'arm64').asset,
    selectAsset(version, 'darwin', 'x64').asset,
    selectAsset(version, 'linux', 'arm64').asset,
    selectAsset(version, 'linux', 'x64').asset,
    selectAsset(version, 'win32', 'arm64').asset,
    selectAsset(version, 'win32', 'x64').asset,
  ].sort();
  return names.map((name) => {
    const bytes = bytesByAsset.has(name)
      ? bytesByAsset.get(name)
      : Buffer.from(`local-fixture-only:${name}`);
    return `${crypto.createHash('sha256').update(bytes).digest('hex')}  ${name}\n`;
  }).join('');
}

module.exports = { fixtureManifest };
