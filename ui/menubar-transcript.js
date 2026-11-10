import React from 'react';
const {createElement:h,useEffect,useRef,useState}=React;
const preference='xyz-menubar-transcript';
function readEnabled(){try{return localStorage.getItem(preference)==='true'}catch{return false}}
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
  h('button',{type:'button',className:'xyz-btn','aria-pressed':enabled,onClick:()=>{try{localStorage.setItem(preference,String(!enabled))}catch{return}window.dispatchEvent(new Event(preference))}},enabled?'已开启':'已关闭'));
}
export function scrollingSubtitle(segments,seconds,duration){
 if(!Number.isFinite(seconds)||seconds<0)return {text:'',progress:0};
 let lo=0,hi=segments.length-1,index=-1;
 while(lo<=hi){const mid=(lo+hi)>>1;if(segments[mid].startMs<=seconds*1000){index=mid;lo=mid+1}else hi=mid-1}
 if(index<0)return {text:'',progress:0};
 const segment=segments[index],start=segment.startMs/1000;
 const end=segments[index+1]?.startMs/1000||duration;
 const fraction=end>start?(seconds-start)/(end-start):0;
 // Briefly hold the beginning and end; interpolate using audio time, so
 // pausing freezes the text and seeking immediately repositions it.
 return {text:segment.text,progress:Math.max(0,Math.min(1,(fraction-.12)/.76))};
}
export function MenubarTranscript({player,call}){
 const enabled=useEnabled(),eid=player.current?.eid;
 const current=useRef(player);current.current=player;
 const [started,setStarted]=useState(''),[result,setResult]=useState({eid:null,segments:[]});
 const last=useRef(null);
 const send=(text,progress=0)=>{const key=text+'|'+progress.toFixed(4);if(last.current===key)return;last.current=key;window.mygo.call('Backend.MenuSubtitle',text,progress).catch(()=>{last.current=null})};
 useEffect(()=>{
  let alive=true;
  const tick=()=>{
   const p=current.current,a=p.audioRef.current;
   if(!enabled||!eid){send('');return}
   if(a?.dataset.xyzEid===eid&&!a.paused)setStarted(eid);
   if(result.eid!==eid||a?.dataset.xyzEid!==eid){send('');return}
   const subtitle=scrollingSubtitle(result.segments,a.currentTime,Number.isFinite(a.duration)?a.duration:p.current?.duration);
   send(subtitle.text,subtitle.progress);
  };
  tick();const timer=setInterval(()=>{if(alive)tick()},100);
  return()=>{alive=false;clearInterval(timer)};
 },[enabled,eid,result]);
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
  const toggle=()=>{try{localStorage.setItem(preference,String(!readEnabled()))}catch{return}window.dispatchEvent(new Event(preference))};
  window.addEventListener('xyz-toggle-menubar-transcript',toggle);
  return()=>{window.removeEventListener('xyz-toggle-menubar-transcript',toggle);window.mygo.call('Backend.MenuSubtitle','',0).catch(()=>{})};
 },[]);
 return null;
}
