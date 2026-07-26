'use strict';

const PLATFORM_NAMES = Object.freeze({
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows',
});

const ARCH_NAMES = Object.freeze({
  arm64: 'arm64',
  x64: 'amd64',
});

function selectAsset(version, platform = process.platform, arch = process.arch) {
  const goos = PLATFORM_NAMES[platform];
  const goarch = ARCH_NAMES[arch];
  if (!goos || !goarch) {
    throw new Error(`unsupported platform ${platform}/${arch}; qweather-cli does not compile from source`);
  }
  const extension = platform === 'win32' ? 'zip' : 'tar.gz';
  const binary = platform === 'win32' ? 'qweather.exe' : 'qweather';
  const root = `qweather-cli_${version}_${goos}_${goarch}`;
  return {
    asset: `${root}.${extension}`,
    binary,
    root,
  };
}

module.exports = { selectAsset };
