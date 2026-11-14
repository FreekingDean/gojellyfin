const { test, expect, view } = require('./fixtures');

test.describe('the Media Downloaders dashboard page', () => {
  test('is in the dashboard nav and lists the seeded connection', async ({ signedIn: page }) => {
    await page.locator('.headerButton.mainDrawerButton').click();
    await page.locator('.mainDrawer').getByText('Dashboard').click();
    await expect(page).toHaveURL(/dashboard/);

    await page.getByRole('link', { name: 'Media Downloaders' }).click();
    await expect(page).toHaveURL(/configurationpage\?name=media_downloaders/i);

    await expect(view(page)).toContainText('Library Sources');
    await expect(view(page)).toContainText('Fixtures');
    await expect(view(page)).toContainText('Add connection');

    await expect(page).toHaveScreenshot('media-downloaders.png');
  });
});
