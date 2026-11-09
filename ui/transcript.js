import React from 'react';
import {createPortal} from 'react-dom';
import {TranscriptShareDialog} from './transcript-share.js';
import {readTranscriptSelection} from './transcript-selection.js';
const {createElement:h,useEffect,useRef,useState}=React;
const styles=`
.xyz-transcript{display:flex;flex-direction:column;min-height:0;flex:1}
.xyz-transcript-scroll{overflow:auto;overscroll-behavior:contain;min-height:0;flex:1;padding:4px 20px 20px;scrollbar-gutter:stable;user-select:text;-webkit-user-select:text;cursor:text}
.xyz-transcript-row{padding:12px 0}
.xyz-transcript-paragraph{margin:4px 0 0;color:var(--app-fg);opacity:.65;font-size:14px;font-weight:400;line-height:1.9;white-space:pre-wrap;overflow-wrap:anywhere;user-select:text;-webkit-user-select:text}
.xyz-transcript-row[aria-current=true] .xyz-transcript-paragraph{opacity:1}
.xyz-transcript-time{display:block;padding:0;border:0;background:none;color:var(--app-fg);opacity:.45;font:inherit;font-size:11px;line-height:1.6;font-variant-numeric:tabular-nums;cursor:pointer;user-select:none;-webkit-user-select:none}
.xyz-transcript-time:hover{opacity:1;color:var(--app-share)}
.xyz-transcript button:focus-visible,.xyz-transcript-excerpt-trigger:focus-visible,.xyz-notes-tabs button:focus-visible{outline:2px solid var(--app-focus);outline-offset:3px}
.xyz-transcript-paragraph::selection{background:var(--app-selected);color:var(--app-fg)}
.xyz-transcript-toolbar{padding:8px 20px 4px;font-size:11px;display:flex;align-items:center;justify-content:space-between;gap:8px;min-height:24px;flex-shrink:0;color:var(--app-fg)}
.xyz-transcript-toolbar>span{opacity:.45}
.xyz-transcript-toolbar>button{background:none;border:0;padding:0;color:var(--app-fg);font:inherit;cursor:pointer;opacity:.7}
.xyz-transcript-excerpt-trigger{position:fixed;z-index:2147483400;padding:7px 12px;border:1px solid var(--app-border);border-radius:7px;background:var(--app-bg);color:var(--app-fg);box-shadow:0 3px 12px #0003;font:inherit;font-size:12px;cursor:pointer;line-height:18px;white-space:nowrap}
.xyz-notes-tabs{display:flex;gap:8px;padding:8px 14px;border-bottom:1px solid var(--app-border);flex-shrink:0}
.xyz-notes-tabs button{font:inherit;font-size:12px;border:1px solid var(--app-border);border-radius:6px;padding:6px 12px;background:var(--app-hover);color:var(--app-fg);cursor:pointer}
.xyz-notes-tabs button[aria-pressed=true]{background:var(--app-selected);border-color:var(--app-focus)}
`;
const timestamp=ms=>{const s=Math.floor(ms/1000);return `${Math.floor(s/60)}:${String(s%60).padStart(2,'0')}`};

