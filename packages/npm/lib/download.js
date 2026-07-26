'use strict';

const fs = require('node:fs');
const fsPromises = require('node:fs/promises');
const https = require('node:https');
const { Transform } = require('node:stream');
const { pipeline } = require('node:stream/promises');

const DEFAULT_MAX_BYTES = 64 * 1024 * 1024;
const DEFAULT_MAX_REDIRECTS = 5;
const DEFAULT_TOTAL_TIMEOUT_MS = 5 * 60 * 1000;
const DEFAULT_IDLE_TIMEOUT_MS = 30 * 1000;

async function downloadToFile(url, destination, options = {}) {
  const request = options.request || https.request;
  const proxyEnv = options.proxyEnv || process.env;
  const maxBytes = options.maxBytes || DEFAULT_MAX_BYTES;
  const maxRedirects = options.maxRedirects ?? DEFAULT_MAX_REDIRECTS;
  const totalTimeoutMs = options.totalTimeoutMs || DEFAULT_TOTAL_TIMEOUT_MS;
  const idleTimeoutMs = options.idleTimeoutMs || DEFAULT_IDLE_TIMEOUT_MS;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(new Error('download exceeded five-minute total timeout')), totalTimeoutMs);
  timer.unref?.();
  try {
    await requestWithRedirects(new URL(url), destination, {
      request,
      proxyEnv,
      maxBytes,
      maxRedirects,
      idleTimeoutMs,
      signal: controller.signal,
    }, 0);
  } catch (error) {
    await fsPromises.rm(destination, { force: true });
    throw error;
  } finally {
    clearTimeout(timer);
  }
}

async function requestWithRedirects(url, destination, options, redirects) {
  if (url.protocol !== 'https:') {
    throw new Error('download URL and every redirect must remain HTTPS');
  }
  const outcome = await requestOnce(url, destination, options);
  if (!outcome.redirect) return;
  if (redirects >= options.maxRedirects) {
    throw new Error(`download exceeded ${options.maxRedirects} HTTPS redirects`);
  }
  const next = new URL(outcome.redirect, url);
  if (next.protocol !== 'https:') {
    throw new Error('download redirect must remain HTTPS');
  }
  await requestWithRedirects(next, destination, options, redirects + 1);
}

function requestOnce(url, destination, options) {
  return new Promise((resolve, reject) => {
    let settled = false;
    const finish = (error, result) => {
      if (settled) return;
      settled = true;
      if (error) reject(error);
      else resolve(result);
    };
    const requestOptions = {
      method: 'GET',
      headers: {
        'User-Agent': 'qweather-cli-npm-installer',
        Accept: 'application/octet-stream',
      },
      proxyEnv: options.proxyEnv,
      signal: options.signal,
    };
    const request = options.request(url, requestOptions, (response) => {
      const status = response.statusCode || 0;
      if (status >= 300 && status < 400) {
        const location = response.headers.location;
        response.resume();
        if (!location) {
          finish(new Error(`download redirect ${status} has no Location header`));
          return;
        }
        finish(null, { redirect: location });
        return;
      }
      if (status !== 200) {
        response.resume();
        finish(new Error(`download failed with HTTP ${status}`));
        return;
      }
      const contentLength = Number(response.headers['content-length']);
      if (Number.isFinite(contentLength) && contentLength > options.maxBytes) {
        response.resume();
        finish(new Error(`download exceeds the ${options.maxBytes}-byte size limit`));
        return;
      }
      let received = 0;
      const limiter = new Transform({
        transform(chunk, _encoding, callback) {
          received += chunk.length;
          if (received > options.maxBytes) {
            callback(new Error(`download exceeds the ${options.maxBytes}-byte size limit`));
            return;
          }
          callback(null, chunk);
        },
      });
      const output = fs.createWriteStream(destination, { flags: 'wx', mode: 0o600 });
      pipeline(response, limiter, output).then(
        () => finish(null, {}),
        (error) => finish(error),
      );
    });
    request.once('error', (error) => finish(error));
    request.setTimeout(options.idleTimeoutMs, () => {
      request.destroy(new Error('download socket idle timeout'));
    });
    request.end();
  });
}

module.exports = { downloadToFile };
