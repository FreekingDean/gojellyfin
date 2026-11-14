const { test, expect, loadLogin, signIn, view } = require('./fixtures');

test.describe('signing in', () => {
  test('offers the seeded user on the login page', async ({ page, harness }) => {
    await loadLogin(page, harness);

    await expect(page.locator('#loginPage button.card')).toContainText(harness.username);
    await expect(page).toHaveScreenshot('login.png');
  });

  test('refuses the wrong password and stays put', async ({ page, harness }) => {
    await signIn(page, harness, 'not-the-password');

    await expect(page.getByText('Invalid username or password')).toBeVisible();
    await expect(page).toHaveURL(/login\.html/);
  });

  test('lands on the home screen', async ({ signedIn: page }) => {
    await expect(view(page)).toContainText('My Media');
  });
});
