const { test, expect, view, openDashboard } = require('./fixtures');

test.describe("the dashboard's server panel", () => {
  test("shows gojellyfin's build beside the API version", async ({ signedIn: page }) => {
    await openDashboard(page);

    const panel = view(page).locator('.MuiPaper-root', { hasText: 'Server version' });
    await expect(panel.getByText(/^\d+\.\d+\.\d+\+gojellyfin\.dev$/)).toBeVisible();

    await expect(panel).toHaveScreenshot('server-info.png');
  });

  test('keeps the public version a bare number clients can parse', async ({ page, harness }) => {
    const response = await page.request.get(harness.baseUrl + '/System/Info/Public');
    const info = await response.json();

    expect(info.Version).toMatch(/^\d+\.\d+\.\d+$/);
  });
});
