'use strict';

exports.checkAvailability = async (backends, interval, maxPadsPerInstance) => {
  const superagent = require('superagent');
  let available = Object.keys(backends);
  for (const backendId of Object.keys(backends)) {
    const backend = backends[backendId];
    // query if it's free
    try {
      const stats = await superagent.get(`http://${backend.host}:${backend.port}/stats`);
      const activePads = JSON.parse(stats.text).activePads || 0;
      if (activePads < maxPadsPerInstance) {
        // console.log(`available backend: ${backendId}: ${activePads}`);
      } else {
        available = available.filter((backend) => backend !== backendId);
      }
    } catch (e) {
      // console.log(`removing backend: ${backendId}`);
      available = available.filter((backend) => backend !== backendId);
    }
  }
  return available;
};
