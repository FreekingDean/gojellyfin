const { test, expect } = require('./fixtures');

const ADMIN = 'MediaBrowser Client="Infuse spec", Device="Setup", DeviceId="infuse-spec-admin", Version="1"';
const INFUSE = 'MediaBrowser Client="Infuse-Direct", Device="iPhone", DeviceId="infuse-spec-iphone", Version="8.5.8"';
const VIEWER = { Name: 'infuse-viewer', Password: 'infuse-viewer-password' };

const LIST_FIELDS = 'DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,ProviderIds,SortName,RecursiveItemCount,ChildCount';
const EPISODE_FIELDS = 'Etag,MediaSources,AlternateMediaSources,Genres,Overview,ParentId,ProviderIds';

async function signIn(baseUrl, authorization, username, password) {
  const response = await fetch(baseUrl + '/Users/AuthenticateByName', {
    method: 'POST',
    headers: { Authorization: authorization, 'Content-Type': 'application/json' },
    body: JSON.stringify({ Username: username, Pw: password }),
  });
  expect(response.ok).toBeTruthy();
  const body = await response.json();

  return { token: body.AccessToken, userId: body.User.Id };
}

function client(baseUrl, token) {
  return async (path) => {
    const response = await fetch(baseUrl + path, {
      headers: { Authorization: `${INFUSE}, Token="${token}"`, 'User-Agent': 'Infuse-Direct/8.5.8' },
    });

    return { status: response.status, body: response.ok ? await response.json() : null };
  };
}

function expectNamedFiles(items) {
  expect(items.length).toBeGreaterThan(0);
  for (const item of items) {
    expect(item.Path, `${item.Name} carries no Path`).toMatch(/^[^/]+\.[a-z0-9]+$/);
    expect(item.MediaSources?.length, `${item.Name} lists no MediaSources`).toBeGreaterThan(0);
    expect(item.MediaSources[0].Path).toBe(item.Path);
  }
}

test.describe('Infuse', () => {
  let viewer;
  let admin;

  test.beforeAll(async ({ harness }) => {
    admin = await signIn(harness.baseUrl, ADMIN, harness.username, harness.password);
    const created = await fetch(harness.baseUrl + '/Users/New', {
      method: 'POST',
      headers: { Authorization: `${ADMIN}, Token="${admin.token}"`, 'Content-Type': 'application/json' },
      body: JSON.stringify(VIEWER),
    });
    expect(created.ok).toBeTruthy();
    viewer = await signIn(harness.baseUrl, INFUSE, VIEWER.Name, VIEWER.Password);
  });

  test.afterAll(async ({ harness }) => {
    if (!viewer) return;
    await fetch(harness.baseUrl + `/Users/${viewer.userId}`, {
      method: 'DELETE',
      headers: { Authorization: `${ADMIN}, Token="${admin.token}"` },
    });
  });

  test('a user who is not an administrator can open every library Infuse lists', async ({ harness }) => {
    const get = client(harness.baseUrl, viewer.token);

    expect((await get('/Library/VirtualFolders')).status).toBe(403);

    const views = await get(`/Users/${viewer.userId}/Views`);
    expect(views.status).toBe(200);
    const movies = views.body.Items.find((view) => view.CollectionType === 'movies');
    const shows = views.body.Items.find((view) => view.CollectionType === 'tvshows');

    const listed = await get(
      `/Items?excludeLocationTypes=Virtual&parentId=${movies.Id}&sortBy=SortName,ProductionYear&sortOrder=Ascending` +
        `&includeItemTypes=Movie&recursive=true&startIndex=0&limit=50&fields=${LIST_FIELDS}&userId=${viewer.userId}`,
    );
    expect(listed.status).toBe(200);
    expectNamedFiles(listed.body.Items);

    const latest = await get(`/Items/Latest?parentId=${movies.Id}&limit=20&fields=${LIST_FIELDS}&userId=${viewer.userId}`);
    expect(latest.status).toBe(200);
    expectNamedFiles(latest.body.filter((item) => item.Type === 'Movie'));

    const episodes = await get(
      `/Users/${viewer.userId}/Items?ParentId=${shows.Id}&Recursive=true&IncludeItemTypes=Episode&Fields=${EPISODE_FIELDS},Path`,
    );
    expect(episodes.status).toBe(200);
    expectNamedFiles(episodes.body.Items);

    const seasons = await get(`/Users/${viewer.userId}/Items?ParentId=${shows.Id}&Recursive=true&IncludeItemTypes=Season`);
    expect(seasons.body.TotalRecordCount).toBeGreaterThan(0);

    const series = episodes.body.Items[0].SeriesId;
    const ofSeries = await get(`/Shows/${series}/Episodes?userId=${viewer.userId}&fields=${EPISODE_FIELDS}`);
    expect(ofSeries.status).toBe(200);
    expectNamedFiles(ofSeries.body.Items);
  });
});
