'use strict';

exports.checkAvailability = async (backends, interval, maxPadsPerInstance) => {
  const superagent = require('superagent');
  let available = Object.keys(backends);
  for (const backendId of Object.keys(backends)) {
    const backend = backends[backendId];
    // query if it's free
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
  }
  if (available.length) {
    // there is an available backend so send it to that..
    // we should of done this above tho..
    const randomBackend = available[Math.floor(Math.random() * available.length)];
    // console.log(`random available backend: ${randomBackend}`);
    return randomBackend;
  } else {
    // no available backends so send it to a random backend XD
    // TODO future, support an error message if no backends are available?
    const randomBackend =
        Object.keys(backends)[Math.floor(Math.random() * Object.keys(backends).length)];
    // console.log(`Full up: random backend: ${randomBackend}`);
    return randomBackend;
  }
};
