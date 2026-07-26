'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');

const { expectedChecksum, verifyChecksum } = require('../lib/checksums.js');

test('expectedChecksum reads the exact accepted manifest format', () => {
  const manifest = [
    `${'a'.repeat(64)}  qweather-cli_1.2.3_linux_amd64.tar.gz`,
    `${'b'.repeat(64)}  qweather-cli_1.2.3_windows_amd64.zip`,
    '',
  ].join('\n');
  assert.equal(expectedChecksum(manifest, 'qweather-cli_1.2.3_linux_amd64.tar.gz'), 'a'.repeat(64));
});

test('expectedChecksum rejects malformed, duplicate, and missing entries', () => {
  assert.throws(() => expectedChecksum('not-a-manifest\n', 'asset.tar.gz'), /malformed checksum manifest/);
  const duplicate = `${'a'.repeat(64)}  asset.tar.gz\n${'b'.repeat(64)}  asset.tar.gz\n`;
  assert.throws(() => expectedChecksum(duplicate, 'asset.tar.gz'), /duplicate checksum/);
  assert.throws(() => expectedChecksum(`${'a'.repeat(64)}  other.tar.gz\n`, 'asset.tar.gz'), /does not contain/);
});

test('verifyChecksum accepts matching bytes and rejects mismatches', () => {
  const expected = '2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824';
  assert.doesNotThrow(() => verifyChecksum(Buffer.from('hello'), expected));
  assert.throws(() => verifyChecksum(Buffer.from('different'), expected), /checksum mismatch/);
});
