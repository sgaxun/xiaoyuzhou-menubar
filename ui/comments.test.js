import test from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import TestRenderer,{act} from 'react-test-renderer';
import {Comments} from './comments.js';
const h=React.createElement;
const episode={eid:'episode-a',duration:1000};
const player={current:null,queue:[]};
const comment={id:'comment-a',text:'首条评论',author:{nickname:'听众'},createdAt:'2026-11-10T00:00:00Z',threadReplyCount:1};
const findButton=(root,text)=>root.findAllByType('button').find(node=>node.children.join('')===text);
async function click(root,text){const button=findButton(root,text);assert.ok(button,`missing button: ${text}`);await act(async()=>button.props.onClick());}

test('visited sorts, replies, pagination and scroll survive hide/show without refetching',async()=>{
 const requests=[],viewports=[];
 const call=async(action,body)=>{
  assert.equal(action,'comments');requests.push(body);
  if(body.thread)return {ok:true,data:[{...comment,id:'reply-a',text:'回复内容',threadReplyCount:0}]};
  if(body.loadMoreKey)return {ok:true,data:[{...comment,id:'comment-b',text:'下一页'}],loadMoreKey:null};
  return {ok:true,data:[comment],loadMoreKey:body.order==='HOT'?{id:'next'}:null};
 };
 const props={episode,player,call};let tree;
 await act(async()=>{tree=TestRenderer.create(h(Comments,props),{createNodeMock:element=>{
  const node={scrollTop:0};if(element.props.className==='xyz-comments-page')viewports.push(node);return node;
 }})});
 assert.equal(requests.length,1);
 await click(tree.root,'加载更多');assert.equal(requests.length,2);
 await click(tree.root,'查看 1 条回复');assert.equal(requests.length,3);
 await click(tree.root,'收起回复');await click(tree.root,'查看 1 条回复');assert.equal(requests.length,3);
 const page=tree.root.findAllByProps({className:'xyz-comments-page'})[0];
 viewports[0].scrollTop=160;page.props.onScroll();
 await click(tree.root,'最新');assert.equal(requests.length,4);
 viewports[0].scrollTop=0;
 await click(tree.root,'全部');assert.equal(requests.length,4);assert.equal(viewports[0].scrollTop,160);
 assert.ok(tree.root.findAllByType('p').some(node=>node.children.includes('下一页')));
 await act(async()=>tree.update(h(Comments,{...props,active:false})));
 viewports[0].scrollTop=0;
 await act(async()=>tree.update(h(Comments,{...props,active:true})));
 assert.equal(requests.length,4);assert.equal(viewports[0].scrollTop,160);
 await click(tree.root,'时点');assert.equal(requests.length,5);assert.equal(requests.at(-1).order,'TIMESTAMP');
 await act(async()=>tree.unmount());
});

test('an in-flight page finishing while hidden is retained and never requested twice',async()=>{
 let resolve,requests=0,tree;
 const call=()=>{requests++;return new Promise(r=>{resolve=r})};
 const props={episode,player,call};
 await act(async()=>{tree=TestRenderer.create(h(Comments,props))});
 await act(async()=>tree.update(h(Comments,{...props,active:false})));
 await act(async()=>{resolve({ok:true,data:[comment]});await Promise.resolve()});
 await act(async()=>tree.update(h(Comments,{...props,active:true})));
 assert.equal(requests,1);assert.ok(tree.root.findAllByType('p').some(node=>node.children.includes('首条评论')));
 await act(async()=>tree.unmount());
});

test('reply failures retry only on demand without discarding the main list',async()=>{
 let attempts=0,tree;
 const call=async(action,body)=>{
  if(!body.thread)return {ok:true,data:[comment]};
  if(++attempts===1)throw Error('temporary failure');
  return {ok:true,data:[]};
 };
 await act(async()=>{tree=TestRenderer.create(h(Comments,{episode,player,call}))});
 await click(tree.root,'查看 1 条回复');assert.equal(attempts,1);
 await click(tree.root,'收起回复');await click(tree.root,'查看 1 条回复');assert.equal(attempts,1);
 await click(tree.root,'重试');assert.equal(attempts,2);
 assert.ok(tree.root.findAllByType('p').some(node=>node.children.includes('首条评论')));
 await act(async()=>tree.unmount());
});
