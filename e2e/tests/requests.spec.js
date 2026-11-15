const { test, expect, view, card, open } = require('./fixtures');

async function added(harness) {
  const response = await fetch(harness.downloaderUrl + '/added');

  return (await response.json()).filter((movie) => movie.tmdbId === harness.wantedMovie.id);
}

async function openWanted(page, harness) {
  await page.getByRole('link', { name: 'Search' }).click();
  await expect(page).toHaveURL(/search/);
  await view(page).locator('.searchfields-txtSearch').fill('Wanted');
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

test.describe('a search', () => {
  test('finds a title the library lacks, and only of a kind a downloader takes', async ({ signedIn: page, harness }) => {
    await page.getByRole('link', { name: 'Search' }).click();
    await expect(page).toHaveURL(/search/);
    await view(page).locator('.searchfields-txtSearch').fill('Wanted');

    await expect(card(page, harness.wantedMovie.name)).toBeVisible();
    await expect(card(page, harness.wantedShow.name)).toHaveCount(0);
  });

  test('opens a title the library lacks as an item with nothing to play', async ({ signedIn: page, harness }) => {
    const failed = [];
    page.on('response', (response) => {
      if (response.status() >= 400) {
        failed.push(`${response.status()} ${new URL(response.url()).pathname}`);
      }
    });

    const details = await openWanted(page, harness);
    await expect(details.locator('h1.itemName')).toHaveText(harness.wantedMovie.name);
    await expect(details.locator('.overview')).toContainText(harness.wantedMovie.overview);
    await expect(details.locator('.tagline')).toContainText(harness.wantedMovie.tagline);
    await expect(details.locator('.mainDetailButtons .btnPlay')).toBeHidden();
    await expect(details.locator('.mainDetailButtons .btnUserRating')).toBeVisible();

    await expect(page).toHaveScreenshot('wanted-movie.png');
    expect(failed).toEqual([]);
  });
});
