const http = require('node:http');
const net = require('node:net');
const path = require('node:path');
const fs = require('node:fs');

const TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
  '.woff2': 'font/woff2',
  '.woff': 'font/woff',
  '.ttf': 'font/ttf',
  '.webmanifest': 'application/manifest+json',
};

// Posters the seed painted, on the same origin as the client so the browser
// lets them through and the server can fetch them for a collage.
const POSTERS = '/fixture-posters';

// One origin, the way the chart's httproute.yaml serves it: /web is a file on
// disk and everything else is the API. Two would mean CORS and the client's
// server picker before any assertion. /web/ConfigurationPage and
// /web/ConfigurationPages are the two spec paths under /web, and the chart
// routes them back to the API; served as files, the dashboard's nav gets
// index.html instead of a list and the React dashboard dies on it.
const API_UNDER_WEB = /^\/web\/configurationpages?(\?|$)/i;

function start({ webRoot, apiPort, posterRoot }) {
  const server = http.createServer((request, response) => {
    const isWeb = request.url === '/web' || request.url.startsWith('/web/');
    if (isWeb && !API_UNDER_WEB.test(request.url)) {
      serveFile(webRoot, '/web', request, response);
      return;
    }
    if (posterRoot && request.url.startsWith(POSTERS + '/')) {
      serveFile(posterRoot, POSTERS, request, response);
      return;
    }

    proxy(apiPort, request, response);
  });

  // A signed-in client holds its websocket open for as long as it runs, and
  // close() waits for every connection, so the harness ends the upgraded
  // ones itself or a suite that finishes signed in never exits.
  const upgraded = new Set();
  const close = server.close.bind(server);
  server.close = (callback) => {
    for (const socket of upgraded) {
      socket.destroy();
    }
    server.closeAllConnections();
    return close(callback);
  };

  server.on('upgrade', (request, socket, head) => {
    upgraded.add(socket);
    socket.on('close', () => upgraded.delete(socket));
    const upstream = net.connect(apiPort, '127.0.0.1', () => {
      upstream.write(rebuildHead(request));
      if (head && head.length) {
        upstream.write(head);
      }
      upstream.pipe(socket);
      socket.pipe(upstream);
    });

    upstream.on('close', () => socket.destroy());
    socket.on('close', () => upstream.destroy());
    upstream.on('error', () => socket.destroy());
    socket.on('error', () => upstream.destroy());
  });

  return server;
}

function rebuildHead(request) {
  const lines = [`${request.method} ${request.url} HTTP/1.1`];
  for (let i = 0; i < request.rawHeaders.length; i += 2) {
    lines.push(`${request.rawHeaders[i]}: ${request.rawHeaders[i + 1]}`);
  }

  return lines.join('\r\n') + '\r\n\r\n';
}

function serveFile(webRoot, prefix, request, response) {
  const relative = decodeURIComponent(request.url.slice(prefix.length).split('?')[0]) || '/';
  const resolved = path.join(webRoot, path.normalize(relative));

  if (!resolved.startsWith(webRoot)) {
    response.writeHead(403).end();
    return;
  }

  let target = resolved;
  const missing = !fs.existsSync(target) || fs.statSync(target).isDirectory();
  // Only a navigation falls back to index.html: an asset that fell back would
  // reach the client as HTML and die as "Unexpected token '<'".
  if (missing) {
    if (path.extname(relative)) {
      response.writeHead(404, { 'Content-Type': 'text/plain' });
      response.end('no such file: ' + relative);
      return;
    }
    target = path.join(webRoot, 'index.html');
  }

  fs.readFile(target, (error, body) => {
    if (error) {
      response.writeHead(404).end();
      return;
    }

    response.writeHead(200, { 'Content-Type': TYPES[path.extname(target)] || 'application/octet-stream' });
    response.end(body);
  });
}

function proxy(apiPort, request, response) {
  const upstream = http.request(
    { host: '127.0.0.1', port: apiPort, path: request.url, method: request.method, headers: request.headers },
    (answer) => {
      response.writeHead(answer.statusCode, answer.headers);
      answer.pipe(response);
    },
  );

  upstream.on('error', () => {
    if (!response.headersSent) {
      response.writeHead(502);
    }
    response.end();
  });

  request.pipe(upstream);
}

module.exports = { start, POSTERS };
