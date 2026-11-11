import test from 'node:test';
import assert from 'node:assert/strict';
import {createPlaybackReporter} from './cloud-sync.js';

test('only genuine playback creates tasks; pause duplicates are suppressed and resume permits a new pause',async()=>{
  const calls=[];
  const report=createPlaybackReporter(async(action,body)=>calls.push({action,body}));
  const episode={eid:'episode-a',pid:'podcast-a',title:'test'};
  await report('paused',episode,12);
  assert.equal(calls.length,0);
  await report('playing',episode,12);
  await report('paused',episode,20);
  await report('paused',episode,20);
  await report('playing',episode,20);
  await report('paused',episode,30);
  await report('ended',episode,50);
  await report('paused',episode,50);
  assert.deepEqual(calls.map(x=>x.body.event),['playing','paused','playing','paused','ended']);
  assert.equal(calls.at(-1).body.position,50);
  assert.ok(calls.every(x=>x.action==='playbackEvent'&&x.body.playedAt));
});

test('bridge events remain ordered even when earlier calls are unresolved',async()=>{
  const events=[];let release;
  const gate=new Promise(resolve=>{release=resolve});
  const report=createPlaybackReporter(async(_,body)=>{events.push(body.event);if(body.event==='playing')await gate});
  const ep={eid:'a'};
  const first=report('playing',ep,0),second=report('paused',ep,10);
  await Promise.resolve();await Promise.resolve();
  assert.deepEqual(events,['playing']);
  release();await Promise.all([first,second]);
  assert.deepEqual(events,['playing','paused']);
});

test('manual cloud pull updates the queue and keeps current media untouched',async()=>{
  const React=await import('react');
  const {default:Renderer,act}=await import('react-test-renderer');
  const {PullCloudPlaylist}=await import('./cloud-sync.js');
  const old={eid:'old',title:'local'},cloud={eid:'cloud',title:'remote'};
  const calls=[];let queue,index;
  const player={queue:[old],current:old,flushLocalData:async pause=>assert.equal(pause,false),setQueue:q=>queue=q,setIndex:i=>index=i};
  const call=async(action,body)=>{calls.push({action,body});return action==='pullCloudPlaylist'?{queue:[cloud,old]}:{ok:true}};
  let tree;
  await act(async()=>{tree=Renderer.create(React.createElement(PullCloudPlaylist,{call,player,loggedIn:true}))});
  await act(async()=>{await tree.root.findByType('button').props.onClick()});
  assert.deepEqual(queue,[cloud,old]);assert.equal(index,1);
  assert.deepEqual(calls.map(x=>x.action),['pullCloudPlaylist','saveQueue']);
  assert.equal(player.current,old);
  await act(async()=>tree.unmount());
});

test('an edit or completion during cloud pull wins over the late cloud result',async()=>{
  const React=await import('react');
  const {default:Renderer,act}=await import('react-test-renderer');
  const {PullCloudPlaylist}=await import('./cloud-sync.js');
  for(const changeCurrent of [false,true]){
    const old={eid:'old'},cloud={eid:'cloud'};
    let resolvePull,changed=0;const actions=[];
    const player={queue:[old],current:old,flushLocalData:async()=>{},setQueue:()=>changed++,setIndex:()=>changed++};
    const call=async action=>{actions.push(action);if(action==='pullCloudPlaylist')return new Promise(resolve=>resolvePull=resolve);return {ok:true}};
    let tree,pending;
    await act(async()=>{tree=Renderer.create(React.createElement(PullCloudPlaylist,{call,player,loggedIn:true}))});
    await act(async()=>{pending=tree.root.findByType('button').props.onClick();await Promise.resolve()});
    if(changeCurrent)player.current=cloud;else player.queue=[];
    await act(async()=>{resolvePull({queue:[cloud,old]});await pending});
    assert.equal(changed,0);assert.deepEqual(actions,['pullCloudPlaylist']);
    assert.match(JSON.stringify(tree.toJSON()),/本次未合并/);
    await act(async()=>tree.unmount());
  }
});

test('settings separates waiting and failure and only offers retry for failures',async()=>{
  const React=await import('react');
  const {default:Renderer,act}=await import('react-test-renderer');
  const {CloudSyncSettings}=await import('./cloud-sync.js');
  const previous=globalThis.window;
  globalThis.window={addEventListener(){},removeEventListener(){}};
  try{
    for(const failed of [0,2]){
      const call=async()=>({state:{cloudSync:{enabled:true,waiting:1,failed,blocked:!!failed,pending:1+failed}}});
      let tree;
      await act(async()=>{tree=Renderer.create(React.createElement(CloudSyncSettings,{call,loggedIn:true}))});
      const text=JSON.stringify(tree.toJSON());
      assert.match(text,/等待上传 1 项/);
      assert.equal(text.includes('重试失败记录'),!!failed);
      assert.equal(text.includes('自动上传已暂停'),!!failed);
      await act(async()=>tree.unmount());
    }
  }finally{globalThis.window=previous}
});
