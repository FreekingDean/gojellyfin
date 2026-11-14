const {
  test,
  expect,
  freshStart,
  connect,
  openSignIn,
  signInWithPassword,
  quickConnectCode,
  qrOn,
  home,
} = require('./fixtures');

test.describe('signing in on the television', () => {
  test('connects to the server by its address', async ({ tv, harness, shot }) => {
    await freshStart(tv);
    await shot('welcome');
    await connect(tv, harness);

    expect(tv.find({ label: /unsupported/i })).toHaveLength(0);
    await shot('server');
  });

  test('signs in with a password', async ({ tv, harness, shot }) => {
    await freshStart(tv);
    await connect(tv, harness);
    await openSignIn(tv, harness);
    await signInWithPassword(tv, harness);

    await home(tv, harness);
    await shot('home-after-password');
  });

  test('signs in with a Quick Connect code another device approves', async ({ tv, harness, server, shot }) => {
    await freshStart(tv);
    await connect(tv, harness);
    await openSignIn(tv, harness);

    const code = await quickConnectCode(tv);
    await shot('quickconnect');
    await server.authorizeQuickConnect(code);

    await home(tv, harness);
    await shot('home-after-quickconnect');
  });

  // The app draws the server's splashscreen behind its sign-in screen, under
  // a tint and beside its own controls, and that picture carries the QR.
  test('shows a QR a phone can scan to reach this television\'s code', async ({ tv, harness, shot }) => {
    await freshStart(tv);
    await connect(tv, harness);
    await openSignIn(tv, harness);
    const code = await quickConnectCode(tv);

    const scanned = qrOn(await shot('quickconnect-qr'));
    expect(scanned, 'no QR could be read off the sign-in screen').not.toBeNull();
    expect(scanned.startsWith(harness.baseUrl + '/link/tv/')).toBe(true);

    const followed = await fetch(scanned, { redirect: 'manual' });
    expect(followed.status).toBe(302);
    expect(followed.headers.get('location')).toContain(code);
  });
});
