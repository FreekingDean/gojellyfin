const { test, expect, freshStart, connect, signInWithPassword, quickConnectCode, home } = require('./fixtures');

test.describe('signing in on a phone', () => {
  test('connects to the server by its address', async ({ phone, harness, shot }) => {
    await freshStart(phone);
    await shot('welcome');
    await connect(phone, harness);

    expect(phone.find({ label: /unsupported|not compatible/i })).toHaveLength(0);
    await shot('login');
  });

  test('signs in with a password', async ({ phone, harness, shot }) => {
    await freshStart(phone);
    await connect(phone, harness);
    await signInWithPassword(phone, harness);

    await home(phone, harness);
    await shot('home-after-password');
  });

  test('signs in with a Quick Connect code another device approves', async ({ phone, harness, server, shot }) => {
    await freshStart(phone);
    await connect(phone, harness);

    const code = await quickConnectCode(phone);
    await shot('quickconnect');
    await server.authorizeQuickConnect(code);

    await home(phone, harness);
    await shot('home-after-quickconnect');
  });
});
