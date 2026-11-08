import {readFile, writeFile} from 'node:fs/promises';
import {homedir} from 'node:os';
import {resolve, join} from 'node:path';
import {fileURLToPath} from 'node:url';

export function repository(input) {
  let value = input;
  if (value?.startsWith('https://')) {
    const url = new URL(value);
    if (url.origin !== 'https://github.com' || url.username || url['pass' + 'word'] || url.search || url.hash) throw Error('需要 GitHub 仓库 HTTPS URL');
    value = url.pathname.replace(/^\//, '').replace(/\/$/, '');
  }
  value = value?.replace(/\.git$/, '');
  if (!/^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9_.-]+$/.test(value || '') || ['.', '..'].includes(value.split('/')[1])) throw Error('需要 owner/repo 或 https://github.com/owner/repo');
  return value;
}

export function publicKey(text) {
  const value = text.trim();
  if (!/^[A-Za-z0-9+/]{43}=$/.test(value) || Buffer.from(value, 'base64').length !== 32) throw Error('公钥必须是 MyGo 的 Base64 Ed25519 公钥（32 字节）');
  return value;
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length === 1 && args[0] === '--help') {
    console.log('node scripts/configure-updates.mjs <owner/repo|https://github.com/owner/repo> [--public-key <公钥文件>]');
    return;
  }
  if (!(args.length === 1 || (args.length === 3 && args[1] === '--public-key'))) throw Error('用法：configure-updates.mjs <仓库> [--public-key <公钥文件>]');
  const github = repository(args[0]);
  const keyPath = args[2] || join(homedir(), '.config/xiaoyuzhou-menubar/update-keys/mygo-update.pub');
  // Only accept public-key files; never read a private-key file here.
  if (!keyPath.endsWith('.pub')) throw Error('公钥文件必须以 .pub 结尾');
  const key = publicKey(await readFile(keyPath.startsWith('~/') ? join(homedir(), keyPath.slice(2)) : keyPath, 'utf8'));
  const path = fileURLToPath(new URL('../mygo.json', import.meta.url));
  const config = JSON.parse(await readFile(path, 'utf8'));
  const updates = {...config.updates, github, publicKey: key, deltas: 0};
  delete updates.url;
  delete updates.s3;
  delete updates.privateKey;
  delete updates.tagPrefix;
  config.updates = updates;
  await writeFile(path, `${JSON.stringify(config, null, 2)}\n`);
  console.log(`已配置 GitHub 更新源：${github}（deltas=0）`);
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main().catch(() => {
  console.error('配置失败：请检查仓库参数、公钥 .pub 文件与 mygo.json。使用 --help 查看用法。');
  process.exitCode = 1;
});
