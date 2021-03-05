'use strict';

exports.checkAvailability = async (backends, interval, maxPadsPerInstance) => {
  const superagent = require('superagent');
  const backendIds = Object.keys(backend);
  let available = Object.keys(backends);
  for (const backendId of Object.keys(backends)) {
    const backend = backends[backendId];
    // query if it's free
    const stats = await superagent.get(`http://${backend.host}:${backend.port}/stats`);
    const activePads = JSON.parse(stats.text).activePads;
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
      available = available.filter((bcakend) => backend !== backendId);
      // console.log(`delete backend: ${backendId}: ${activePads}`);
    }
  }
  if (available.length) return available[Math.floor(Math.random() * available.length)];
  return backendIds[Math.floor(Math.random() * backendIds.length)];
};
