'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const tar = require('tar');
const test = require('node:test');

const { extractArchive, validateEntries } = require('../lib/extract.js');

async function createTarFixture(directory, names) {
  const source = path.join(directory, 'source');
  await fs.mkdir(source, { recursive: true });
  for (const [name, contents] of Object.entries(names)) {
    const file = path.join(source, ...name.split('/'));
    await fs.mkdir(path.dirname(file), { recursive: true });
    await fs.writeFile(file, contents);
  }
  const archive = path.join(directory, 'fixture.tar.gz');
  await tar.c({ cwd: source, file: archive, gzip: true, portable: true }, Object.keys(names));
  return archive;
}

function crc32(data) {
  let crc = 0xffffffff;
  for (const byte of data) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit += 1) {
      crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

async function createZipFixture(directory, names) {
  const local = [];
  const central = [];
  let offset = 0;
  for (const [name, value] of Object.entries(names)) {
    const filename = Buffer.from(name);
    const contents = Buffer.from(value);
    const header = Buffer.alloc(30 + filename.length);
    header.writeUInt32LE(0x04034b50, 0);
    header.writeUInt16LE(20, 4);
    header.writeUInt16LE(0, 6);
    header.writeUInt16LE(0, 8);
    header.writeUInt32LE(crc32(contents), 14);
    header.writeUInt32LE(contents.length, 18);
    header.writeUInt32LE(contents.length, 22);
    header.writeUInt16LE(filename.length, 26);
    filename.copy(header, 30);
    local.push(header, contents);

    const directoryHeader = Buffer.alloc(46 + filename.length);
    directoryHeader.writeUInt32LE(0x02014b50, 0);
    directoryHeader.writeUInt16LE(20, 4);
    directoryHeader.writeUInt16LE(20, 6);
    directoryHeader.writeUInt16LE(0, 8);
    directoryHeader.writeUInt16LE(0, 10);
    directoryHeader.writeUInt32LE(crc32(contents), 16);
    directoryHeader.writeUInt32LE(contents.length, 20);
    directoryHeader.writeUInt32LE(contents.length, 24);
    directoryHeader.writeUInt16LE(filename.length, 28);
    directoryHeader.writeUInt32LE(0o100644 * 2 ** 16, 38);
    directoryHeader.writeUInt32LE(offset, 42);
    filename.copy(directoryHeader, 46);
    central.push(directoryHeader);
    offset += header.length + contents.length;
  }
  const centralDirectory = Buffer.concat(central);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(central.length ? Object.keys(names).length : 0, 8);
  end.writeUInt16LE(central.length ? Object.keys(names).length : 0, 10);
  end.writeUInt32LE(centralDirectory.length, 12);
  end.writeUInt32LE(offset, 16);
  const archive = path.join(directory, 'fixture.zip');
  await fs.writeFile(archive, Buffer.concat([...local, centralDirectory, end]));
  return archive;
}

test('extractArchive accepts the exact three-entry tar layout', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-extract-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const root = 'qweather-cli_1.2.3_linux_amd64';
  const archive = await createTarFixture(directory, {
    [`${root}/qweather`]: 'binary',
    [`${root}/LICENSE`]: 'license',
    [`${root}/README.md`]: 'readme',
  });
  const destination = path.join(directory, 'output');

  const binaryPath = await extractArchive(archive, destination, { root, binary: 'qweather' });

  assert.equal(binaryPath, path.join(destination, root, 'qweather'));
  assert.equal(await fs.readFile(binaryPath, 'utf8'), 'binary');
});

test('extractArchive accepts the exact three-entry zip layout', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-extract-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const root = 'qweather-cli_1.2.3_windows_amd64';
  const archive = await createZipFixture(directory, {
    [`${root}/qweather.exe`]: 'binary',
    [`${root}/LICENSE`]: 'license',
    [`${root}/README.md`]: 'readme',
  });
  const destination = path.join(directory, 'output');

  const binaryPath = await extractArchive(archive, destination, { root, binary: 'qweather.exe' });

  assert.equal(binaryPath, path.join(destination, root, 'qweather.exe'));
  assert.equal(await fs.readFile(binaryPath, 'utf8'), 'binary');
});

test('extractArchive rejects unexpected entries and size overflow', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-extract-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const root = 'qweather-cli_1.2.3_linux_amd64';
  const archive = await createTarFixture(directory, {
    [`${root}/qweather`]: 'binary',
    [`${root}/LICENSE`]: 'license',
    [`${root}/README.md`]: 'readme',
    [`${root}/unexpected`]: 'unexpected',
  });

  await assert.rejects(
    extractArchive(archive, path.join(directory, 'unexpected-output'), { root, binary: 'qweather' }),
    /exactly three|unexpected archive entry/,
  );

  const exactArchive = await createTarFixture(path.join(directory, 'second'), {
    [`${root}/qweather`]: 'binary',
    [`${root}/LICENSE`]: 'license',
    [`${root}/README.md`]: 'readme',
  });
  await assert.rejects(
    extractArchive(exactArchive, path.join(directory, 'large-output'), { root, binary: 'qweather', maxBytes: 5 }),
    /extracted size limit/,
  );
});

test('validateEntries rejects traversal paths and symlinks', () => {
  const root = 'qweather-cli_1.2.3_linux_amd64';
  const regular = [
    { name: `${root}/qweather`, size: 6, type: 'File' },
    { name: `${root}/LICENSE`, size: 7, type: 'File' },
    { name: `${root}/README.md`, size: 6, type: 'File' },
  ];
  assert.throws(
    () => validateEntries([{ ...regular[0], name: '../qweather' }, regular[1], regular[2]], root, 'qweather', 1024),
    /unsafe archive entry|unexpected archive entry/,
  );
  assert.throws(
    () => validateEntries([{ ...regular[0], type: 'SymbolicLink' }, regular[1], regular[2]], root, 'qweather', 1024),
    /not a regular file/,
  );
});
