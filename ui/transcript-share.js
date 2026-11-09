import React from 'react';
import {createPortal} from 'react-dom';
import {createTranscriptCard} from './transcript-card.js';
const {createElement:h,useState,useEffect,useRef}=React;
const styles=`
.xyz-transcript-share{position:fixed;inset:0;z-index:2147483500;background:#0009;display:flex;justify-content:center;align-items:center;padding:14px;box-sizing:border-box}
.xyz-transcript-share-dialog{width:100%;max-width:460px;height: min(90vh, 860px);max-height:100%;overflow:hidden;background:var(--app-bg);color:var(--app-fg);border:1px solid var(--app-border);border-radius:14px;padding:16px;box-sizing:border-box;display:flex;flex-direction:column;gap:14px;font-size:12px}
.xyz-transcript-share-head{display:flex;align-items:center;justify-content:space-between;gap:12px;font-size:15px}
.xyz-transcript-share button{font:inherit;color:inherit;cursor:pointer;border:1px solid var(--app-border);background:var(--app-hover);border-radius:6px;padding:6px 10px}
.xyz-transcript-share button:disabled{opacity:.4;cursor:default}
.xyz-transcript-share button:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px}
.xyz-transcript-share button[aria-pressed=true]{background:var(--app-selected);border-color:var(--app-focus)}
.xyz-transcript-share-controls{display:flex;gap:6px;align-items:center}
.xyz-transcript-share-preview{flex:1;min-height:0;overflow:auto;background:var(--app-hover);border:1px solid var(--app-border);border-radius:8px}
.xyz-transcript-share-preview:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px}
.xyz-transcript-share-preview img{display:block;width:100%;height:auto}
.xyz-transcript-share-footer{display:flex;justify-content:space-between;gap:8px;align-items:center;flex-shrink:0}
.xyz-transcript-share-actions{display:flex;gap:6px}
.xyz-transcript-share-actions button:last-child{background:var(--app-share);color:var(--app-share-ink);border-color:transparent}
`;

export function TranscriptShareDialog({episode,excerpt,onClose}) {
 const [theme,setTheme]=useState(()=>window.matchMedia?.('(prefers-color-scheme: dark)').matches?'dark':'light');
 const [generated,setGenerated]=useState(null),[generationError,setGenerationError]=useState('');
 const [busy,setBusy]=useState(false),[message,setMessage]=useState(''),[error,setError]=useState('');
 const root=useRef(null),alive=useRef(true),pending=useRef(false),currentKey=useRef(null);
 const key=JSON.stringify([episode,excerpt.startMs,excerpt.text,theme]);
 currentKey.current=key;
 const card=generated?.key===key?generated.src:'';
 useEffect(()=>{
  alive.current=true;const previous=document.activeElement;
  root.current?.querySelector('button')?.focus();
  return()=>{alive.current=false;if(previous?.isConnected)previous.focus()};
 },[]);
 useEffect(()=>{
  let active=true;setGenerationError('');setMessage('');setError('');
  createTranscriptCard(episode,[{startMs:excerpt.startMs,text:excerpt.text}],{theme,endMs:null}).then(src=>{
   if(active&&currentKey.current===key)setGenerated({key,src});
  }).catch(err=>{if(active&&currentKey.current===key)setGenerationError(err.message||'卡片生成失败，请重试')});
  return()=>{active=false};
 },[episode,excerpt.startMs,excerpt.text,theme]);
 const run=async method=>{
  if(pending.current||!card)return;pending.current=true;setBusy(true);setMessage('');setError('');
  try {
   const result=await window.mygo.call(method,card);
   if(alive.current&&currentKey.current===key)setMessage(method==='Backend.SaveShareCard'?(result?'卡片已保存':'已取消保存'):'卡片已复制，可粘贴到聊天窗口');
  }catch(err){if(alive.current&&currentKey.current===key)setError(err.message||'操作失败，请重试')}
  finally{pending.current=false;if(alive.current)setBusy(false)}
 };
 const onKeyDown=e=>{
  if(e.key==='Escape'){e.preventDefault();e.stopPropagation();if(!pending.current)onClose()}
  if(e.key==='Tab'){
   const items=[...root.current.querySelectorAll('button:not([disabled]), [tabindex="0"]')],first=items[0],last=items.at(-1);
   if(e.shiftKey&&(document.activeElement===first||!items.includes(document.activeElement))){e.preventDefault();last?.focus()}
   else if(!e.shiftKey&&(document.activeElement===last||!items.includes(document.activeElement))){e.preventDefault();first?.focus()}
  }
 };
 return createPortal(h('div',{className:'xyz-transcript-share',onMouseDown:e=>{if(e.target===e.currentTarget&&!pending.current)onClose()}},
  h('style',null,styles),
  h('section',{ref:root,className:'xyz-transcript-share-dialog',role:'dialog','aria-modal':true,'aria-label':'分享摘录',onKeyDown},
   h('div',{className:'xyz-transcript-share-head'},h('strong',null,'分享摘录'),h('button',{type:'button','aria-label':'关闭摘录分享',disabled:busy,onClick:onClose},'×')),
   h('div',{className:'xyz-transcript-share-preview',tabIndex:0,'aria-label':'摘录卡片预览'},card?h('img',{src:card,alt:'摘录分享卡片预览'}):h('div',{style:{padding:20},role:generationError?'alert':'status'},generationError||'正在生成卡片…')),
   h('div',{className:'xyz-transcript-share-footer'},
    h('div',{className:'xyz-transcript-share-controls','aria-label':'卡片配色'},
     ...[['light','浅色'],['dark','深色']].map(([value,label])=>h('button',{key:value,type:'button','aria-pressed':theme===value,disabled:busy,onClick:()=>setTheme(value)},label))),
    h('div',{className:'xyz-transcript-share-actions'},
     h('button',{type:'button',disabled:busy||!card,onClick:()=>run('Backend.SaveShareCard')},'保存'),
     h('button',{type:'button',disabled:busy||!card,onClick:()=>run('Backend.CopyShareCard')},'复制'))),
   message?h('div',{role:'status'},message):null,error?h('div',{role:'alert',className:'xyz-err'},error):null)),document.body);
}
