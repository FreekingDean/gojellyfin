const { defineConfig, devices } = require('@playwright/test');

const remote = process.env.E2E_BROWSER_WS;

// Screenshots are only compared in the pinned Playwright image, which is where
// CI renders and where the committed baselines came from. A browser on a
// laptop draws text with whatever fonts it has, so a run there checks
// behaviour and skips the pixels rather than failing on antialiasing.
module.exports = defineConfig({
  testDir: './tests',
  globalSetup: require.resolve('./setup/global-setup'),
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  timeout: 60000,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  ignoreSnapshots: !remote,
  snapshotPathTemplate: '{testDir}/__screenshots__/{testFilePath}/{arg}{ext}',
  expect: {
    timeout: 15000,
    toHaveScreenshot: {
      animations: 'disabled',
      caret: 'hide',
      maxDiffPixelRatio: 0.01,
      stylePath: require.resolve('./screenshot.css'),
    },
  },
  use: {
    ...devices['Desktop Chrome'],
    viewport: { width: 1280, height: 900 },
    locale: 'en-US',
    timezoneId: 'UTC',
    colorScheme: 'dark',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    connectOptions: remote ? { wsEndpoint: remote, exposeNetwork: '<loopback>' } : undefined,
  },
});
