import React from 'react';
import TestRenderer,{act} from 'react-test-renderer';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {QueueAdd} from './queue-add.js';

test('click appends; long press inserts next without also appending; movement cancels',async()=>{
  const calls=[];
  let view;
  act(()=>{view=TestRenderer.create(React.createElement(QueueAdd,{episode:{eid:'episode'},player:{queue:[],enqueue:(_ep,where)=>calls.push(where)}}))});
  const button=()=>view.root.findByType('button');
  const event={button:0,clientX:10,clientY:10,detail:1,stopPropagation(){}};
  await act(async()=>button().props.onClick(event));
  assert.deepEqual(calls,['end']);
  act(()=>button().props.onPointerDown(event));
  await act(async()=>{await new Promise(resolve=>setTimeout(resolve,550))});
  await act(async()=>{button().props.onPointerUp(event);button().props.onClick(event)});
  assert.deepEqual(calls,['end','next']);
  act(()=>{button().props.onPointerDown(event);button().props.onPointerMove({...event,clientX:30})});
  await act(async()=>{await new Promise(resolve=>setTimeout(resolve,550));button().props.onPointerUp(event);button().props.onClick(event)});
  assert.deepEqual(calls,['end','next']);
  act(()=>view.unmount());
});
