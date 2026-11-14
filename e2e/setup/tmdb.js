const http = require('node:http');

// TMDB as the server sees it: the image configuration, the two trending lists
// and the detail record behind each title. Artwork is named under the proxy's
// own origin, because the browser is only let through to that one.
function start({ imageBase, movies, shows }) {
  const routes = {
    '/3/configuration': { images: { base_url: imageBase, secure_base_url: imageBase } },
    '/3/trending/movie/week': listing(movies, movieSummary),
    '/3/trending/tv/week': listing(shows, showSummary),
  };
  for (const movie of movies) {
    routes[`/3/movie/${movie.id}`] = { ...movieSummary(movie), tagline: movie.tagline, runtime: 101 };
  }
  for (const show of shows) {
    routes[`/3/tv/${show.id}`] = { ...showSummary(show), tagline: show.tagline, status: 'Returning Series' };
  }

  return http.createServer((request, response) => {
    const found = routes[new URL(request.url, 'http://tmdb.invalid').pathname];
    const body = found || { success: false, status_code: 34, status_message: 'The resource you requested could not be found.' };

    response.writeHead(found ? 200 : 404, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(body));
  });
}

function listing(titles, summarise) {
  return { page: 1, results: titles.map(summarise), total_pages: 1, total_results: titles.length };
}

function movieSummary(movie) {
  return {
    id: movie.id,
    title: movie.name,
    overview: movie.overview,
    release_date: movie.released,
    vote_average: 7.5,
    poster_path: movie.poster,
  };
}

function showSummary(show) {
  return {
    id: show.id,
    name: show.name,
    overview: show.overview,
    first_air_date: show.released,
    vote_average: 8.5,
    poster_path: show.poster,
  };
}

module.exports = { start };
