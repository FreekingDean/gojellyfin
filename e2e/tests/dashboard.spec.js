const { test, expect, openDashboard } = require('./fixtures');

test.describe('the Requests dashboard page', () => {
  test('links the admin app for downloaders', async ({ signedIn: page }) => {
    await openDashboard(page);

    await expect(page.getByRole('link', { name: 'Media Downloaders' })).toHaveCount(0);
    await page.getByRole('link', { name: 'Requests' }).click();
    await expect(page).toHaveURL(/configurationpage\?name=requests/i);

    await expect(page.locator('#gojellyfinRequestsPage').getByRole('link', { name: 'admin app' })).toHaveAttribute('href', '/admin/libraries');
  });

  test('lists who may ask for titles on the Requests page', async ({ signedIn: page, harness }) => {
    await openDashboard(page);

    await page.getByRole('link', { name: 'Requests' }).click();
    await expect(page).toHaveURL(/configurationpage\?name=requests/i);

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
