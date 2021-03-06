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
      if (activePads === 0) {
        // console.log(`free backend: ${backendId}: ${activePads}`);
        return backendId;
      }
      if (activePads < maxPadsPerInstance) {
        // console.log(`available backend: ${backendId}: ${activePads}`);
        return backendId;
      } else {
        available = available.filter((backend) => backend !== backendId);
      }
    } catch (e) {
      // console.log(`removing backend: ${backendId}`);
      available = available.filter((backend) => backend !== backendId);
    }
  }
  if (available.length) {
    // there is an available backend so send it to that..
    const randomBackend = available[Math.floor(Math.random() * available.length)];
    // console.log(`random available backend: ${randomBackend}`);
    return randomBackend;
  }
};
