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

async function collage(page, library) {
  const picture = card(page, library).locator('.cardImageContainer[style*="/Images/Primary"]').first();
  await expect(picture).toBeVisible();
  const style = await picture.getAttribute('style');
  const url = style.match(/url\("?([^")]+)"?\)/)[1];

  return page.evaluate(async (source) => {
    const response = await fetch(source);
    if (!response.ok || response.headers.get('Content-Type') !== 'image/jpeg') {
      throw new Error(`${source} answered ${response.status} ${response.headers.get('Content-Type')}`);
    }

    const bitmap = await createImageBitmap(await response.blob());
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
    const context = canvas.getContext('2d');
    context.drawImage(bitmap, 0, 0);

    const columns = 5;
    const rows = 2;
    const cells = [];
    for (let row = 0; row < rows; row++) {
      for (let column = 0; column < columns; column++) {
        const x = Math.floor(((column + 0.5) * bitmap.width) / columns);
        const y = Math.floor(((row + 0.5) * bitmap.height) / rows);
        const [r, g, b] = context.getImageData(x, y, 1, 1).data;
        cells.push([r, g, b]);
      }
    }

    return cells;
  }, url);
}

function near(cell, want) {
  return cell.every((value, index) => Math.abs(value - want[index]) <= 24);
}
