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

test.describe('the virtual rows', () => {
  test('list the newest aired episode of each show, newest first', async ({ signedIn: page, harness }) => {
    const aired = row(page, 'Recently Aired');
    await aired.scrollIntoViewIfNeeded();

    const cards = aired.locator('.card');
    await expect(cards).toHaveCount(2);
    await expect(cards.nth(0).locator('.cardText').first()).toHaveText(harness.series);
    await expect(cards.nth(0)).toContainText(harness.episodes[1]);
    await expect(cards.nth(1).locator('.cardText').first()).toHaveText(harness.drama);
    await expect(cards.nth(1)).toContainText(harness.finale);

    await expect(aired).toHaveScreenshot('recently-aired.png');
  });

  test('list the movies by release date, newest first', async ({ signedIn: page, harness }) => {
    const [alpha, beta] = harness.movies;
    const releases = row(page, 'New Releases');
    await releases.scrollIntoViewIfNeeded();

    const cards = releases.locator('.card');
    await expect(cards).toHaveCount(2);
    await expect(cards.nth(0)).toContainText(alpha);
    await expect(cards.nth(1)).toContainText(beta);

    await expect(releases).toHaveScreenshot('new-releases.png');
  });
});
