const { emulatorSetup } = require('../setup');

// The television plays a series' episodes one after another, so each is a
// real file there.
module.exports = emulatorSetup({ ...require('./app'), playableEpisodes: true });
