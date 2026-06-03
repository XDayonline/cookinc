const BRIDGE = 'http://127.0.0.1:19999';
const $ = id => document.getElementById(id);

let allowlist = [];

function status(msg, ok) {
  const el = $('statusBar');
  el.textContent = msg;
  el.className = 'status-bar' + (ok ? ' ok' : ' err');
}

async function loadAllowlist() {
  try {
    const r = await fetch(BRIDGE + '/config');
    const cfg = await r.json();
    allowlist = cfg.allowlist || [];
    status(`${allowlist.length} domaines`, true);
    render();
  } catch {
    status('Bridge injoignable sur :19999');
    allowlist = [];
    render();
  }
}

function render() {
  const list = $('domainList');
  list.innerHTML = '';
  for (const d of allowlist) {
    const row = document.createElement('div');
    row.className = 'domain-row';
    row.innerHTML = `<span class="name">${esc(d)}</span>
      <button class="remove" data-domain="${esc(d)}">&times;</button>`;
    row.querySelector('.remove').onclick = () => removeDomain(d);
    list.appendChild(row);
  }
}

function esc(s) { return s.replace(/[&<>"]/g, ''); }

async function saveAllowlist() {
  try {
    const r = await fetch(BRIDGE + '/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ allowlist }),
    });
    if (!r.ok) throw new Error(await r.text());
    status('Allowlist mise à jour', true);
  } catch (e) {
    status('Erreur: ' + e.message);
  }
}

async function removeDomain(domain) {
  allowlist = allowlist.filter(d => d !== domain);
  render();
  await saveAllowlist();
}

$('addBtn').onclick = async () => {
  const input = $('newDomain');
  const d = input.value.trim().toLowerCase().replace(/^https?:\/\//, '').split('/')[0];
  if (!d) { status('Entrez un domaine'); return; }
  if (allowlist.includes(d)) { status('Déjà présent'); return; }
  allowlist.push(d);
  input.value = '';
  render();
  await saveAllowlist();
};

$('newDomain').onkeydown = e => { if (e.key === 'Enter') $('addBtn').click(); };
$('refreshBtn').onclick = loadAllowlist;

$('syncNow').onclick = async () => {
  status('Sync déclenchée...');
  try {
    // Trigger the background service worker to push
    await fetch(BRIDGE + '/sync-now', { method: 'POST' });
    status('Sync effectuée', true);
  } catch {
    status('Sync déclenchée (vérifie VPS)');
  }
};

loadAllowlist();
