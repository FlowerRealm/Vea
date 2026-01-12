import { createAPI, utils } from './vea-sdk.esm.js';

(async function () {
  const candidates = [
    new URL('../_shared/js/app.js', import.meta.url),
    new URL('../../_shared/js/app.js', import.meta.url),
  ];

  let mod = null;
  let lastErr = null;

  for (const url of candidates) {
    try {
      mod = await import(url.href);
      break;
    } catch (err) {
      lastErr = err;
    }
  }

  if (!mod || typeof mod.bootstrapTheme !== 'function') {
    const msg = lastErr && lastErr.message ? lastErr.message : String(lastErr || 'unknown');
    throw new Error(`Failed to load theme app module: ${msg}`);
  }

  mod.bootstrapTheme({ createAPI, utils });
})();
