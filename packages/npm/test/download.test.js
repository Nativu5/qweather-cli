'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { Readable } = require('node:stream');
const test = require('node:test');

const { downloadToFile, resolveProxyOptions } = require('../lib/download.js');

function scriptedRequest(responses, observed) {
  return async (url, options) => {
    observed.push({ url: String(url), options });
    const responseData = responses.shift();
    return {
      statusCode: responseData.statusCode,
      headers: responseData.headers || {},
      body: Readable.from(responseData.body ? [responseData.body] : []),
    };
  };
}

test('downloadToFile resolves proxy configuration and writes bounded bytes', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-download-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const destination = path.join(directory, 'archive');
  const observed = [];
  const env = { npm_config_https_proxy: 'http://proxy.example:8080', NO_PROXY: 'github.com' };

  await downloadToFile('https://github.com/release/archive', destination, {
    request: scriptedRequest([{ statusCode: 200, body: Buffer.from('archive') }], observed),
    env,
    maxBytes: 32,
  });

  assert.equal(await fs.readFile(destination, 'utf8'), 'archive');
  assert.equal(observed.length, 1);
  assert.equal(observed[0].options.maxRedirections, 0);
  assert.deepEqual(resolveProxyOptions(env), {
    httpProxy: '',
    httpsProxy: 'http://proxy.example:8080',
    noProxy: 'github.com',
  });
  assert.equal(observed[0].options.headers.Authorization, undefined);
});

test('downloadToFile reuses one proxy dispatcher across HTTPS redirects', async (t) => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), 'qweather-download-'));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  const destination = path.join(directory, 'archive');
  const observed = [];
  const request = scriptedRequest([
    { statusCode: 302, headers: { location: 'https://release-assets.example/archive' } },
    { statusCode: 200, body: Buffer.from('archive') },
  ], observed);

  await downloadToFile('https://github.com/release/archive', destination, {
    request,
    env: { HTTPS_PROXY: 'http://proxy.example:8080' },
  });

  assert.equal(observed.length, 2);
  assert.equal(observed[0].options.dispatcher, observed[1].options.dispatcher);
  assert.equal(await fs.readFile(destination, 'utf8'), 'archive');
});

test('resolveProxyOptions gives npm lifecycle settings precedence over standard variables', () => {
  assert.deepEqual(resolveProxyOptions({
    npm_config_proxy: 'http://npm-http.example:8080',
    npm_config_https_proxy: 'http://npm-https.example:8081',
    npm_config_noproxy: 'npm.example',
    http_proxy: 'http://lower-http.example:8082',
    HTTPS_PROXY: 'http://upper-https.example:8083',
    NO_PROXY: 'upper.example',
  }), {
    httpProxy: 'http://npm-http.example:8080',
    httpsProxy: 'http://npm-https.example:8081',
    noProxy: 'npm.example',
  });
});

test('resolveProxyOptions prefers lowercase standard variables and reuses the HTTP proxy for HTTPS', () => {
  assert.deepEqual(resolveProxyOptions({
    http_proxy: 'http://lower-http.example:8080',
    HTTP_PROXY: 'http://upper-http.example:8081',
    no_proxy: 'lower.example',
    NO_PROXY: 'upper.example',
  }), {
    httpProxy: 'http://lower-http.example:8080',
    httpsProxy: 'http://lower-http.example:8080',
    noProxy: 'lower.example',
  });
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
