'use strict';

const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { Readable } = require('node:stream');
const test = require('node:test');

const { downloadToFile } = require('../lib/download.js');

function scriptedRequest(responses, observed) {
  return (url, options, callback) => {
    observed.push({ url: String(url), options });
    const request = new EventEmitter();
    request.setTimeout = () => request;
    request.end = () => {
      queueMicrotask(() => {
        const responseData = responses.shift();
        const response = Readable.from(responseData.body ? [responseData.body] : []);
        response.statusCode = responseData.statusCode;
        response.headers = responseData.headers || {};
        callback(response);
      });
    };
    request.destroy = (error) => {
      if (error) queueMicrotask(() => request.emit('error', error));
    };
    return request;
  };
}

test('downloadToFile forwards proxy environment and writes bounded bytes', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-download-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const destination = path.join(directory, 'archive');
  const observed = [];
  const proxyEnv = { HTTPS_PROXY: 'http://proxy.example:8080', NO_PROXY: 'github.com' };

  await downloadToFile('https://github.com/release/archive', destination, {
    request: scriptedRequest([{ statusCode: 200, body: Buffer.from('archive') }], observed),
    proxyEnv,
    maxBytes: 32,
  });

  assert.equal(await fs.readFile(destination, 'utf8'), 'archive');
  assert.equal(observed.length, 1);
  assert.equal(observed[0].options.proxyEnv, proxyEnv);
  assert.equal(observed[0].options.headers.Authorization, undefined);
});

test('downloadToFile rejects HTTPS downgrade redirects', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-download-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const destination = path.join(directory, 'archive');
  const observed = [];
  const request = scriptedRequest([{ statusCode: 302, headers: { location: 'http://example.com/archive' } }], observed);

  await assert.rejects(
    downloadToFile('https://github.com/release/archive', destination, { request }),
    /redirect must remain HTTPS/,
  );
  await assert.rejects(fs.stat(destination), /ENOENT/);
});

test('downloadToFile removes an oversized partial download', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-download-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const destination = path.join(directory, 'archive');
  const observed = [];
  const request = scriptedRequest([{ statusCode: 200, body: Buffer.alloc(33) }], observed);

  await assert.rejects(downloadToFile('https://github.com/release/archive', destination, { request, maxBytes: 32 }), /64 MiB|size limit/);
  await assert.rejects(fs.stat(destination), /ENOENT/);
});

test('downloadToFile stops after five redirects without retrying', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-download-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const destination = path.join(directory, 'archive');
  const observed = [];
  const responses = Array.from({ length: 6 }, (_, index) => ({
    statusCode: 302,
    headers: { location: `https://example.com/redirect-${index}` },
  }));

  await assert.rejects(
    downloadToFile('https://github.com/release/archive', destination, { request: scriptedRequest(responses, observed) }),
    /exceeded 5 HTTPS redirects/,
  );
  assert.equal(observed.length, 6);
});
