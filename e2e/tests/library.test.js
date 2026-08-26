const ui = require('../setup/ui');

const HOME = '#indexPage';

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
    const [alpha, beta] = ui.harness().movies;
    const body = await ui.waitForText(session.page, beta, HOME);

    expect(body).toContain(ui.harness().library);
    expect(body).toContain(alpha);
  });

  it('lists the movies when the library is opened', async () => {
    const [alpha, beta] = ui.harness().movies;
    await ui.waitForText(session.page, ui.harness().library, HOME);
    await click(session.page, ui.harness().library);

    await ui.waitForRoute(session.page, 'movies.html');
    const body = await ui.waitForText(session.page, beta, '#moviesPage');

    expect(body).toContain(alpha);
  });

  it('opens an item and shows its detail page', async () => {
    const [alpha] = ui.harness().movies;
    await ui.waitForText(session.page, alpha, HOME);
    await click(session.page, alpha);

    await ui.waitForRoute(session.page, 'details');

    expect(await ui.textOf(session.page, 'h1.itemName')).toBe(alpha);
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
