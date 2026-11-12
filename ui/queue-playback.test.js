import test from 'node:test';
import assert from 'node:assert/strict';
import {episodeFirst,removeQueueEpisode} from './queue-playback.js';

const a={eid:'a'},b={eid:'b'},c={eid:'c'},d={eid:'d'};

test('playing a middle episode moves it first and preserves every other episode in order',()=>{
  const queue=[a,b,c,d];
  assert.deepEqual(episodeFirst(queue,c),[c,a,b,d]);
  assert.deepEqual(queue,[a,b,c,d]);
  assert.deepEqual(episodeFirst(queue,{...c,title:'updated'}),[{...c,title:'updated'},a,b,d]);
  assert.deepEqual(episodeFirst(queue,{eid:'new'}),[{eid:'new'},a,b,c,d]);
  assert.equal(episodeFirst(queue,a),queue);
});

function fixture(queue,currentEid){
  const events=[];
  const state={queue,index:queue.findIndex(ep=>ep.eid===currentEid),currentEid};
  return {state,events,actions:{queue,currentEid,
    stop(){events.push('stop');state.currentEid=null},
    playAt(list,index){events.push(`play:${list[index].eid}`);state.queue=episodeFirst(list,list[index]);state.index=0;state.currentEid=list[index].eid},
    setQueue(list){state.queue=list},setIndex(index){state.index=index}
  }};
}

test('removing the playing episode stops it before advancing and promotes its successor',()=>{
  for(const [queue,eid,next,expected] of [
    [[a,b,c],'a','b',[b,c]],
    [[a,b,c,d],'b','c',[c,a,d]], // dragged current item: follow its identity, not index 0
    [[a,b,c],'c','a',[a,b]],
  ]){
    const f=fixture(queue,eid);
    removeQueueEpisode(f.actions,eid);
    assert.deepEqual(f.events,['stop',`play:${next}`]);
    assert.deepEqual(f.state.queue,expected);
    assert.equal(f.state.index,0);
    assert.equal(f.state.currentEid,next);
  }
});

test('removing the only episode stops playback and empties the queue',()=>{
  const f=fixture([a],'a');
  removeQueueEpisode(f.actions,'a');
  assert.deepEqual(f.events,['stop']);
  assert.deepEqual(f.state,{queue:[],index:-1,currentEid:null});
});

test('removing another episode leaves playback untouched and keeps its index accurate',()=>{
  const f=fixture([a,b,c],'b');
  removeQueueEpisode(f.actions,'a');
  assert.deepEqual(f.events,[]);
  assert.deepEqual(f.state,{queue:[b,c],index:0,currentEid:'b'});
  removeQueueEpisode(f.actions,'missing');
  assert.deepEqual(f.events,[]);
});
