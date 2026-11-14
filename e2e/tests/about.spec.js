const { test, expect, view } = require('./fixtures');

test.describe("the dashboard's server panel", () => {
  test("shows gojellyfin's build beside the API version", async ({ signedIn: page }) => {
    await page.locator('.headerButton.mainDrawerButton').click();
    await page.locator('.mainDrawer').getByText('Dashboard').click();
    await expect(page).toHaveURL(/dashboard/);

    const panel = view(page).locator('.serverInfo');
    await expect(panel.locator('#versionNumber')).toHaveText(/^\d+\.\d+\.\d+\+gojellyfin\.dev$/);

    await expect(panel).toHaveScreenshot('server-info.png');
  });

  test('keeps the public version a bare number clients can parse', async ({ page, harness }) => {
    const response = await page.request.get(harness.baseUrl + '/System/Info/Public');
    const info = await response.json();

    expect(info.Version).toMatch(/^\d+\.\d+\.\d+$/);
  });
});
