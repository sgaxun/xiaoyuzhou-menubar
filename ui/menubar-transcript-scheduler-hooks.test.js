import test from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import {act,create} from 'react-test-renderer';
import {MenubarTranscript} from './menubar-transcript.js';

function harness(t,{enabled=false,playing=false,call=async()=>({status:'available',segments:[{startMs:0,text:'subtitle'}]})}={}){
 const saved=new Map(['window','localStorage','MutationObserver'].map(key=>[key,Object.getOwnPropertyDescriptor(globalThis,key)]));
 const win=new EventTarget(),calls=[],fetches=[],timers=new Map();let next=0,commits=0,root;
 let preference=String(enabled);
 win.mygo={call:async(...args)=>{calls.push(args)}};
 globalThis.window=win;
 globalThis.localStorage={getItem:()=>preference,setItem:(_key,value)=>{preference=value}};
 globalThis.MutationObserver=class{observe(){}disconnect(){}};
 t.mock.method(globalThis,'setInterval',fn=>{timers.set(++next,fn);return next});
 t.mock.method(globalThis,'clearInterval',id=>timers.delete(id));
 const audio=Object.assign(new EventTarget(),{dataset:{xyzEid:'a'},paused:!playing,ended:false,readyState:1,currentTime:20,duration:100});
 const player={audioRef:{current:null},current:{eid:'a',duration:100}};
 const request=url=>{fetches.push(url);return call(url)};
 const view=()=>React.createElement(React.Profiler,{id:'subtitle',onRender:()=>commits++},
  React.createElement('audio',{ref:player.audioRef}),React.createElement(MenubarTranscript,{player,call:request}));
 t.after(async()=>{
  if(root)await act(async()=>root.unmount());
  for(const [key,descriptor]of saved){if(descriptor)Object.defineProperty(globalThis,key,descriptor);else delete globalThis[key]}
 });
 return {audio,player,calls,fetches,timers,get commits(){return commits},
  async mount(){await act(async()=>{root=create(view(),{createNodeMock:()=>audio})})},
  async enable(value){await act(async()=>{preference=String(value);win.dispatchEvent(new Event('xyz-menubar-transcript'))})},
  async event(type){await act(async()=>audio.dispatchEvent(new Event(type)))},
  async render(){await act(async()=>root.update(view()))},
  async tick(){await act(async()=>{for(const fn of [...timers.values()])fn()})},
  async unmount(){await act(async()=>root.unmount());root=null},
 };
}

test('hook defaults disabled, enables mounted playing audio, and ticks without React commits',async t=>{
 const f=harness(t,{playing:true});await f.mount();
 assert.equal(f.player.audioRef.current,f.audio);assert.equal(f.fetches.length,0);assert.equal(f.timers.size,0);
 assert.ok(f.calls.some(args=>args[0]==='Backend.MenuSubtitleEnabled'&&args[1]===false));
 await f.enable(true);assert.deepEqual(f.fetches,['transcript?eid=a']);assert.equal(f.timers.size,1);
 const commits=f.commits,count=f.calls.length;
 await f.tick();assert.equal(f.commits,commits);assert.equal(f.calls.length,count);
 f.audio.currentTime=40;await f.tick();assert.equal(f.commits,commits);assert.ok(f.calls.length>count);
 f.audio.paused=true;await f.event('pause');assert.equal(f.timers.size,0);
 f.audio.currentTime=10;await f.event('seeked');assert.equal(f.calls.at(-1)[1],'subtitle');
 await f.enable(false);assert.equal(f.timers.size,0);assert.deepEqual(f.calls.at(-1),['Backend.MenuSubtitleEnabled',false]);
 await f.unmount();assert.equal(f.timers.size,0);
});

test('hook waits for actual matching playback and resumes fetching the correct eid',async t=>{
 const f=harness(t,{enabled:true});await f.mount();assert.equal(f.fetches.length,0);
 f.audio.paused=false;f.audio.dataset.xyzEid='other';await f.event('play');assert.equal(f.fetches.length,0);
 f.audio.dataset.xyzEid='a';await f.event('play');assert.deepEqual(f.fetches,['transcript?eid=a']);
 await f.enable(false);
 f.player.current={eid:'b',duration:100};f.audio.dataset.xyzEid='b';f.audio.paused=true;await f.render();
 await f.enable(true);assert.equal(f.fetches.length,1);assert.equal(f.timers.size,0);
 f.audio.paused=false;await f.event('play');assert.deepEqual(f.fetches,['transcript?eid=a','transcript?eid=b']);
 f.audio.ended=true;await f.event('ended');assert.equal(f.timers.size,0);
});

test('hook ignores disabled in-flight results and never polls unavailable transcripts',async t=>{
 let resolve;
 const f=harness(t,{enabled:true,playing:true,call:()=>new Promise(done=>{resolve=done})});
 await f.mount();assert.equal(f.fetches.length,1);assert.equal(f.timers.size,0);
 await f.enable(false);
 await act(async()=>resolve({status:'available',segments:[{startMs:0,text:'stale'}]}));
 assert.equal(f.timers.size,0);assert.ok(!f.calls.some(args=>args[1]==='stale'));
 await f.enable(true);assert.equal(f.fetches.length,2);
 await act(async()=>resolve({status:'unavailable'}));assert.equal(f.timers.size,0);
 await f.event('timeupdate');assert.equal(f.fetches.length,2);
});

test('native toggle works when embedded page storage is denied',async t=>{
 const f=harness(t,{playing:true});
 globalThis.localStorage={getItem(){throw new Error('SecurityError')},setItem(){throw new Error('SecurityError')}};
 await f.mount();
 await act(async()=>window.dispatchEvent(new Event('xyz-toggle-menubar-transcript')));
 assert.deepEqual(f.fetches,['transcript?eid=a']);
 assert.ok(f.calls.some(args=>args[0]==='Backend.MenuSubtitleEnabled'&&args[1]===true));
 await act(async()=>window.dispatchEvent(new Event('xyz-toggle-menubar-transcript')));
 assert.equal(f.calls.filter(args=>args[0]==='Backend.MenuSubtitleEnabled').at(-1)[1],false);
 assert.equal(f.timers.size,0);
});
