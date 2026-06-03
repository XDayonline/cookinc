// Cookinc Bridge — Chrome Extension Service Worker
// Reads cookies and pushes them to cookinc.exe on localhost:19999
// chrome.cookies API handles all decryption (including App-Bound Encryption)

const SINK_URL = 'http://localhost:19999';

let allowlist = [];
let intervalMs = 5000;

function loadConfig() {
  return fetch(SINK_URL + '/config')
    .then(r => r.json())
    .then(cfg => {
      allowlist = cfg.allowlist || [];
      intervalMs = (cfg.interval || 5) * 1000;
      console.log('[cookinc] config loaded:', allowlist, intervalMs + 'ms');
    })
    .catch(e => console.warn('[cookinc] config not available, retrying...'));
}

function getDomainVariations(domain) {
  return [domain, '.' + domain];
}

async function pushCookies() {
  if (!allowlist.length) {
    await loadConfig();
    return;
  }

  const allCookies = await chrome.cookies.getAll({});
  const matched = allCookies.filter(c =>
    allowlist.some(allowed =>
      c.domain === allowed || c.domain === '.' + allowed || c.domain.endsWith('.' + allowed)
    )
  );

  if (!matched.length) {
    console.log('[cookinc] no matching cookies for:', allowlist);
    return;
  }

  console.log('[cookinc] pushing', matched.length, 'cookies...');
  try {
    const resp = await fetch(SINK_URL + '/cookies', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(matched),
    });
    const result = await resp.json();
    console.log('[cookinc] done:', result.status, result.count, 'cookies');
  } catch (e) {
    console.warn('[cookinc] push failed:', e.message);
  }
}

// Start
loadConfig().then(() => {
  pushCookies();
  setInterval(pushCookies, intervalMs);
});
