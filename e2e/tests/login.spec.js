const { test, expect, loadLogin, signIn, view } = require('./fixtures');

test.describe('signing in', () => {
  test('asks for a username rather than listing the accounts', async ({ page, harness }) => {
    await loadLogin(page, harness);

    await expect(page.locator('#loginPage button.card')).toHaveCount(0);
    await expect(page.locator('#loginPage')).not.toContainText(harness.username);
    await expect(page.locator('#loginPage .btnQuick')).toBeVisible();
    await expect(page).toHaveScreenshot('login.png');
  });

  test('refuses the wrong password and stays put', async ({ page, harness }) => {
    await signIn(page, harness, 'not-the-password');

    await expect(page.getByText('Invalid username or password')).toBeVisible();
    await expect(page).toHaveURL(/#\/login/);
  });

  test('lands on the home screen', async ({ signedIn: page }) => {
    await expect(view(page)).toContainText('My Media');
  });
});
