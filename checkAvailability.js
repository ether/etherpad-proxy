'use strict';

exports.checkAvailability = async (backends, interval, maxPadsPerInstance) => {
  const superagent = require('superagent');
  let available = Object.keys(backends);
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
      available = available.filter((backend) => backend !== backendId);
    }
  }
  if (available.length) {
    // there is an available backend so send it to that..
    // we should of done this above tho..
    return available[Math.floor(Math.random() * available.length)];
  } else {
    // no available backends so send it to a random backend XD
    // TODO future, support an error message if no backends are available?
    return Object.keys(backends)[Math.floor(Math.random() * Object.keys(backends).length)];
  }
};
