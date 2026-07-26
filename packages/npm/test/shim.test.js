'use strict';

const assert = require('node:assert/strict');
const { execFile, spawn } = require('node:child_process');
const fs = require('node:fs/promises');
const path = require('node:path');
const { promisify } = require('node:util');
const test = require('node:test');

const execFileAsync = promisify(execFile);
const packageRoot = path.resolve(__dirname, '..');
const binary = path.join(packageRoot, 'libexec', process.platform === 'win32' ? 'qweather.exe' : 'qweather');
const shim = path.join(packageRoot, 'bin', 'qweather.js');

test('shim preserves arguments, stdout, stderr, and numeric exit code', { concurrency: false }, async (t) => {
  await fs.mkdir(path.dirname(binary), { recursive: true });
  await fs.writeFile(binary, '#!/usr/bin/env node\nprocess.stdout.write(`stdout:${process.argv.slice(2).join(",")}\\n`); process.stderr.write("stderr\\n"); process.exit(7);\n', { mode: 0o755 });
  t.after(() => fs.rm(path.dirname(binary), { recursive: true, force: true }));

  await assert.rejects(execFileAsync(process.execPath, [shim, 'alpha', 'two words']), (error) => {
    assert.equal(error.code, 7);
    assert.equal(error.stdout, 'stdout:alpha,two words\n');
    assert.equal(error.stderr, 'stderr\n');
    return true;
  });
});

test('shim reports explicit global and local repair guidance when the binary is missing', { concurrency: false }, async () => {
  await fs.rm(path.dirname(binary), { recursive: true, force: true });

  await assert.rejects(execFileAsync(process.execPath, [shim, '--help']), (error) => {
    assert.equal(error.code, 1);
    assert.equal(error.stdout, '');
    assert.match(error.stderr, /npm rebuild -g qweather-cli/);
    assert.match(error.stderr, /npm rebuild qweather-cli/);
    return true;
  });
});

test('shim preserves SIGTERM on POSIX', { skip: process.platform === 'win32', concurrency: false }, async (t) => {
  await fs.mkdir(path.dirname(binary), { recursive: true });
  await fs.writeFile(binary, '#!/usr/bin/env node\nprocess.stdout.write("ready\\n"); setInterval(() => {}, 1000);\n', { mode: 0o755 });
  t.after(() => fs.rm(path.dirname(binary), { recursive: true, force: true }));

  const child = spawn(process.execPath, [shim], { stdio: ['ignore', 'pipe', 'pipe'] });
  await new Promise((resolve, reject) => {
    child.stdout.once('data', resolve);
    child.once('error', reject);
  });
  child.kill('SIGTERM');
  const result = await new Promise((resolve) => child.once('exit', (code, signal) => resolve({ code, signal })));
  assert.deepEqual(result, { code: null, signal: 'SIGTERM' });
});

test('shim preserves SIGINT on POSIX', { skip: process.platform === 'win32', concurrency: false }, async (t) => {
  await fs.mkdir(path.dirname(binary), { recursive: true });
  await fs.writeFile(binary, '#!/usr/bin/env node\nprocess.stdout.write("ready\\n"); setInterval(() => {}, 1000);\n', { mode: 0o755 });
  t.after(() => fs.rm(path.dirname(binary), { recursive: true, force: true }));

  const child = spawn(process.execPath, [shim], { stdio: ['ignore', 'pipe', 'pipe'] });
  await new Promise((resolve, reject) => {
    child.stdout.once('data', resolve);
    child.once('error', reject);
  });
  child.kill('SIGINT');
  const result = await new Promise((resolve) => child.once('exit', (code, signal) => resolve({ code, signal })));
  assert.deepEqual(result, { code: null, signal: 'SIGINT' });
});