export function Transcript({episode,player,call}) {
  const eid=episode?.eid;
  const [result,setResult]=useState({eid,status:'loading',segments:[]});
  const [selection,setSelection]=useState(null),[excerpt,setExcerpt]=useState(null);
  const [attempt,setAttempt]=useState(0),[following,setFollowing]=useState(true);
  const container=useRef(null),rows=useRef([]),selecting=useRef(false),excerptRef=useRef(null),selectionRef=useRef(null);
  excerptRef.current=excerpt;selectionRef.current=selection;
  useEffect(()=>{
    let alive=true;
    setResult({eid,status:'loading',segments:[]});setFollowing(true);setSelection(null);setExcerpt(null);rows.current=[];
    if(!eid){setResult({eid,status:'unavailable',segments:[]});return;}
    Promise.resolve().then(()=>call(`transcript?eid=${encodeURIComponent(eid)}`)).then(data=>{
      if(!data?.ok||!['available','unavailable'].includes(data.status))throw new Error('字幕响应异常');
      const segments=(Array.isArray(data.segments)?data.segments:[])
        .filter(s=>Number.isFinite(s.startMs)&&s.startMs>=0&&typeof s.text==='string'&&s.text.trim())
        .slice().sort((a,b)=>a.startMs-b.startMs);
      if(alive)setResult({eid,status:data.status==='available'&&segments.length?'available':'unavailable',segments});
    }).catch(error=>{if(alive)setResult({eid,status:'error',segments:[],error:error.message||'字幕加载失败'})});
    return()=>{alive=false};
  },[eid,call,attempt]);
  useEffect(()=>{
    let frame=0;
    const read=()=>{
      if(excerptRef.current||selecting.current)return;
      const next=readTranscriptSelection(container.current);
      setSelection(next);if(next)setFollowing(false);
    };
    const schedule=()=>{cancelAnimationFrame(frame);frame=requestAnimationFrame(read)};
    const release=()=>{selecting.current=false;schedule()};
    const outside=e=>{
      if(!container.current?.contains(e.target)&&!e.target.closest?.('.xyz-transcript-excerpt-trigger')&&!excerptRef.current)setSelection(null);
    };
    document.addEventListener('selectionchange',schedule);
    document.addEventListener('pointerup',release);
    document.addEventListener('pointercancel',release);
    document.addEventListener('pointerdown',outside);
    window.addEventListener('resize',schedule);
    return()=>{
      cancelAnimationFrame(frame);document.removeEventListener('selectionchange',schedule);
      document.removeEventListener('pointerup',release);document.removeEventListener('pointercancel',release);
      document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',schedule);
    };
  },[eid]);
  const visible=result.eid===eid?result:{status:'loading',segments:[]};
  const current=!!eid&&player.current?.eid===eid;
  let active=-1;
  if(current)for(let i=0;i<visible.segments.length;i++){
    if(visible.segments[i].startMs<=player.position*1000)active=i;else break;
  }
  const reveal=()=>{
    const box=container.current,row=rows.current[active];if(!box||!row)return;
    const b=box.getBoundingClientRect(),r=row.getBoundingClientRect();
    box.scrollTop=Math.max(0,box.scrollTop+r.top-b.top-12);
  };
  useEffect(()=>{if(following&&current&&visible.status==='available'&&!excerpt&&!selection)reveal()},[active,following,current,visible.status,eid,excerpt,selection]);
  const pause=()=>setFollowing(false);
  const clearSelection=()=>{
    if(container.current?.contains(window.getSelection()?.anchorNode))window.getSelection()?.removeAllRanges();
    setSelection(null);
  };
  const jump=segment=>{
    clearSelection();const seconds=segment.startMs/1000;
    if(current){player.seek(seconds);setFollowing(true)}
    else{player.setIndex(player.queue.findIndex(ep=>ep.eid===eid));player.play(episode,seconds);setFollowing(true)}
  };
  const closeShare=()=>{
    setExcerpt(null);
    const saved=selectionRef.current;
    if(saved?.range?.startContainer.isConnected&&saved.range.endContainer.isConnected){
      const native=window.getSelection();native.removeAllRanges();native.addRange(saved.range);
      container.current?.focus({preventScroll:true});
    }
  };
  return h('div',{className:'xyz-transcript'},h('style',null,styles),
    visible.status==='loading'?h('div',{className:'xyz-notes-message',role:'status'},'正在加载字幕…'):
    visible.status==='error'?h('div',{className:'xyz-notes-message',role:'alert'},`${visible.error || '字幕加载失败'} `,h('button',{type:'button',className:'xyz-btn',onClick:()=>setAttempt(n=>n+1)},'重试')):
    visible.status==='unavailable'?h('div',{className:'xyz-notes-message',role:'status'},'这期节目暂无字幕'):
    h(React.Fragment,null,
      h('div',{className:'xyz-transcript-toolbar'},h('span',null,'划选文字分享 · 点时间跳转'),current&&!following?h('button',{type:'button',disabled:active<0,onClick:()=>{clearSelection();setFollowing(true);reveal()}},'回到当前段'):null),
      h('div',{ref:container,className:'xyz-transcript-scroll',role:'region','aria-label':'字幕文字',tabIndex:0,
        onWheel:pause,onTouchMove:pause,
        onPointerDown:e=>{pause();if(!e.target.closest('button')){selecting.current=true;setSelection(null)}},
        onScroll:()=>{if(!excerptRef.current&&!selecting.current)setSelection(readTranscriptSelection(container.current))},
        onKeyDown:e=>{
          if(['ArrowUp','ArrowDown','PageUp','PageDown','Home','End',' '].includes(e.key))pause();
          if(e.key==='Escape'&&selectionRef.current){e.preventDefault();e.stopPropagation();clearSelection()}
        }},
        visible.segments.map((segment,i)=>h('div',{key:`${segment.startMs}-${i}`,ref:el=>{rows.current[i]=el},className:'xyz-transcript-row','aria-current':i===active?'true':undefined},
          h('button',{type:'button',className:'xyz-transcript-time','aria-label':`跳转到 ${timestamp(segment.startMs)}`,onClick:()=>jump(segment)},timestamp(segment.startMs)),
          h('p',{className:'xyz-transcript-paragraph','data-transcript-text':true,'data-start-ms':segment.startMs},segment.text)))),
      selection&&!excerpt?createPortal(h('button',{type:'button',className:'xyz-transcript-excerpt-trigger',style:{left:selection.left,top:selection.top},
        onMouseDown:e=>e.preventDefault(),onClick:()=>{setFollowing(false);setExcerpt({text:selection.text,startMs:selection.startMs})},
        onKeyDown:e=>{if(e.key==='Escape'){e.preventDefault();e.stopPropagation();clearSelection()}}},'分享摘录'),document.body):null,
      excerpt?h(TranscriptShareDialog,{key:eid,episode,excerpt,onClose:closeShare}):null));
}
