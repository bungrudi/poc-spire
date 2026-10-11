'use strict';

const scenarios = {
  approved: {
    outcome: 'success', title: 'Go trusted caller', binary: '/trusted-caller', credential: 'Own credential in memory',
    credentialEdge: '3. Return SVID', agentVerdict: '2. All registration conditions match',
    transferEdge: '4. Direct mTLS · POST /transfer · 200',
    mobileCredential: 'Own SVID + key + trust bundle', mobileAttestation: 'Every registration condition matches',
    resultTitle: 'Attested and authorized.', resultDescription: 'The approved executable in the approved deployment receives /caller/trusted. Money permits this exact identity to transfer.', pill: 'HTTP 200',
    expectedBinary: 'trusted-caller hash', actualBinary: 'trusted-caller hash', expectedRole: 'demo-trusted', actualRole: 'demo-trusted',
    accessible: 'Approved Go caller: all registration conditions match. SPIRE supplies an SVID and private key. Direct mutual TLS authenticates the caller, and Money authorizes POST /transfer with HTTP 200.'
  },
  'wrong-executable': {
    outcome: 'denied', title: 'Different Go executable', binary: '/probe', credential: 'No identity issued',
    credentialEdge: '3. No matching SVID', agentVerdict: '2. Executable SHA-256 does not match',
    transferEdge: '4. No client certificate · TLS rejected',
    mobileCredential: 'PermissionDenied: no identity issued', mobileAttestation: 'Executable SHA-256 does not match',
    resultTitle: 'Same deployment. Different executable.', resultDescription: 'UID, image, and deployment match. The executable hash differs, so SPIRE grants no credential. A no-client-certificate probe fails TLS before Money handles HTTP.', pill: 'NO HTTP STATUS',
    expectedBinary: 'trusted-caller hash', actualBinary: 'different binary hash', expectedRole: 'demo-trusted', actualRole: 'demo-trusted',
    accessible: 'Wrong executable: the UID and Docker deployment match, but the executable hash differs. SPIRE returns no matching identity. A direct connection without a client certificate fails TLS; Money returns no HTTP status.'
  },
  'wrong-deployment': {
    outcome: 'denied', title: 'Copied approved binary', binary: '/trusted-caller', credential: 'No identity issued',
    credentialEdge: '3. No matching SVID', agentVerdict: '2. Docker deployment role does not match',
    transferEdge: '4. No client certificate · TLS rejected',
    mobileCredential: 'PermissionDenied: no identity issued', mobileAttestation: 'Docker deployment role does not match',
    resultTitle: 'Same binary. Different deployment.', resultDescription: 'The executable hash, UID, and image match. The deployment role differs, so copying the approved binary does not grant its identity. The no-certificate probe fails TLS.', pill: 'NO HTTP STATUS',
    expectedBinary: 'trusted-caller hash', actualBinary: 'trusted-caller hash', expectedRole: 'demo-trusted', actualRole: 'demo-unidentified',
    accessible: 'Wrong deployment: the approved executable, UID, and image match, but the Docker role differs. SPIRE returns no matching identity. The no-certificate probe fails TLS before any Money HTTP response.'
  },
  observer: {
    outcome: 'forbidden', title: 'Go UI observer', binary: '/ui', credential: 'Own /ui SVID in memory',
    credentialEdge: '3. Return /ui SVID', agentVerdict: '2. Observer registration conditions match',
    transferEdge: '4. Valid mTLS · POST /transfer · 403',
    mobileCredential: 'Own /ui SVID + key + trust bundle', mobileAttestation: 'The process matches its own observer registration',
    resultTitle: 'Authenticated. Transfer denied.', resultDescription: 'The UI receives its own valid /ui identity and completes mTLS. Money permits it to read balances, but denies POST /transfer with HTTP 403.', pill: 'HTTP 403',
    expectedBinary: 'ui binary hash', actualBinary: 'ui binary hash', expectedRole: 'demo-ui', actualRole: 'demo-ui',
    accessible: 'Valid observer identity: the UI matches its own registration and receives a valid /ui SVID. Mutual TLS succeeds, but Money denies POST /transfer with HTTP 403. The UI identity is allowed only to read balances.'
  }
};

