import {spawnSync} from 'node:child_process';
import {readdirSync} from 'node:fs';
const go=process.env.MYGO_GO||'go';
for(const args of [['run','build:ui']]){const r=spawnSync('npm',args,{stdio:'inherit'});if(r.error)throw r.error;if(r.status)process.exit(r.status)}
const tests=readdirSync('ui').filter(name=>name.endsWith('.test.js')).map(name=>`ui/${name}`);
if(tests.length){const r=spawnSync(process.execPath,['--test',...tests],{stdio:'inherit'});if(r.error)throw r.error;if(r.status)process.exit(r.status)}
const result=spawnSync(go,['test','./...'],{stdio:'inherit'});if(result.error)throw result.error;process.exit(result.status??1);
