import {readFile, readdir, stat, mkdtemp, copyFile, rm} from 'node:fs/promises';
import {spawnSync} from 'node:child_process';
import {createHash, createPublicKey, verify} from 'node:crypto';
import {homedir, tmpdir} from 'node:os';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {repository, publicKey} from './configure-updates.mjs';

const root = fileURLToPath(new URL('..', import.meta.url));
function run(cmd, args, env, capture = false) {
  const p = spawnSync(cmd, args, {cwd: root, env, stdio: capture ? 'pipe' : 'inherit', encoding: 'utf8'});
  if (p.error || p.status !== 0) throw Error(`${cmd} 执行失败`);
  return p.stdout;
}
async function main() {
  const args = process.argv.slice(2);
  if (args.length === 1 && args[0] === '--help') {
    console.log('node scripts/release.mjs [--draft]：默认仅构建；--draft 创建草稿并上传，绝不正式发布。');
    return;
  }
  if (args.some(a => a !== '--draft') || args.length > 1) throw Error('仅支持 --draft 或 --help');
  const draft = args.includes('--draft');
  const config = JSON.parse(await readFile(join(root, 'mygo.json'), 'utf8'));
  const pkg = JSON.parse(await readFile(join(root, 'package.json'), 'utf8'));
  const version = config.version;
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(version || '') || version !== pkg.version) throw Error('mygo.json 和 package.json 需要相同的有效版本号');
  const u = config.updates;
  const repo = repository(u?.github);
  if (u.github !== repo || u.url || u.s3 || u.privateKey || (u.tagPrefix && u.tagPrefix !== 'v') || u.deltas !== 0) throw Error('请先配置 GitHub 更新源、默认 v 标签及 deltas:0，不要在配置中保存私钥');
  const pub = Buffer.from(publicKey(u.publicKey), 'base64');
  if (process.env.GITHUB_REPOSITORY && process.env.GITHUB_REPOSITORY.toLowerCase() !== repo.toLowerCase()) throw Error('GITHUB_REPOSITORY 与 updates.github 不匹配');
  const tag = `v${version}`;
  if (process.env.GITHUB_REF_TYPE === 'tag' && process.env.GITHUB_REF_NAME !== tag) throw Error('GitHub 标签与配置版本不匹配');
  if (process.env.GITHUB_ACTIONS === 'true' && process.env.GITHUB_REF_TYPE !== 'tag') throw Error('手动工作流必须选择版本标签运行');
  if (process.platform !== 'darwin' || process.arch !== 'arm64') throw Error('需要 Apple Silicon macOS（darwin-arm64）构建机');
  const env = {...process.env};
  let secret = env.MYGO_UPDATER_PRIVATE_KEY;
  if (env.MYGO_UPDATER_PRIVATE_KEY_FILE) {
    if (secret) throw Error('签名密钥文件和环境变量只能设置一个');
    const path = env.MYGO_UPDATER_PRIVATE_KEY_FILE;
    secret = await readFile(path.startsWith('~/') ? join(homedir(), path.slice(2)) : path, 'utf8');
  }
  secret = secret?.trim();
  if (!secret || !/^[A-Za-z0-9+/]+={0,2}$/.test(secret)) throw Error('缺少有效的 MyGo 签名密钥');
  const key = Buffer.from(secret, 'base64');
  if (key.length !== 64 || !key.subarray(32).equals(pub)) throw Error('签名密钥格式错误或与 updates.publicKey 不匹配');
  env.MYGO_UPDATER_PRIVATE_KEY = secret;
  delete env.MYGO_UPDATER_PRIVATE_KEY_FILE;
  // Signing material goes only into the packaging child environment, never arguments or logs.
  run('npm', ['run', 'package'], env);
  const output = join(root, 'release');
  const manifestName = 'update-darwin-arm64.json';
  const manifest = JSON.parse(await readFile(join(output, manifestName), 'utf8'));
  const url = new URL(manifest.url);
  const prefix = `https://github.com/${repo}/releases/download/${tag}/`;
  const archiveName = decodeURIComponent(url.pathname.split('/').at(-1));
  if (manifest.version !== version || !manifest.url.startsWith(prefix) || !archiveName.endsWith(`-${version}-darwin-arm64.tar.gz`) || archiveName.includes('/') || archiveName.includes('\\') || manifest.deltas?.length) throw Error('更新 manifest 的版本、URL、归档名称或 deltas 不正确');
  const archive = await readFile(join(output, archiveName));
  const keyObject = createPublicKey({key: Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), pub]), format: 'der', type: 'spki'});
  if (archive.length !== manifest.size || !verify(null, createHash('sha256').update(archive).digest(), keyObject, Buffer.from(manifest.signature || '', 'base64'))) throw Error('更新归档大小或签名校验失败');
  const dmgs = (await readdir(output)).filter(name => name.endsWith('.dmg') && name.includes(`-${version}-`));
  if (dmgs.length !== 1 || !(await stat(join(output, dmgs[0]))).size) throw Error('release 目录必须包含当前版本唯一且非空的 DMG');
  const assets = [dmgs[0], archiveName, manifestName].map(name => join(output, name));
  if (!draft) { console.log(`已验证 ${tag} 的 DMG、签名更新归档及 manifest；未进行远端写入。`); return; }
  const ghEnv = {...process.env};
  delete ghEnv.MYGO_UPDATER_PRIVATE_KEY;
  delete ghEnv.MYGO_UPDATER_PRIVATE_KEY_FILE;
  // Require an existing remote tag, including when this directory has no local .git.
  run('gh', ['api', `repos/${repo}/git/ref/tags/${tag}`], ghEnv, true);
  const existing = JSON.parse(run('gh', ['api', '--paginate', '--slurp', `repos/${repo}/releases?per_page=100`], ghEnv, true)).flat().find(r => r.tag_name === tag);
  if (existing && !existing.draft) throw Error('该版本已正式发布，拒绝覆盖');
  if (!existing) run('gh', ['release', 'create', tag, '--repo', repo, '--draft', '--verify-tag', '--title', tag, '--generate-notes', ...(version.includes('-') ? ['--prerelease'] : [])], ghEnv, true);
  const state = JSON.parse(run('gh', ['release', 'view', tag, '--repo', repo, '--json', 'isDraft'], ghEnv, true));
  if (!state.isDraft) throw Error('目标不是草稿，停止上传');
  // GitHub normalizes non-ASCII asset names. Upload an ASCII basename while
  // retaining the Chinese local DMG name; the temporary copy is always removed.
  const staging = await mkdtemp(join(tmpdir(), 'xiaoyuzhou-release-'));
  try {
    const installer = join(staging, `xiaoyuzhou-menubar-${version}-AppleSilicon.dmg`);
    await copyFile(assets[0], installer);
    assets[0] = installer;
    run('gh', ['release', 'upload', tag, ...assets, '--repo', repo, '--clobber'], ghEnv, true);
  } finally { await rm(staging, {recursive: true, force: true}); }
  console.log(`已上传 ${repo} ${tag} 草稿；请人工检查后发布。`);
}
main().catch(error => {
  // Never echo child output or key-related filesystem errors.
  const safe = /^(mygo.json|请先配置|GITHUB_|GitHub 标签|手动工作流|需要 Apple|签名密钥|缺少有效|更新 |更新归档|release 目录|该版本|目标不是|仅支持|npm 执行失败|gh 执行失败)/.test(error.message);
  console.error(safe ? error.message : '发布准备失败：请检查配置、密钥文件和构建产物（敏感详情已隐藏）。');
  process.exitCode = 1;
});
