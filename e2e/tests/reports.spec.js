const { test, expect } = require('./fixtures');
const { GRAB } = require('../setup/radarr');

async function replaced(harness) {
  const response = await fetch(harness.downloaderUrl + '/replaced');

  return response.json();
}

function queued(page, name) {
  return page.locator('#gojellyfinRequestsPage [data-id=reportsHost] tr', { hasText: name });
}

async function openQueue(page) {
  await page.locator('.headerButton.mainDrawerButton').click();
  await page.locator('.mainDrawer').getByText('Dashboard').click();
  await expect(page).toHaveURL(/dashboard/);

  await page.getByRole('link', { name: 'Requests and Reports' }).click();
  await expect(page).toHaveURL(/configurationpage\?name=requests/i);
}

test.describe('a broken copy', () => {
  test('is reported from /report and replaced from the dashboard', async ({ signedIn: page, harness }) => {
    await page.goto(harness.baseUrl + '/report');
    await page.locator('#term').fill(harness.playable);
    await page.locator('.title', { hasText: harness.playable }).click();

    await expect(page.locator('#chosen')).toHaveText(harness.playable);
    await page.getByLabel('The sound is wrong or missing').check();
    await page.locator('#note').fill('silent after the titles');
    await page.getByRole('button', { name: 'Send report' }).click();
    await expect(page.locator('#done')).toContainText(harness.playable);

    await page.goto(harness.baseUrl + '/web/');
    await openQueue(page);
    const report = queued(page, harness.playable).filter({ hasText: 'Audio' });
    await expect(report).toContainText('silent after the titles');
    await expect(report).toContainText(harness.username);

    await report.locator('[data-id=replace]').click();
    await expect(report).toContainText('Resolved');
    await expect(report.locator('[data-id=replace]')).toBeHidden();

    expect(await replaced(harness)).toEqual([
      'DELETE /api/v3/moviefile/72',
      `POST /api/v3/history/failed/${GRAB}`,
      'POST /api/v3/command',
    ]);
  });

  test('is reported, and kept, when a client deletes it', async ({ signedIn: page, harness }) => {
    const found = await page.evaluate(async (name) => {
      const client = window.ApiClient;
      const items = await client.getItems(client.getCurrentUserId(), { SearchTerm: name, Recursive: true, IncludeItemTypes: 'Movie' });
      const [movie] = items.Items;
      await client.deleteItem(movie.Id);

      return { id: movie.Id, canDelete: movie.CanDelete, kept: (await client.getItem(client.getCurrentUserId(), movie.Id)).Name };
    }, harness.transcoded);

    expect(found.canDelete).toBe(true);
    expect(found.kept).toBe(harness.transcoded);

    await openQueue(page);
    const report = queued(page, harness.transcoded);
    await expect(report).toContainText('Unplayable');
    await expect(report).toContainText('Delete was pressed in a client');
  });
});
