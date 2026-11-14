const { test, expect, view, card, open } = require('./fixtures');

const SECTIONS = /\/(Similar|SpecialFeatures|LocalTrailers|Intros)$/i;

// The client loads a card's image when the card nears the viewport, so a row
// far down a tall page is drawn or not depending on timing. Walking the page
// once brings every row in, and the wait holds until the requests that walk
// started have finished.
async function settle(page) {
  await page.evaluate(async () => {
    for (let top = 0; top < document.documentElement.scrollHeight; top += window.innerHeight / 2) {
      window.scrollTo(0, top);
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    window.scrollTo(0, 0);
  });
  await page.waitForLoadState('networkidle');
}

test.describe('the home screen', () => {
  test('shows a card for each library and the latest items', async ({ signedIn: page, harness }) => {
    const home = view(page);

    await expect(card(page, harness.library)).toBeVisible();
    await expect(card(page, harness.shows)).toBeVisible();
    for (const movie of harness.movies) {
      await expect(home).toContainText(movie);
    }

    await expect(home).toContainText(harness.wantedMovie.name);
    await expect(home).toContainText(harness.wantedShow.name);

    await settle(page);
    await expect(page).toHaveScreenshot('home.png', { fullPage: true });
  });

  test('lists the movies when a library card is opened', async ({ signedIn: page, harness }) => {
    await open(page, harness.library);

    await expect(page).toHaveURL(/#\/movies/);
    for (const movie of [...harness.movies, harness.playable]) {
      await expect(card(page, movie)).toBeVisible();
    }

    await expect(page).toHaveScreenshot('movies.png');
  });

  test('opens an item and shows its detail page', async ({ signedIn: page, harness }) => {
    const [alpha] = harness.movies;
    const asked = [];
    page.on('response', (response) => {
      const path = new URL(response.url()).pathname;
      if (SECTIONS.test(path)) {
        asked.push(`${path.split('/').pop()} ${response.status()}`);
      }
    });

    await open(page, alpha);

    await expect(page).toHaveURL(/details/);
    await expect(view(page).locator('h1.itemName')).toHaveText(alpha);
    await expect.poll(() => asked).toEqual(['Similar 200']);
  });
});
