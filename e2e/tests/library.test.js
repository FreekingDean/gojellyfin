const ui = require('../setup/ui');

describe('the library', () => {
  let session;

  beforeEach(async () => {
    session = await ui.open();
    await ui.signInAndLand(session.page);
  });

  afterEach(async () => {
    await session.close();
  });

  it('shows the library and its items on the home screen', async () => {
    await ui.waitForText(session.page, ui.harness().library);
    for (const movie of ui.harness().movies) {
      await ui.waitForText(session.page, movie);
    }
    const body = await ui.text(session.page);

    for (const movie of ui.harness().movies) {
      expect(body).toContain(movie);
    }
  });

  it('lists the movies when the library is opened', async () => {
    await ui.waitForText(session.page, ui.harness().library);
    await session.page.evaluate((name) => {
      const card = Array.from(document.querySelectorAll('a,button'))
        .find((element) => (element.innerText || '').trim() === name);
      card.click();
    }, ui.harness().library);

    await ui.waitForRoute(session.page, 'movies.html');
    for (const movie of ui.harness().movies) {
      await ui.waitForText(session.page, movie);
    }
    const body = await ui.text(session.page);

    for (const movie of ui.harness().movies) {
      expect(body).toContain(movie);
    }
  });

  it('opens an item and shows its detail page', async () => {
    const [movie] = ui.harness().movies;
    await ui.waitForText(session.page, movie);
    await session.page.evaluate((name) => {
      const card = Array.from(document.querySelectorAll('a,button'))
        .find((element) => (element.innerText || '').trim().startsWith(name));
      card.click();
    }, movie);

    await ui.waitForRoute(session.page, 'details');
    await ui.waitForText(session.page, movie);

    expect(await ui.text(session.page)).toContain(movie);
  });
});
