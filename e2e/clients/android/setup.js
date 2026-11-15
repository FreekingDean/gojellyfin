const { emulatorSetup } = require('../setup');

// The emulator draws in software, and a Pixel 6's full panel took the phone
// app 26 seconds a frame on a CI runner, slow enough that typed text never
// reached the connect field. A 720x1600 panel is under half the pixels.
module.exports = emulatorSetup({ ...require('./app'), display: { size: '720x1600', density: 320 } });
