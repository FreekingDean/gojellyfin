const fs = require('node:fs');

const { SERVER_LOG } = require('../setup/boot');

// What a failed test leaves behind: what was on screen, what the app and the
// server logged last, and a screenshot.
async function reportFailure(device, PACKAGE, testInfo) {
  for (const error of testInfo.errors) {
    console.log(`"${testInfo.title}" failed: ${error.message}`);
  }
  console.log(`On screen when "${testInfo.title}" ended:\n${device.summary()}`);
  console.log(`What the app logged:\n${appLog(device, PACKAGE)}`);
  if (fs.existsSync(SERVER_LOG)) {
    console.log(`What the server logged last:\n${fs.readFileSync(SERVER_LOG, 'utf8').split('\n').slice(-40).join('\n')}`);
  }
  const name = 'failed-' + testInfo.title.toLowerCase().replace(/[^a-z0-9]+/g, '-');
  await testInfo.attach(`${name}-current.png`, { path: device.screenshot(testInfo.outputPath('failure.png')) });
  await testInfo.attach('logcat.txt', { path: device.logcat(testInfo.outputPath('logcat.txt')) });
  if (fs.existsSync(SERVER_LOG)) {
    await testInfo.attach('server.log', { path: SERVER_LOG, contentType: 'text/plain' });
  }
}

// The app's own lines out of the device log, which is where it says why a
// screen did not change.
function appLog(device, PACKAGE) {
  const pid = device.shell(`pidof ${PACKAGE}`, { allowFailure: true }).trim().split(/\s+/)[0];
  if (!pid) {
    const crash = device.adb(['logcat', '-d', '-b', 'crash', '-v', 'brief'], { allowFailure: true }).trim();

    return `  (the app is not running)\n${crash.split('\n').slice(-80).join('\n')}`;
  }

  return device.adb(['logcat', '-d', '-v', 'brief', `--pid=${pid}`], { allowFailure: true }).split('\n').slice(-80).join('\n');
}

module.exports = { reportFailure };
