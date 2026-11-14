const http = require('node:http');

// Radarr as a request sees it: the lookup that describes a title by its TMDB
// id and the add that takes it. What was added is kept and answered on /added,
// which is the test's window rather than anything Radarr serves.
function start() {
  const added = [];

  return http.createServer((request, response) => {
    const url = new URL(request.url, 'http://radarr.invalid');
    const answer = (status, body) => {
      response.writeHead(status, { 'Content-Type': 'application/json' });
      response.end(JSON.stringify(body));
    };

    if (url.pathname === '/added') {
      answer(200, added);
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

module.exports = { start };
