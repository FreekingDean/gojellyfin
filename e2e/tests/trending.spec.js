const { test, expect, view, open } = require('./fixtures');

function row(page, shelf) {
  return view(page).locator('.verticalSection').filter({
    has: page.locator('.sectionTitle', { hasText: `Recently Added in ${shelf}` }),
  });
}

test.describe('the trending rows', () => {
  test('list only what the provider calls trending and the library holds', async ({ signedIn: page, harness }) => {
    const movies = row(page, 'Trending Movies');
    await movies.scrollIntoViewIfNeeded();
    await expect(movies.locator('.card')).toHaveCount(1);
    await expect(movies.locator('.card')).toContainText(harness.playable);
    await expect(movies.locator('.card')).not.toContainText(harness.wantedMovie.name);

    await expect(row(page, 'Trending Shows')).toHaveCount(0);

    await expect(movies).toHaveScreenshot('trending-movies.png');
  });

  test('open a trending title as the item the library holds', async ({ signedIn: page, harness }) => {
    await row(page, 'Trending Movies').scrollIntoViewIfNeeded();
    await open(page, harness.playable);

    await expect(page).toHaveURL(/details/);
    const details = view(page);
    await expect(details.locator('h1.itemName')).toHaveText(harness.playable);
    await expect(details.locator('.mainDetailButtons .btnPlay')).toBeVisible();
  });
});
