const { test, expect, view, open } = require('./fixtures');

async function added(harness) {
  const response = await fetch(harness.downloaderUrl + '/added');

  return response.json();
}

async function openWanted(page, harness) {
  const row = view(page).locator('.verticalSection').filter({
    has: page.locator('.sectionTitle', { hasText: 'Recently Added in Trending Movies' }),
  });
  await row.scrollIntoViewIfNeeded();
  await open(page, harness.wantedMovie.name);
  await expect(page).toHaveURL(/details/);

  return view(page);
}

test.describe('a title the library lacks', () => {
  test('is asked of the downloader when it is hearted', async ({ signedIn: page, harness }) => {
    const details = await openWanted(page, harness);
    const heart = details.locator('.mainDetailButtons .btnUserRating');

    const marked = page.waitForResponse((response) => /FavoriteItems/i.test(response.url()) && response.request().method() === 'POST');
    await heart.click();
    expect((await marked).status()).toBe(200);

    await expect.poll(() => added(harness)).toHaveLength(1);
    const [movie] = await added(harness);
    expect(movie).toMatchObject({
      tmdbId: harness.wantedMovie.id,
      qualityProfileId: 1,
      rootFolderPath: '/fixtures',
      monitored: true,
      addOptions: { searchForMovie: true },
    });

    await page.reload();
    await expect(view(page).locator('h1.itemName')).toHaveText(harness.wantedMovie.name);
    await expect(view(page).locator('.tagline')).toContainText('Requested');
    await expect.soft(page).toHaveScreenshot('requested-movie.png');

    const withdrawn = page.waitForResponse((response) => /FavoriteItems/i.test(response.url()) && response.request().method() === 'DELETE');
    await view(page).locator('.mainDetailButtons .btnUserRating').click();
    expect((await withdrawn).status()).toBe(200);

    await page.reload();
    await expect(view(page).locator('.tagline')).toContainText(harness.wantedMovie.tagline);
    expect(await added(harness)).toHaveLength(1);
  });
});
