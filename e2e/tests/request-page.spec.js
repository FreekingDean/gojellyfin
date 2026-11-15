const { test, expect } = require('./fixtures');

async function added(harness) {
  const response = await fetch(harness.downloaderUrl + '/added');

  return (await response.json()).filter((movie) => movie.tmdbId === harness.askedMovie.id);
}

test.describe('the request page', () => {
  test('signs in on its own when the client has not', async ({ page, harness }) => {
    await page.goto(harness.baseUrl + '/request');
    await expect(page.getByRole('heading', { name: 'Sign in to request' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Sign in with single sign-on' })).toHaveAttribute('href', '/sso/oidc/start?redirect=%2Frequest');

    await page.getByPlaceholder('Username').fill(harness.username);
    await page.getByPlaceholder('Password').fill('not-the-password');
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(page.locator('#refused')).toHaveText('That username and password did not match.');

    await page.getByPlaceholder('Password').fill(harness.password);
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(page.getByRole('searchbox', { name: 'Search movies and shows' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'My requests' })).toBeVisible();
  });

  test('asks the downloader for a title found by search', async ({ signedIn: page, harness }) => {
    await page.goto(harness.baseUrl + '/request');
    await page.getByRole('searchbox', { name: 'Search movies and shows' }).fill('Asked');

    const result = page.locator('#results li', { hasText: harness.askedMovie.name });
    await expect(result).toContainText('2022 · Movie');
    await result.getByRole('button', { name: 'Request' }).click();

    await expect(page.locator('#said')).toHaveText('Requested ' + harness.askedMovie.name + '.');
    await expect(result.locator('.chip')).toHaveText('Requested');
    await expect(page.locator('#requested li', { hasText: harness.askedMovie.name })).toContainText('Requested');
    await expect.poll(() => added(harness)).toHaveLength(1);

    await page.getByRole('searchbox', { name: 'Search movies and shows' }).fill(harness.playable);
    await expect(page.locator('#results li', { hasText: harness.playable }).first().locator('.chip')).toHaveText('Ready');

    await page.getByRole('searchbox', { name: 'Search movies and shows' }).fill('Asked');
    await expect(result.locator('.chip')).toHaveText('Requested');
    await expect(page).toHaveScreenshot('request-page.png');
  });
});
