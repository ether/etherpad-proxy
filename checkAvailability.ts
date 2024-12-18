import superagent from 'superagent'

export type Backend = {
  host: string,
  port: number,
}

export type Backends = {
  [key: string]: Backend,
}



export const checkAvailability = async (backends: Backends, _interval: number, maxPadsPerInstance: number) => {
  let available = Object.keys(backends);
  let up = Object.keys(backends);
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
      available = available.filter((backend) => backend !== backendId);
      up = up.filter((backend) => backend !== backendId);
    }
  }
  // console.log('returning available backends of', available);
  // console.log('returning up backends of', up);
  return {
    up,
    available,
  };
};
