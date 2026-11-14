const { test, expect, view } = require('./fixtures');

// A Continue Watching card is landscape, so the client asks for a Thumb, then
// the item's backdrop, then an episode's series backdrop, and only then the
// poster. The seed gives Beta and the series a backdrop and Alpha a poster
// alone, so each card exercises one rung of that ladder.
function section(page) {
  return view(page)
    .locator('.verticalSection')
    .filter({ has: page.locator('.sectionTitle', { hasText: 'Continue Watching' }) });
}

async function imageOf(page, name) {
  const image = section(page)
    .locator('.card')
    .filter({ has: page.locator('.cardText', { hasText: name }) })
    .first()
    .locator('.cardImageContainer')
    .first();

  await expect(image).toHaveCSS('background-image', /url\(/);

  return image.evaluate((element) => getComputedStyle(element).backgroundImage);
}

test.describe('continue watching', () => {
  test('draws every card from the landscape fallbacks', async ({ signedIn: page, harness }) => {
    const [alpha, beta, pilot] = harness.resumed;
    const row = section(page);

    for (const name of harness.resumed) {
      await expect(row).toContainText(name);
    }

    expect(await imageOf(page, beta)).toMatch(/\/Images\/Backdrop/i);
    expect(await imageOf(page, pilot)).toMatch(/\/Images\/Backdrop/i);
    expect(await imageOf(page, alpha)).toMatch(/\/Images\/Primary/i);

    await row.scrollIntoViewIfNeeded();
    await expect(row).toHaveScreenshot('continue-watching.png');
  });
});
