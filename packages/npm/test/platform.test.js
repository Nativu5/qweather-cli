'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');

const { selectAsset } = require('../lib/platform.js');

test('selectAsset maps the six supported targets to exact release names', () => {
  const expected = [
    ['darwin', 'arm64', 'qweather-cli_1.2.3_darwin_arm64.tar.gz'],
    ['darwin', 'x64', 'qweather-cli_1.2.3_darwin_amd64.tar.gz'],
    ['linux', 'arm64', 'qweather-cli_1.2.3_linux_arm64.tar.gz'],
    ['linux', 'x64', 'qweather-cli_1.2.3_linux_amd64.tar.gz'],
    ['win32', 'arm64', 'qweather-cli_1.2.3_windows_arm64.zip'],
    ['win32', 'x64', 'qweather-cli_1.2.3_windows_amd64.zip'],
  ];

  for (const [platform, arch, asset] of expected) {
    assert.deepEqual(selectAsset('1.2.3', platform, arch), {
      asset,
      binary: platform === 'win32' ? 'qweather.exe' : 'qweather',
      root: asset.replace(/\.(?:tar\.gz|zip)$/, ''),
    });
  }
});

test('selectAsset rejects unsupported platforms without a source-build fallback', () => {
  assert.throws(() => selectAsset('1.2.3', 'freebsd', 'x64'), /unsupported platform freebsd\/x64/);
  assert.throws(() => selectAsset('1.2.3', 'linux', 'riscv64'), /unsupported platform linux\/riscv64/);
});
