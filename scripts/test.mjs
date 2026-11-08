import {spawnSync} from 'node:child_process';
const go=process.env.MYGO_GO||'go';
for(const args of [['run','build:ui']]){const r=spawnSync('npm',args,{stdio:'inherit'});if(r.error)throw r.error;if(r.status)process.exit(r.status)}
const result=spawnSync(go,['test','./...'],{stdio:'inherit'});if(result.error)throw result.error;process.exit(result.status??1);
