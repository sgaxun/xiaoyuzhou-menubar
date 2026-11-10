import React from 'react';
const {createElement:h,useEffect,useState}=React;
export function PinWindow(){
 const [pinned,setPinned]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState('');
 useEffect(()=>{
  let alive=true;
  const change=e=>{if(alive)setPinned(!!e.detail)};
  window.addEventListener('xyz-pin-changed',change);
  window.mygo.call('Backend.Pinned').then(value=>{if(alive)setPinned(!!value)}).catch(()=>{});
  return()=>{alive=false;window.removeEventListener('xyz-pin-changed',change)};
 },[]);
 const toggle=async()=>{
  setBusy(true);setError('');
  try{setPinned(!!await window.mygo.call('Backend.SetPinned',!pinned))}
  catch{setError('置顶失败，请重试')}
  finally{setBusy(false)}
 };
 return h('button',{type:'button',className:'xyz-icon-btn',disabled:busy,onClick:toggle,
  'aria-pressed':pinned,'aria-label':pinned?'取消置顶':'置顶窗口',
  title:error||(pinned?'取消置顶：恢复切换应用时自动收起':'置顶窗口：切换应用不收起'),
  style:{flexShrink:0,color:pinned?'var(--app-share)':undefined,background:pinned?'var(--app-selected)':undefined}},
  h('svg',{width:15,height:15,viewBox:'0 0 24 24',fill:pinned?'currentColor':'none',stroke:'currentColor',strokeWidth:1.5,strokeLinecap:'round',strokeLinejoin:'round','aria-hidden':true},h('path',{d:'m15 3 6 6-4 1-3 5-2-2-6 7 4-9-2-2 5-3Z'})));
}
