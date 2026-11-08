import {build} from 'esbuild';
import {writeFile} from 'node:fs/promises';
const output=await build({entryPoints:['ui/player.js'],bundle:true,write:false,format:'iife',platform:'browser',minify:true,target:['safari16.4'],define:{'process.env.NODE_ENV':'"production"'}});
const js=output.outputFiles[0].text.replaceAll('</script','<\\/script');
await writeFile('assets/player.html',`<!doctype html><html lang="zh-CN"><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>小宇宙播放器</title><main id="root"></main><script>${js}</script></html>`);
