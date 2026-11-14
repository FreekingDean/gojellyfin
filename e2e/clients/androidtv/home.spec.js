const { test, expect, openLibrary } = require('./fixtures');

test.describe('browsing on the television', () => {
  test('lists the libraries on the home screen', async ({ signedIn: tv, harness, shot }) => {
    await tv.waitFor({ label: harness.shows });
    expect(tv.find({ label: /^My media$/i }).length).toBeGreaterThan(0);

    await shot('home');
  });

  test('shows what was added lately under the libraries', async ({ signedIn: tv, harness, shot }) => {
    tv.key('down');
    await tv.waitFor({ label: new RegExp(`^Recently added in ${harness.library}$`, 'i') });
    await tv.waitFor({ label: harness.movies[0] });

    await shot('home-recently-added');
  });

  test('opens a library and lists its titles', async ({ signedIn: tv, harness, shot }) => {
    await openLibrary(tv, harness);

    for (const movie of harness.movies) {
      expect(tv.find({ label: movie }).length).toBeGreaterThan(0);
    }
    await shot('library');
  });
});
