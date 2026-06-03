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
    initCurrentDomain();
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

function domainFromUrl(url) {
  try { return new URL(url).hostname.replace(/^www\./, ''); } catch { return ''; }
}

async function initCurrentDomain() {
  const el = $('currentDomain');
  const nameEl = $('currentDomainName');
  const btnEl = $('toggleDomainBtn');

  try {
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const url = tabs[0]?.url;
    if (!url || url.startsWith('chrome://') || url.startsWith('brave://')) {
      el.style.display = 'none';
      return;
    }
    const domain = domainFromUrl(url);
    if (!domain) { el.style.display = 'none'; return; }

    el.style.display = 'flex';
    const exists = allowlist.includes(domain);
    nameEl.textContent = '🌐 ' + domain;
    btnEl.style.display = 'inline-block';

    if (exists) {
      btnEl.textContent = 'Supprimer';
      btnEl.style.background = '#ef4444';
      btnEl.style.color = '#fff';
      btnEl.onclick = async () => {
        if (!confirm(`Retirer ${domain} de l'allowlist ?`)) return;
        allowlist = allowlist.filter(d => d !== domain);
        await saveAllowlist();
        render();
        initCurrentDomain();
      };
    } else {
      btnEl.textContent = 'Ajouter';
      btnEl.style.background = '#0284c7';
      btnEl.style.color = '#fff';
      btnEl.onclick = async () => {
        allowlist.push(domain);
        await saveAllowlist();
        render();
        initCurrentDomain();
      };
    }
  } catch {
    el.style.display = 'none';
  }
}

async function saveAllowlist() {
  try {
    await fetch(BRIDGE + '/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ allowlist }),
    });
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
  const d = input.value.trim().toLowerCase().replace(/^https?:\/\//, '').split('/')[0].replace(/^www\./, '');
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
    await fetch(BRIDGE + '/sync-now', { method: 'POST' });
    status('Sync effectuée', true);
  } catch {
    status('Sync déclenchée (vérifie VPS)');
  }
};

loadAllowlist();
