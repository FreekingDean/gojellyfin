const { test, expect, openLibrary, onScreen } = require('./fixtures');

test.describe('browsing on a phone', () => {
  test('lists the libraries on the home screen', async ({ signedIn: phone, harness, shot }) => {
    await phone.waitFor({ label: harness.shows });
    expect(phone.find({ label: /^My Media$/i }).length).toBeGreaterThan(0);

    await shot('home');
  });

  test('shows what was added lately under the libraries', async ({ signedIn: phone, harness, shot }) => {
    await phone.waitFor({ label: new RegExp(`^Recently Added in ${harness.library}`, 'i') }, { timeout: 60000 });

    await shot('home-recently-added');
  });

  test('opens a library and lists its titles', async ({ signedIn: phone, harness, shot }) => {
    await openLibrary(phone, harness);

    await shot('library');
    expect(onScreen(phone, harness.movies)).toEqual(harness.movies);
  });
});
