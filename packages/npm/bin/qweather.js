#!/usr/bin/env node
'use strict';

const { spawn } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const binaryName = process.platform === 'win32' ? 'qweather.exe' : 'qweather';
const binaryPath = path.resolve(__dirname, '..', 'libexec', binaryName);

try {
  fs.accessSync(binaryPath, process.platform === 'win32' ? fs.constants.F_OK : fs.constants.X_OK);
} catch {
  process.stderr.write([
    'qweather-cli binary is missing or not executable.',
    'Repair a global install with: npm rebuild -g qweather-cli',
    'Repair a project-local install with: npm rebuild qweather-cli',
    '',
  ].join('\n'));
  process.exitCode = 1;
  return;
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  windowsHide: false,
});

const forwardedSignals = process.platform === 'win32' ? [] : ['SIGINT', 'SIGTERM', 'SIGHUP', 'SIGQUIT'];
const handlers = new Map();
for (const signal of forwardedSignals) {
  const handler = () => {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill(signal);
    }
  };
  handlers.set(signal, handler);
  process.on(signal, handler);
}

let settled = false;
const removeHandlers = () => {
  for (const [signal, handler] of handlers) {
    process.removeListener(signal, handler);
  }
};

child.once('error', (error) => {
  if (settled) return;
  settled = true;
  removeHandlers();
  process.stderr.write(`qweather-cli failed to launch its binary: ${error.message}\n`);
  process.exitCode = 1;
});

child.once('exit', (code, signal) => {
  if (settled) return;
  settled = true;
  removeHandlers();
  if (signal && process.platform !== 'win32') {
    try {
      process.kill(process.pid, signal);
    } catch (error) {
      process.stderr.write(`qweather-cli failed to preserve ${signal}: ${error.message}\n`);
      process.exitCode = 1;
    }
    return;
  }
  process.exitCode = Number.isInteger(code) ? code : 1;
});
