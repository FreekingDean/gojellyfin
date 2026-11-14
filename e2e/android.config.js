const { defineConfig } = require('@playwright/test');

// The native Android app on a phone emulator, driven over adb. Playwright is
// only the runner here: there is no browser, and no screenshot is compared,
// because an emulator's frame is not reproducible the way the pinned browser
// image is. Each one is attached and published for a person to look at.
module.exports = defineConfig({
  testDir: './clients/android',
  globalSetup: require.resolve('./clients/android/setup'),
  outputDir: './test-results/android',
  fullyParallel: false,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  timeout: 240000,
  reporter: process.env.CI
    ? [['list'], ['json', { outputFile: '.build/android-results.json' }]]
    : 'list',
});
