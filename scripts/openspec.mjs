// Run the locked development tool without reading or changing user-wide config.
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const [major, minor] = process.versions.node.split('.').map(Number);
if (major < 20 || (major === 20 && minor < 19)) {
  console.error('OpenSpec requires Node >=20.19.0; select an existing suitable runtime before running npm ci.');
  process.exit(1);
}
const cli = join(root, 'node_modules/@fission-ai/openspec/bin/openspec.js');
if (!existsSync(cli)) {
  console.error('Project-local OpenSpec is missing. Run npm ci with Node >=20.19.0.');
  process.exit(1);
}
const args = process.argv.slice(2);
if (args[0] === 'setup') {
  args.splice(0, 1, 'init', '--tools', 'codex', '--profile', 'custom', '--no-animation');
}
const temporary = mkdtempSync(join(tmpdir(), 'poc-spire-openspec-'));
try {
  const config = join(temporary, 'config/openspec');
  mkdirSync(config, { recursive: true });
  writeFileSync(join(config, 'config.json'), JSON.stringify({
    profile: 'custom',
    delivery: 'skills',
    workflows: ['explore', 'propose', 'apply', 'update', 'sync', 'verify', 'archive'],
    telemetry: { enabled: false },
  }));
  const result = spawnSync(process.execPath, [cli, ...args], {
    cwd: root,
    stdio: 'inherit',
    env: {
      ...process.env,
      XDG_CONFIG_HOME: join(temporary, 'config'),
      XDG_DATA_HOME: join(temporary, 'data'),
      OPENSPEC_TELEMETRY: '0',
      DO_NOT_TRACK: '1',
      OPENSPEC_NO_UPDATE_CHECK: '1',
    },
  });
  if (result.error) console.error(result.error.message);
  process.exitCode = result.status ?? 1;
} finally {
  rmSync(temporary, { recursive: true, force: true });
}
