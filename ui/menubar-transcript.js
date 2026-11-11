import React from 'react';
import {createSubtitleScheduler,scrollingSubtitle} from './menubar-transcript-scheduler.js';
const {createElement:h,useEffect,useRef,useState}=React;
const preference='xyz-menubar-transcript';
function readEnabled(){if(typeof window[preference]==='boolean')return window[preference];try{return localStorage.getItem(preference)==='true'}catch{return false}}
function toggleEnabled(){const next=!readEnabled();window[preference]=next;try{localStorage.setItem(preference,String(next))}catch{/* Embedded pages may deny storage; keep the switch usable for this session. */}window.dispatchEvent(new Event(preference))}
function useEnabled(){
 const [enabled,setEnabled]=useState(readEnabled);
 useEffect(()=>{const update=()=>setEnabled(readEnabled());window.addEventListener(preference,update);return()=>window.removeEventListener(preference,update)},[]);
 return enabled;
}
export function MenubarTranscriptSettings(){
 const enabled=useEnabled();
 return h('div',{className:'xyz-setting-row'},h('div',{className:'xyz-setting-label'},
  h('div',{className:'xyz-setting-name'},'菜单栏显示字幕'),
  h('div',{className:'xyz-setting-desc'},'固定宽度滚动显示，随播放进度前进和回退')),
  h('button',{type:'button',className:'xyz-btn','aria-pressed':enabled,onClick:toggleEnabled},enabled?'已开启':'已关闭'));
}
export {scrollingSubtitle} from './menubar-transcript-scheduler.js';
export function MenubarTranscript({player,call}){
 const enabled=useEnabled(),eid=player.current?.eid;
 const current=useRef(player);current.current=player;
 const [started,setStarted]=useState(''),[result,setResult]=useState({eid:null,segments:[]});
 const last=useRef(null),startedRef=useRef('');
 const send=(text,progress=0)=>{const key=text+'|'+progress.toFixed(4);if(last.current===key)return;last.current=key;window.mygo.call('Backend.MenuSubtitle',text,progress).catch(()=>{last.current=null})};
 useEffect(()=>createSubtitleScheduler({
  audio:current.current.audioRef.current,enabled,eid,
  hasSegments:result.eid===eid&&result.segments.length>0,
  onStarted:id=>{if(startedRef.current!==id){startedRef.current=id;setStarted(id)}},
  onUpdate:audio=>{
   if(!audio||result.eid!==eid){send('');return}
   const subtitle=scrollingSubtitle(result.segments,audio.currentTime,Number.isFinite(audio.duration)?audio.duration:current.current.current?.duration);
   send(subtitle.text,subtitle.progress);
  }
 }),[enabled,eid,result]);
 useEffect(()=>{
  let alive=true;
  if(!enabled||!eid||started!==eid)return;
  call(`transcript?eid=${encodeURIComponent(eid)}`).then(d=>{
   if(alive)setResult({eid,segments:d.status==='available'&&Array.isArray(d.segments)?d.segments.filter(s=>Number.isFinite(s.startMs)&&s.startMs>=0&&typeof s.text==='string').sort((a,b)=>a.startMs-b.startMs):[]});
  }).catch(()=>{if(alive)setResult({eid,segments:[]})});
  return()=>{alive=false};
 },[enabled,eid,started,call]);
 useEffect(()=>{window.mygo.call('Backend.MenuSubtitleEnabled',enabled).catch(()=>{})},[enabled]);
 useEffect(()=>{
  const toggle=toggleEnabled;
  window.addEventListener('xyz-toggle-menubar-transcript',toggle);
  return()=>{window.removeEventListener('xyz-toggle-menubar-transcript',toggle);window.mygo.call('Backend.MenuSubtitle','',0).catch(()=>{})};
 },[]);
 return null;
}
