const { test, expect, card } = require('./fixtures');

const RED = [220, 40, 40];
const BLUE = [40, 60, 220];
const GREEN = [40, 200, 70];

test.describe('the library cards', () => {
  test('draw the movie library as a collage of its posters', async ({ signedIn: page, harness }) => {
    const cells = await collage(page, harness.library);

    expect(cells.every((cell) => near(cell, RED) || near(cell, BLUE))).toBe(true);
    expect(cells.some((cell) => near(cell, RED))).toBe(true);
    expect(cells.some((cell) => near(cell, BLUE))).toBe(true);
    await expect(card(page, harness.library)).toHaveScreenshot('movies-card.png');
  });

  test('draw the show library as a collage of its series posters', async ({ signedIn: page, harness }) => {
    const cells = await collage(page, harness.shows);

    expect(cells.every((cell) => near(cell, GREEN))).toBe(true);
    await expect(card(page, harness.shows)).toHaveScreenshot('shows-card.png');
  });
});

test.describe('the shelf cards', () => {
  test('title Recently Aired and stack its shows with the newest in front', async ({ signedIn: page }) => {
    const [front, , corner] = await sampled(page, 'Recently Aired', STACK);

    expect(near(front, GREEN)).toBe(true);
    expect(near(corner, GREEN)).toBe(false);
  });

  test('title New Releases and stack its movies with the newest in front and the next behind it', async ({ signedIn: page }) => {
    const [front, beside, corner] = await sampled(page, 'New Releases', STACK);

    expect(near(front, RED) || near(front, BLUE)).toBe(true);
    expect(near(beside, near(front, RED) ? BLUE : RED)).toBe(true);
    expect(near(corner, front)).toBe(false);
  });
});

const GRID = Array.from({ length: 10 }, (_, cell) => [((cell % 5) + 0.5) / 5, (Math.floor(cell / 5) + 0.5) / 2]);
const STACK = [
  [0.885, 0.5],
  [0.73, 0.7],
  [0.02, 0.96],
];

function collage(page, library) {
  return sampled(page, library, GRID);
}

async function sampled(page, library, points) {
  const picture = card(page, library).locator('.cardImageContainer[style*="/Images/Primary"]').first();
  await expect(picture).toBeVisible();
  const style = await picture.getAttribute('style');
  const url = style.match(/url\("?([^")]+)"?\)/)[1];

  return page.evaluate(async ({ source, spots }) => {
    const response = await fetch(source);
    if (!response.ok || response.headers.get('Content-Type') !== 'image/jpeg') {
      throw new Error(`${source} answered ${response.status} ${response.headers.get('Content-Type')}`);
    }

    const bitmap = await createImageBitmap(await response.blob());
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
    const context = canvas.getContext('2d');
    context.drawImage(bitmap, 0, 0);

    return spots.map(([across, down]) => {
      const x = Math.floor(across * bitmap.width);
      const y = Math.floor(down * bitmap.height);
      const [r, g, b] = context.getImageData(x, y, 1, 1).data;

      return [r, g, b];
    });
  }, { source: url, spots: points });
}

function near(cell, want) {
  return cell.every((value, index) => Math.abs(value - want[index]) <= 24);
}
