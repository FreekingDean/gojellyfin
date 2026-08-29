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

// One origin, the way the chart's httproute.yaml serves it: /web is a file on
// disk and everything else is the API. Two would mean CORS and the client's
// server picker before any assertion.
function start({ webRoot, apiPort }) {
  const server = http.createServer((request, response) => {
    if (request.url === '/web' || request.url.startsWith('/web/')) {
      serveFile(webRoot, request, response);
      return;
    }

    proxy(apiPort, request, response);
  });

  server.on('upgrade', (request, socket, head) => {
    const upstream = net.connect(apiPort, '127.0.0.1', () => {
      upstream.write(rebuildHead(request));
      if (head && head.length) {
        upstream.write(head);
      }
      upstream.pipe(socket);
      socket.pipe(upstream);
    });

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

function serveFile(webRoot, request, response) {
  const relative = decodeURIComponent(request.url.slice('/web'.length).split('?')[0]) || '/';
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

module.exports = { start };
