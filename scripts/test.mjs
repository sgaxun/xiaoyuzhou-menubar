import {spawnSync} from 'node:child_process';
import {readdirSync} from 'node:fs';
const go=process.env.MYGO_GO||'go';
for(const args of [['run','build:ui']]){const r=spawnSync(process.execPath,[process.env.npm_execpath||(()=>{throw Error('Run through npm test')})(),...args],{stdio:'inherit'});if(r.error)throw r.error;if(r.status!==0)process.exit(r.status??1)}
const tests=readdirSync('ui').filter(name=>name.endsWith('.test.js')).map(name=>`ui/${name}`);
if(tests.length){const r=spawnSync(process.execPath,['--test',...tests],{stdio:'inherit'});if(r.error)throw r.error;if(r.status!==0)process.exit(r.status??1)}
const result=spawnSync(go,['test','./...'],{stdio:'inherit'});if(result.error)throw result.error;process.exit(result.status??1);
