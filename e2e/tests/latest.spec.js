const { test, expect, view } = require('./fixtures');

function row(page, library) {
  return view(page).locator('.verticalSection').filter({
    has: page.locator('.sectionTitle', { hasText: `Recently Added in ${library}` }),
  });
}

test.describe('the recently added row of a show library', () => {
  test('shows a lone new episode as itself and folds several into their series', async ({ signedIn: page, harness }) => {
    const shows = row(page, harness.shows);
    const cards = shows.locator('.card');

    await shows.scrollIntoViewIfNeeded();
    await expect(cards).toHaveCount(2);

    const [newest, grouped] = [cards.nth(0), cards.nth(1)];
    await expect(newest.locator('.cardText').first()).toHaveText(harness.drama);
    await expect(newest).toContainText(harness.finale);
    await expect(newest.locator('.cardImageContainer[style*="/Images/Primary"]')).toBeVisible();

    await expect(grouped.locator('.cardText').first()).toHaveText(harness.series);
    for (const episode of harness.episodes) {
      await expect(grouped).not.toContainText(episode);
    }

    await expect(shows).toHaveScreenshot('recently-added-shows.png');
  });
});
