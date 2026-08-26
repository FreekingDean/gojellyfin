const ui = require('../setup/ui');

describe('a series', () => {
  let session;

  beforeEach(async () => {
    session = await ui.open();
    await ui.signInAndLand(session.page);
  });

  afterEach(async () => {
    await session.close();
  });

  it('lists the episodes of a season the client opened from the series', async () => {
    const { shows, series, season, episodes } = ui.harness();

    await ui.waitForText(session.page, shows);
    await click(session.page, shows);

    await ui.waitForRoute(session.page, 'tv.html');
    await ui.waitForText(session.page, series);
    await click(session.page, series);

    await ui.waitForRoute(session.page, 'details');
    expect(await ui.textOf(session.page, 'h1.itemName')).toBe(series);

    await ui.waitForText(session.page, season);
    await click(session.page, season);

    const body = await ui.waitForText(session.page, episodes[1]);
    expect(body).toContain(episodes[0]);
  });
});

function click(page, label) {
  return page.evaluate((name) => {
    const card = Array.from(document.querySelectorAll('a,button'))
      .find((element) => (element.innerText || '').trim().startsWith(name));
    if (!card) {
      throw new Error(`nothing on the page is labelled ${name}`);
    }

    card.click();
  }, label);
}
