'use strict';

exports.checkAvailability = async (backends, interval, maxPadsPerInstance) => {
  const superagent = require('superagent');
  for (const backendId of Object.keys(backends)) {
    const backend = backends[backendId];
    // query if it's free
    const stats = await superagent.get(`http://${backend.host}:${backend.port}/stats`);
    const activePads = JSON.parse(stats.text).activePads || 0;
    if (activePads === 0) {
      // console.log(`Free backend: ${backend} with ${activePads} active pads`);
      return backendId;
    }
    if (activePads <= maxPadsPerInstance) {
      return backendId;
    }
    // Return a backend if none of the others are totally free..
    return backends[Math.floor(Math.random() * backends.length)];
  }
};
