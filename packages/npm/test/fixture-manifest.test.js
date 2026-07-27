'use strict';

const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const test = require('node:test');

const { parseManifest } = require('../lib/checksums.js');
const { fixtureManifest } = require('../scripts/fixture-manifest.js');

test('fixtureManifest creates the exact versioned asset set and accepts byte overrides', () => {
  const version = '9.8.7';
  const overriddenAsset = `qweather-cli_${version}_linux_amd64.tar.gz`;
  const overriddenBytes = Buffer.from('release archive');
  const entries = parseManifest(fixtureManifest(version, new Map([[overriddenAsset, overriddenBytes]])));

  assert.deepEqual([...entries.keys()], [
    `qweather-cli_${version}_darwin_amd64.tar.gz`,
    `qweather-cli_${version}_darwin_arm64.tar.gz`,
    `qweather-cli_${version}_linux_amd64.tar.gz`,
    `qweather-cli_${version}_linux_arm64.tar.gz`,
    `qweather-cli_${version}_windows_amd64.zip`,
    `qweather-cli_${version}_windows_arm64.zip`,
  ]);
  assert.equal(
    entries.get(overriddenAsset),
    crypto.createHash('sha256').update(overriddenBytes).digest('hex'),
  );
  assert.equal(fixtureManifest(version), fixtureManifest(version));
});
