'use strict';

exports.checkAvailability = async (backends, interval, maxPadsPerInstance) => {
  const superagent = require('superagent');
  for (const backendId of Object.keys(backends)) {
    const backend = backends[backendId];
    // query if it's free
    const stats = await superagent.get(`http://${backend.host}:${backend.port}/stats`);
    const activePads = JSON.parse(stats.text).activePads || 0;
    console.log(`${backendId}: ${activePads}`);
    if (activePads === 0) {
      // console.log(`Free backend: ${backend} with ${activePads} active pads`);
      console.log(`free backend: ${backendId}: ${activePads}`);
      return backendId;
    }
    if (activePads < maxPadsPerInstance) {
      console.log(`available backend: ${backendId}: ${activePads}`);
      return backendId;
    } else {
      console.log(`delete backend: ${backendId}: ${activePads}`);
      delete backends.backendId;
    }
  }
  // TODO handle no backends available gracefully.
  if (backends.length === 0) throw new Error('Server full');
  const randomBackend = Math.floor(Math.random() * Object.keys(backends.length));
  return randomBackend;
};
