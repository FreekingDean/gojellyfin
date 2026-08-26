const ui = require('../setup/ui');

describe('signing in', () => {
  let session;

  beforeEach(async () => {
    session = await ui.open();
  });

  afterEach(async () => {
    await session.close();
  });

  it('offers the seeded user on the login page', async () => {
    await ui.loadLogin(session.page);

    expect(await ui.text(session.page)).toContain(ui.harness().username);
  });

  it('refuses the wrong password and stays put', async () => {
    await ui.signIn(session.page, 'not-the-password');
    await ui.waitForText(session.page, 'Invalid username or password');

    expect(session.page.url()).toContain('login.html');
  });

  it('lands on the home screen', async () => {
    await ui.signInAndLand(session.page);
    await ui.waitForText(session.page, 'My Media');

    expect(session.page.url()).toContain('home.html');
  });
});