function setText(slot, value) {
  document.querySelectorAll(`[data-slot="${slot}"]`).forEach(element => { element.textContent = value; });
}

function renderScenario(key) {
  const scenario = scenarios[key];
  if (!scenario) return;
  document.querySelectorAll('[data-scenario]').forEach(button => {
    button.setAttribute('aria-pressed', String(button.dataset.scenario === key));
  });
  document.querySelector('.diagram-surface').dataset.outcome = scenario.outcome;
  document.querySelector('.scenario-result').dataset.outcome = scenario.outcome;
  document.querySelector('.result-mark').textContent = scenario.outcome === 'success' ? '✓' : scenario.outcome === 'denied' ? '×' : '↳';
  const slots = {'process-title':'title','process-binary':'binary','process-credential':'credential','credential-edge':'credentialEdge','agent-verdict':'agentVerdict','transfer-edge':'transferEdge','mobile-credential':'mobileCredential','mobile-attestation':'mobileAttestation','result-title':'resultTitle','result-description':'resultDescription','result-pill':'pill'};
  Object.entries(slots).forEach(([slot, field]) => setText(slot, scenario[field]));
  document.querySelectorAll('[data-diagram]').forEach(diagram => {
    diagram.hidden = diagram.dataset.diagram !== key;
  });
  document.getElementById('mermaid-diagram').setAttribute('aria-label', scenario.accessible);
  document.getElementById('mermaid-source').textContent =
    document.getElementById(`mermaid-definition-${key}`).textContent;
  document.getElementById('download-mermaid').href = `diagrams/${key}.mmd`;

  const rows = [
    ['Agent parent', 'Exact attested Agent', 'Exact attested Agent'],
    ['Unix UID', '1000', '1000'],
    ['Executable SHA-256', scenario.expectedBinary, scenario.actualBinary],
    ['Docker role', scenario.expectedRole, scenario.actualRole],
    ['Image config digest', 'Approved image digest', 'Approved image digest']
  ];
  const body = document.getElementById('selector-rows');
  body.replaceChildren(...rows.map(([label, required, actual]) => {
    const row = document.createElement('tr');
    const heading = document.createElement('th');
    heading.scope = 'row'; heading.textContent = label;
    const expected = document.createElement('td'); expected.textContent = required;
    const observed = document.createElement('td');
    const matched = required === actual;
    observed.className = matched ? 'selector-match' : 'selector-mismatch';
    const value = document.createElement('span'); value.textContent = actual;
    const verdict = document.createElement('span'); verdict.className = 'match-label';
    verdict.textContent = matched ? '✓ MATCH' : '× MISMATCH';
    observed.append(value, verdict); row.append(heading, expected, observed); return row;
  }));
}

document.querySelectorAll('[data-scenario]').forEach(button => {
  button.addEventListener('click', () => renderScenario(button.dataset.scenario));
});
document.querySelectorAll('[data-copy-target]').forEach(button => {
  const originalLabel = button.textContent;
  button.addEventListener('click', async () => {
    const content = document.getElementById(button.dataset.copyTarget);
    const status = document.getElementById('copy-status');
    try {
      await navigator.clipboard.writeText(content.textContent);
      button.textContent = 'Copied';
      status.textContent = 'Source copied to clipboard.';
    } catch {
      const range = document.createRange(); range.selectNodeContents(content);
      const selection = window.getSelection(); selection.removeAllRanges(); selection.addRange(range);
      button.textContent = 'Source selected';
      status.textContent = 'Source selected. Use your browser’s copy command.';
    }
    setTimeout(() => { button.textContent = originalLabel; }, 2000);
  });
});
