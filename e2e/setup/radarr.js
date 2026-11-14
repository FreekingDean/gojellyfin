const http = require('node:http');

// Radarr as a request sees it: the lookup that describes a title by its TMDB
// id and the add that takes it. What was added is kept and answered on /added,
// which is the test's window rather than anything Radarr serves. A broken copy
// is replaced through the history, the file record and a search command, and
// /replaced answers those calls in the order they arrived.
function start() {
  const added = [];
  const replaced = [];

  return http.createServer((request, response) => {
    const url = new URL(request.url, 'http://radarr.invalid');
    const answer = (status, body) => {
      response.writeHead(status, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify(body));
    };

    if (url.pathname === '/added') {
      answer(200, added);
    } else if (url.pathname === '/replaced') {
      answer(200, replaced);
    } else if (url.pathname === '/api/v3/history/movie') {
      answer(200, [{ id: GRAB, date: '2026-01-02T00:00:00Z' }]);
    } else if (/^\/api\/v3\/(moviefile|history\/failed)\/\d+$/.test(url.pathname) || url.pathname === '/api/v3/command') {
      replaced.push(request.method + ' ' + url.pathname);
      request.resume();
      answer(200, {});
    } else if (url.pathname === '/api/v3/movie/lookup/tmdb') {
      answer(200, { tmdbId: Number(url.searchParams.get('tmdbId')), title: 'Looked up' });
    } else if (url.pathname === '/api/v3/tag') {
      answer(200, []);
    } else if (url.pathname === '/api/v3/movie' && request.method === 'POST') {
      let body = '';
      request.on('data', (chunk) => (body += chunk));
      request.on('end', () => {
        added.push(JSON.parse(body));
        answer(201, { id: added.length });
      });
    } else {
      answer(404, { message: 'NotFound' });
    }
  });
}

const GRAB = 9;

module.exports = { start, GRAB };
