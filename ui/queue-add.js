import React from 'react';
import {createPortal} from 'react-dom';
const h=React.createElement;
const {useState,useRef,useEffect}=React;
export function QueueIcon({next=false}) {
  return h('svg',{width:15,height:15,viewBox:'0 0 24 24',fill:'none',stroke:'currentColor',strokeWidth:1.8,strokeLinecap:'round',strokeLinejoin:'round','aria-hidden':true},
    h('path',{d:next?'M6 3v8M2 7h8M14 6h6M14 11h6M4 16h16M4 21h16':'M4 6h12M4 11h12M4 16h6M18 13v8M14 17h8'}));
}
export function QueueAdd({episode,player,disabled=false,onAdd}) {
  const [menu,setMenu]=useState(null),[busy,setBusy]=useState(false),[error,setError]=useState('');
  const anchor=useRef(null),popup=useRef(null);
  const queued=player.queue.some(ep=>ep.eid===episode.eid)||player.current?.eid===episode.eid;
  const close=()=>{setMenu(null);anchor.current?.focus()};
  useEffect(()=>{
    if(!menu)return;
    popup.current?.querySelector('button')?.focus({preventScroll:true});
    const outside=e=>{if(!anchor.current?.contains(e.target)&&!popup.current?.contains(e.target))setMenu(null)};
    const scroll=e=>{if(!popup.current?.contains(e.target))setMenu(null)};
    document.addEventListener('pointerdown',outside,true);window.addEventListener('scroll',scroll,true);
    return()=>{document.removeEventListener('pointerdown',outside,true);window.removeEventListener('scroll',scroll,true)};
  },[menu]);
  const add=async where=>{
    if(busy)return;setBusy(true);setError('');
    try{if(onAdd)await onAdd(where);else player.enqueue(episode,where);close()}
    catch(err){setError(err.message||'加入失败，请重试')}
    finally{setBusy(false)}
  };
  return h(React.Fragment,null,
    h('button',{ref:anchor,type:'button',className:queued?'xyz-row-action added':'xyz-row-action',disabled:disabled||busy||queued,title:queued?'已在播放列表':'加入播放列表','aria-label':queued?'已在播放列表':'加入播放列表','aria-haspopup':'menu','aria-expanded':!!menu,onClick:e=>{e.stopPropagation();if(menu){close();return}const r=e.currentTarget.getBoundingClientRect();setMenu({left:Math.max(8,Math.min(r.right-150,window.innerWidth-158)),top:r.bottom+5,above:r.bottom+100>window.innerHeight})}},
      h('svg',{width:14,height:14,viewBox:'0 0 24 24',fill:'none',stroke:'currentColor',strokeWidth:2,'aria-hidden':true},h('path',{d:queued?'M4 12.5l5 5L20 6.5':'M12 5v14M5 12h14'}))),
    menu?createPortal(h('div',{ref:popup,role:'menu','aria-label':'加入播放列表',className:'xyz-queue-menu',style:{left:menu.left,...(menu.above?{bottom:Math.max(8,window.innerHeight-menu.top+32)}:{top:menu.top})},onClick:e=>e.stopPropagation(),onKeyDown:e=>{e.stopPropagation();if(e.key==='Escape'){e.preventDefault();close()}if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();const items=[...popup.current.querySelectorAll('button')];const i=items.indexOf(document.activeElement);items[(i+(e.key==='ArrowDown'?1:items.length-1))%items.length]?.focus()}if(e.key==='Tab')setMenu(null)}},
      h('button',{type:'button',role:'menuitem',disabled:busy,onClick:()=>add('next')},h(QueueIcon,{next:true}),'下一集播放'),
      h('button',{type:'button',role:'menuitem',disabled:busy,onClick:()=>add('end')},h(QueueIcon,{}),'加入队尾'),
      error?h('div',{role:'alert',className:'xyz-hint'},error):null),document.body):null);
}
