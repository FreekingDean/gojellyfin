const fs = require('node:fs');
const path = require('node:path');

const ui = require('../setup/ui');

const HOME = '#indexPage';
const SHOTS = path.join(__dirname, '..', '.build', 'screenshots');

const RED = [220, 40, 40];
const BLUE = [40, 60, 220];
const GREEN = [40, 200, 70];

describe('the library cards', () => {
  let session;

  beforeEach(async () => {
    session = await ui.open();
    await ui.signInAndLand(session.page);
    await ui.waitForText(session.page, ui.harness().shows, HOME);
  });

  afterEach(async () => {
    await session.close();
  });

  it('draws the movie library as a collage of its posters', async () => {
    const cells = await collage(session.page, ui.harness().library);

    expect(cells.every((cell) => near(cell, RED) || near(cell, BLUE))).toBe(true);
    expect(cells.some((cell) => near(cell, RED))).toBe(true);
    expect(cells.some((cell) => near(cell, BLUE))).toBe(true);
  });

  it('draws the show library as a collage of its series posters', async () => {
    const cells = await collage(session.page, ui.harness().shows);

    expect(cells.every((cell) => near(cell, GREEN))).toBe(true);
  });
});

async function collage(page, library) {
  const handle = await page.waitForFunction(
    (home, name) => {
      const card = Array.from(document.querySelectorAll(`${home} .card`))
        .filter((element) => element.offsetParent !== null)
        .find((element) => (element.innerText || '').trim().startsWith(name));
      const picture = card && card.querySelector('.cardImageContainer');
      if (!picture || !picture.style.backgroundImage.includes('/Images/Primary')) {
        return null;
      }

      return picture.style.backgroundImage.slice(5, -2);
    },
    { timeout: 30000 },
    HOME,
    library,
  );
  const url = await handle.jsonValue();

  await screenshot(page, library);

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

async function screenshot(page, library) {
  fs.mkdirSync(SHOTS, { recursive: true });
  const card = await page.evaluateHandle(
    (home, name) =>
      Array.from(document.querySelectorAll(`${home} .card`))
        .filter((element) => element.offsetParent !== null)
        .find((element) => (element.innerText || '').trim().startsWith(name)),
    HOME,
    library,
  );
  await card.asElement().screenshot({ path: path.join(SHOTS, `${library.replace(/\W+/g, '-')}.png`) });
}

function near(cell, want) {
  return cell.every((value, index) => Math.abs(value - want[index]) <= 24);
}
