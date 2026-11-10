import test from 'node:test';
import assert from 'node:assert/strict';
import {createSubtitleScheduler,scrollingSubtitle} from './menubar-transcript-scheduler.js';

function fixture(options={}){
 const listeners=new Map(),timers=new Map(),starts=[],updates=[];
 let next=0,sourceChanged,disconnected=false;
 const audio={dataset:{xyzEid:'a'},paused:true,ended:false,readyState:1,currentTime:0,duration:100,
  addEventListener(name,fn){listeners.set(name,fn)},
  removeEventListener(name,fn){if(listeners.get(name)===fn)listeners.delete(name)}};
 const dispose=createSubtitleScheduler({audio,enabled:true,hasSegments:true,eid:'a',
  onStarted:id=>starts.push(id),onUpdate:a=>updates.push(a?{...scrollingSubtitle([{startMs:0,text:'first'},{startMs:50000,text:'second'}],a.currentTime,a.duration)}:null),
  setInterval(fn,ms){assert.equal(ms,100);timers.set(++next,fn);return next},
  clearInterval(id){assert.ok(timers.delete(id))},
  observeSource(a,fn){assert.equal(a,options.audio||audio);sourceChanged=fn;return()=>{disconnected=true}},...options});
 return {audio,timers,starts,updates,listeners,dispose,
  event(name){listeners.get(name)?.()},tick(){for(const fn of [...timers.values()])fn()},
  source(){sourceChanged?.()},get disconnected(){return disconnected}};
}

test('disabled has no timers, listeners, source observer, or start/fetch signal',()=>{
 const f=fixture({enabled:false});
 f.audio.paused=false;f.event('play');f.source();
 assert.equal(f.timers.size,0);assert.equal(f.listeners.size,0);
 assert.deepEqual(f.starts,[]);assert.deepEqual(f.updates,[null]);f.dispose();
});

test('paused events update immediately without a timer; play starts exactly one',()=>{
 const f=fixture();
 assert.equal(f.timers.size,0);assert.deepEqual(f.starts,[]);
 for(const event of ['seeking','seeked','timeupdate','loadedmetadata']){
  f.audio.currentTime+=10;f.event(event);
  assert.equal(f.updates.at(-1).progress,scrollingSubtitle([{startMs:0,text:'first'},{startMs:50000,text:'second'}],f.audio.currentTime,100).progress);
  assert.equal(f.timers.size,0);
 }
 f.audio.paused=false;f.event('play');f.event('playing');f.event('timeupdate');
 assert.equal(f.timers.size,1);assert.deepEqual(f.starts,['a']);
 f.tick();f.tick();assert.deepEqual(f.starts,['a']);
 f.audio.paused=true;f.event('pause');assert.equal(f.timers.size,0);
 f.audio.paused=false;f.event('play');assert.equal(f.timers.size,1);
 f.audio.ended=true;f.event('ended');assert.equal(f.timers.size,0);
 assert.deepEqual(f.starts,['a']);f.dispose();
});

test('ticks follow actual audio time including backwards seeks and frozen time',()=>{
 const f=fixture();f.audio.paused=false;f.event('play');
 f.audio.currentTime=40;f.tick();const forward=f.updates.at(-1);
 f.tick();assert.deepEqual(f.updates.at(-1),forward);
 f.audio.currentTime=10;f.event('seeking');assert.ok(f.updates.at(-1).progress<forward.progress);
 f.audio.currentTime=60;f.event('timeupdate');assert.equal(f.updates.at(-1).text,'second');
 f.dispose();
});

test('source changes clear stale subtitles and stop ticks until source matches',()=>{
 const f=fixture();f.audio.paused=false;f.event('play');
 f.audio.dataset.xyzEid='b';f.source();assert.equal(f.updates.at(-1),null);assert.equal(f.timers.size,0);
 f.audio.dataset.xyzEid='a';f.source();assert.equal(f.timers.size,1);
 delete f.audio.dataset.xyzEid;f.event('emptied');assert.equal(f.timers.size,0);assert.equal(f.updates.at(-1),null);
 f.dispose();assert.equal(f.listeners.size,0);assert.equal(f.disconnected,true);
 const count=f.updates.length;f.source();f.tick();assert.equal(f.updates.length,count);
});

