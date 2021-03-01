'use strict';

// TODO: I think some of this logic isn't quite right as the value never seems to increase
exports.backends = [
  'localhost:9001',
  'localhost:9002',
];

exports.mostFreeBackend = {
  activePads: 0,
  backend: exports.backends[0],
};

exports.checkAvailability = () => {
  const superagent = require('superagent');

  // Hard coded backends for now.

  // Check instance availability once a second
  const checkAvailabilityInterval = 1000;

  setInterval(async () => {
    console.log(exports.mostFreeBackend);
    for (const backend of exports.backends) {
      // query if it's free
      const stats = await superagent.get(`http://${backend}/stats`);
      const activePads = JSON.parse(stats.text).activePads || 0;
      if (activePads === 0) {
        // console.log(`Free backend: ${backend} with ${activePads} active pads`);
        exports.mostFreeBackend.activePads = activePads;
        exports.mostFreeBackend.backend = backend;
        break;
      }
      if (activePads <= exports.mostFreeBackend.activePads) {
        // console.log(`Free backend: ${backend} with ${activePads} active pads`);
        exports.mostFreeBackend.activePads = activePads;
        exports.mostFreeBackend.backend = backend;
        break;
      }
    }
  }, checkAvailabilityInterval);
};
