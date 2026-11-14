const { test, expect, view, route } = require('./fixtures');

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

    await expect(view(page)).toHaveScreenshot('media-downloaders.png');
  });

  test('lists who may ask for titles on the Requests page', async ({ signedIn: page, harness }) => {
    await route(page, '#/configurationpage?name=requests');

    const requests = page.locator('#gojellyfinRequestsPage');
    await expect(requests).toContainText('Who may ask');

    const user = requests.locator('[data-id=usersHost] tr', { hasText: harness.username });
    await expect(user.locator('[data-id=chkCanRequest]')).toBeChecked();
    await expect(user.locator('[data-id=chkNeedsApproval]')).not.toBeChecked();

    await user.locator('[data-id=txtQuota]').fill('5');
    await user.locator('[data-id=saveUser]').click();
    await expect(user.locator('[data-id=saveResult]')).toHaveText('Saved');

    await page.reload();
    await expect(requests.locator('[data-id=usersHost] tr', { hasText: harness.username }).locator('[data-id=txtQuota]')).toHaveValue('5');
  });
});
