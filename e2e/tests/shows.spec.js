const { test, expect, view, card, open } = require('./fixtures');

test.describe('a series', () => {
  test('lists the episodes of a season opened from the series', async ({ signedIn: page, harness }) => {
    const { shows, series, season, episodes } = harness;

    await open(page, shows);
    await expect(page).toHaveURL(/#\/tv/);

    await open(page, series);
    await expect(page).toHaveURL(/details/);
    await expect(view(page).locator('h1.itemName')).toHaveText(series);

    await open(page, season);
    for (const episode of episodes) {
      await expect(view(page)).toContainText(episode);
    }
  });
});