test('enabling during playback detects the started eid immediately; cleanup cancels queued callbacks',()=>{
 const audio={dataset:{xyzEid:'a'},paused:false,ended:false,readyState:1,currentTime:20,duration:100,addEventListener(){},removeEventListener(){}};
 const f=fixture({audio});assert.deepEqual(f.starts,['a']);assert.equal(f.timers.size,1);
 const queued=[...f.timers.values()][0];f.dispose();f.dispose();
 assert.equal(f.timers.size,0);const count=f.updates.length;queued();assert.equal(f.updates.length,count);
});

test('subtitle selection handles gaps, invalid times and duration fallback',()=>{
 const segments=[{startMs:10000,text:'one'},{startMs:20000,text:'two'}];
 assert.deepEqual(scrollingSubtitle(segments,5,40),{text:'',progress:0});
 assert.deepEqual(scrollingSubtitle(segments,NaN,40),{text:'',progress:0});
 assert.deepEqual(scrollingSubtitle(segments,30,40),{text:'two',progress:0.5});
 assert.equal(scrollingSubtitle(segments,100,40).progress,1);
});

test('default observer watches source/eid changes and disconnects on disposal',()=>{
 const previous=globalThis.MutationObserver;
 let notify,observed,disconnects=0;
 globalThis.MutationObserver=class {
  constructor(fn){notify=fn}
  observe(audio,options){observed={audio,options}}
  disconnect(){disconnects++}
 };
 try{
  const f=fixture({observeSource:undefined});
  // undefined uses the controller's default observer rather than the fixture hook.
  assert.equal(observed.audio,f.audio);
  assert.deepEqual(observed.options,{attributes:true,attributeFilter:['src','data-xyz-eid'],childList:true});
  f.audio.paused=false;notify();assert.equal(f.timers.size,1);
  f.audio.dataset.xyzEid='other';notify();assert.equal(f.timers.size,0);
  assert.equal(f.updates.at(-1),null);
  f.dispose();f.dispose();assert.equal(disconnects,1);
  const count=f.updates.length;notify();assert.equal(f.updates.length,count);
 }finally{
  if(previous===undefined)delete globalThis.MutationObserver;
  else globalThis.MutationObserver=previous;
 }
});

test('disabling active playback removes all work and re-enabling detects the new episode',()=>{
 const f=fixture();f.audio.paused=false;f.event('play');f.dispose();
 assert.equal(f.listeners.size,0);assert.equal(f.timers.size,0);
 const disabled=fixture({audio:f.audio,enabled:false});
 assert.deepEqual(disabled.starts,[]);assert.equal(disabled.timers.size,0);disabled.dispose();
 f.audio.dataset.xyzEid='b';
 const resumed=fixture({audio:f.audio,eid:'b'});
 assert.deepEqual(resumed.starts,['b']);assert.equal(resumed.timers.size,1);resumed.dispose();
});


test('no periodic ticks before metadata or without usable transcript data',()=>{
 const f=fixture();f.audio.readyState=0;f.audio.paused=false;f.event('play');
 assert.deepEqual(f.starts,['a']);assert.equal(f.timers.size,0);
 f.audio.readyState=1;f.event('loadedmetadata');assert.equal(f.timers.size,1);
 f.audio.readyState=0;f.event('emptied');assert.equal(f.timers.size,0);f.dispose();
 const empty=fixture({hasSegments:false});empty.audio.paused=false;empty.event('play');
 assert.deepEqual(empty.starts,['a']);assert.equal(empty.timers.size,0);
 empty.audio.currentTime=60;empty.event('timeupdate');assert.equal(empty.updates.at(-1).text,'second');
 empty.dispose();
});
