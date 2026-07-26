#!/usr/bin/env node
'use strict';

const { install } = require('./lib/install.js');

install().catch((error) => {
  process.stderr.write(`qweather-cli install failed: ${error.message}\n`);
  process.exitCode = 1;
});
