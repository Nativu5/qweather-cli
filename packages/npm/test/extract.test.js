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
