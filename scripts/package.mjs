// Keep exactly one distributable DMG after a successful, verified build.
import {spawnSync} from 'node:child_process';
import {readFile,readdir,copyFile,unlink,mkdir,rm,writeFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {join} from 'node:path';
const root=fileURLToPath(new URL('..',import.meta.url));
const outputs=join(root,'release');
await mkdir(outputs,{recursive:true});
const go=process.env.MYGO_GO||'go';
const buildEnv={...process.env};
if(buildEnv.MYGO_UPDATER_PRIVATE_KEY_FILE){if(buildEnv.MYGO_UPDATER_PRIVATE_KEY)throw Error('签名密钥来源只能设置一个');buildEnv.MYGO_UPDATER_PRIVATE_KEY=(await readFile(buildEnv.MYGO_UPDATER_PRIVATE_KEY_FILE,'utf8')).trim();delete buildEnv.MYGO_UPDATER_PRIVATE_KEY_FILE;}
function run(cmd,args){const p=spawnSync(cmd,args,{cwd:root,stdio:'inherit',env:buildEnv});if(p.error)throw p.error;if(p.status!==0)throw Error(`${cmd} failed (${p.status})`)}
run(go,['run','./scripts/icon']);
run('npm',['run','build:ui']);
// Build from a clean staging directory.
await rm(join(root,'resources/bin'),{recursive:true,force:true});
if(process.platform==='darwin'){
 const shareApp=join(root,'resources/bin/小宇宙分享助手.app');
 await mkdir(join(shareApp,'Contents/MacOS'),{recursive:true});
 await writeFile(join(shareApp,'Contents/Info.plist'),`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>dev.sgaxun.xiaoyuzhou-player.share</string><key>CFBundleName</key><string>小宇宙分享助手</string><key>CFBundleExecutable</key><string>share-helper</string><key>CFBundlePackageType</key><string>APPL</string><key>LSUIElement</key><true/><key>LSMinimumSystemVersion</key><string>12.0</string></dict></plist>`);
 run('xcrun',['swiftc','-O','-target','arm64-apple-macosx12.0','native/share.swift','-o',join(shareApp,'Contents/MacOS/share-helper')]);
 run('codesign',['--force','--sign','-',shareApp]);
}
await rm(join(root,'build'),{recursive:true,force:true});
run(go,['run','github.com/egoist/mygo/cmd/mygo@v0.8.3','build','-skip-notarize']);
const {version,slug}=JSON.parse(await readFile(join(root,'mygo.json'),'utf8'));
const build=join(root,'build/darwin-arm64');
const name=`小宇宙播放器-${version}-AppleSilicon.dmg`;
const source=join(build,`${slug}-${version}-arm64.dmg`);
const target=join(outputs,name);
await copyFile(source,target);
run('hdiutil',['verify',target]);
const managed=[name];
for(const file of await readdir(build)){
 if(/^update-darwin-arm64\.json$/.test(file)||(/^(?:xiaoyuzhou-player|xiao-yu-zhou-cai-dan-lan)-/.test(file)&&file.endsWith('-darwin-arm64.tar.gz'))){await copyFile(join(build,file),join(outputs,file));managed.push(file)}
}
for(const file of await readdir(outputs)){
 if((/^update-darwin-arm64\.json$/.test(file)||(/^(?:xiaoyuzhou-player|xiao-yu-zhou-cai-dan-lan)-/.test(file)&&file.endsWith('-darwin-arm64.tar.gz')))&&!managed.includes(file))await unlink(join(outputs,file));
}

for(const file of await readdir(outputs))if(/^小宇宙(?:播放器|菜单栏)-.*\.dmg$/u.test(file)&&file!==name)await unlink(join(outputs,file));
for(const file of await readdir(build))if(/^(?:xiaoyuzhou-player|xiao-yu-zhou-cai-dan-lan)-.*\.dmg$/.test(file))await unlink(join(build,file));
console.log(`仅保留最新 DMG：${target}`);
