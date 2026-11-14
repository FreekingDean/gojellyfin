const { test, expect, view, open } = require('./fixtures');

function row(page, shelf) {
  return view(page).locator('.verticalSection').filter({
    has: page.locator('.sectionTitle', { hasText: `Recently Added in ${shelf}` }),
  });
}

test.describe('the trending rows', () => {
  test('list what the provider calls trending and the library does not hold', async ({ signedIn: page, harness }) => {
    const movies = row(page, 'Trending Movies');
    await movies.scrollIntoViewIfNeeded();
    await expect(movies.locator('.card')).toHaveCount(1);
    await expect(movies.locator('.card')).toContainText(harness.wantedMovie.name);
    await expect(movies.locator('.cardImageContainer[style*="/Images/Primary"]')).toBeVisible();

    const shows = row(page, 'Trending Shows');
    await shows.scrollIntoViewIfNeeded();
    await expect(shows.locator('.card')).toHaveCount(1);
    await expect(shows.locator('.card')).toContainText(harness.wantedShow.name);

    await expect(movies).toHaveScreenshot('trending-movies.png');
  });

  test('open a movie the library lacks as an item with nothing to play', async ({ signedIn: page, harness }) => {
    const failed = [];
    page.on('response', (response) => {
      if (response.status() >= 400) {
        failed.push(`${response.status()} ${new URL(response.url()).pathname}`);
      }
    });

    await row(page, 'Trending Movies').scrollIntoViewIfNeeded();
    await open(page, harness.wantedMovie.name);

    await expect(page).toHaveURL(/details/);
    const details = view(page);
    await expect(details.locator('h1.itemName')).toHaveText(harness.wantedMovie.name);
    await expect(details.locator('.overview')).toContainText(harness.wantedMovie.overview);
    await expect(details.locator('.tagline')).toContainText(harness.wantedMovie.tagline);
    await expect(details.locator('.mainDetailButtons .btnPlay')).toBeHidden();
    await expect(details.locator('.mainDetailButtons .btnUserRating')).toBeVisible();

    await expect(page).toHaveScreenshot('wanted-movie.png');
    expect(failed).toEqual([]);
  });

  test('open a show the library lacks', async ({ signedIn: page, harness }) => {
    await row(page, 'Trending Shows').scrollIntoViewIfNeeded();
    await open(page, harness.wantedShow.name);

    await expect(page).toHaveURL(/details/);
    const details = view(page);
    await expect(details.locator('h1.itemName')).toHaveText(harness.wantedShow.name);
    await expect(details.locator('.overview')).toContainText(harness.wantedShow.overview);
  });
});
