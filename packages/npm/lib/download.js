'use strict';

const fs = require('node:fs');
const fsPromises = require('node:fs/promises');
const { Transform } = require('node:stream');
const { pipeline } = require('node:stream/promises');
const { EnvHttpProxyAgent, request: undiciRequest } = require('undici');

const DEFAULT_MAX_BYTES = 64 * 1024 * 1024;
const DEFAULT_MAX_REDIRECTS = 5;
const DEFAULT_TOTAL_TIMEOUT_MS = 5 * 60 * 1000;
const DEFAULT_IDLE_TIMEOUT_MS = 30 * 1000;

async function downloadToFile(url, destination, options = {}) {
  const request = options.request || undiciRequest;
  const proxyOptions = resolveProxyOptions(options.env || process.env);
  const controller = new AbortController();
  const timer = setTimeout(
    () => controller.abort(new Error('download exceeded five-minute total timeout')),
    options.totalTimeoutMs || DEFAULT_TOTAL_TIMEOUT_MS,
  );
  timer.unref?.();
  let dispatcher;
  const ownsDispatcher = !options.dispatcher;
  try {
    dispatcher = options.dispatcher || new EnvHttpProxyAgent(proxyOptions);
    await requestWithRedirects(new URL(url), destination, {
      request,
      dispatcher,
      maxBytes: options.maxBytes || DEFAULT_MAX_BYTES,
      maxRedirects: options.maxRedirects ?? DEFAULT_MAX_REDIRECTS,
      idleTimeoutMs: options.idleTimeoutMs || DEFAULT_IDLE_TIMEOUT_MS,
      signal: controller.signal,
    }, 0);
  } catch (error) {
    await fsPromises.rm(destination, { force: true });
    if (controller.signal.aborted && controller.signal.reason) throw controller.signal.reason;
    throw error;
  } finally {
    clearTimeout(timer);
    if (ownsDispatcher && dispatcher) await dispatcher.close();
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

async function requestOnce(url, destination, options) {
  const response = await options.request(url, {
    method: 'GET',
    headers: {
      'User-Agent': 'qweather-cli-npm-installer',
      Accept: 'application/octet-stream',
    },
    dispatcher: options.dispatcher,
    signal: options.signal,
    maxRedirections: 0,
    headersTimeout: options.idleTimeoutMs,
    bodyTimeout: options.idleTimeoutMs,
  });
  const status = response.statusCode || 0;
  if (status >= 300 && status < 400) {
    const location = header(response.headers, 'location');
    await discardResponse(response);
    if (!location) {
      throw new Error(`download redirect ${status} has no Location header`);
    }
    return { redirect: location };
  }
  if (status !== 200) {
    await discardResponse(response);
    throw new Error(`download failed with HTTP ${status}`);
  }
  const contentLength = Number(header(response.headers, 'content-length'));
  if (Number.isFinite(contentLength) && contentLength > options.maxBytes) {
    await discardResponse(response);
    throw new Error(`download exceeds the ${options.maxBytes}-byte size limit`);
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
  await pipeline(response.body, limiter, output);
  return {};
}

function resolveProxyOptions(env = process.env) {
  const httpProxy = firstNonEmpty(env.npm_config_proxy, env.http_proxy, env.HTTP_PROXY);
  return {
    httpProxy,
    httpsProxy: firstNonEmpty(
      env.npm_config_https_proxy,
      env.https_proxy,
      env.HTTPS_PROXY,
      httpProxy,
    ),
    noProxy: firstNonEmpty(env.npm_config_noproxy, env.no_proxy, env.NO_PROXY),
  };
}

function firstNonEmpty(...values) {
  return values.find((value) => typeof value === 'string' && value.length > 0) || '';
}

function header(headers, name) {
  if (!headers) return undefined;
  if (typeof headers.get === 'function') return headers.get(name) || undefined;
  return headers[name] ?? headers[name.toLowerCase()];
}

async function discardResponse(response) {
  if (!response?.body) return;
  if (typeof response.body.dump === 'function') {
    await response.body.dump();
    return;
  }
  response.body.resume?.();
}

module.exports = { downloadToFile, resolveProxyOptions };
