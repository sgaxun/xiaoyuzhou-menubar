import React from 'react';
import {episodeImage,episodePodcast} from './episode.js';
import QRCode from 'qrcode';
const h = React.createElement;
const {useState, useRef, useEffect} = React;

export function episodeLink(eid, position = null) {
  if (!/^[a-zA-Z0-9]{20,32}$/.test(eid || '')) throw new Error('这期节目暂时无法分享');
  return `https://www.xiaoyuzhoufm.com/episode/${eid}${position !== null && Number.isFinite(position) && position >= 0 ? `#ts=${Math.floor(position)}` : ''}`;
}
export function shareTime(seconds) {
  const n = Math.max(0, Math.floor(Number(seconds) || 0));
  const tail = `${String(Math.floor(n / 60) % 60).padStart(2,'0')}:${String(n % 60).padStart(2,'0')}`;
  return n >= 3600 ? `${Math.floor(n / 3600)}:${tail}` : `${Math.floor(n / 60)}:${String(n % 60).padStart(2,'0')}`;
}
function lines(ctx, text, width, limit) {
  const result = []; let line = '';
  for (const char of String(text || '')) {
    if (ctx.measureText(line + char).width > width && line) {
      result.push(line); line = '';
      if (result.length === limit) {
        let last = result[limit-1];
        while (ctx.measureText(last + '…').width > width) last = last.slice(0,-1);
        result[limit-1] = last + '…'; return result;
      }
    }
    line += char;
  }
  if (line) result.push(line);
  return result;
}
function loadImage(url) {
  return new Promise(resolve => {
    if (!url || !/^https:\/\//.test(url)) return resolve(null);
    const img = new Image();
    const timer = setTimeout(() => {img.src=''; resolve(null)}, 4000);
    img.crossOrigin = 'anonymous'; img.referrerPolicy = 'no-referrer';
    img.onload = () => {clearTimeout(timer); resolve(img)};
    img.onerror = () => {clearTimeout(timer); resolve(null)};
    img.src = url;
  });
}
export async function createShareCard(episode, position = null) {
  const link = episodeLink(episode.eid, position);
  const [cover, qr] = await Promise.all([
    loadImage(episodeImage(episode)),
    QRCode.toCanvas(link, {width:220, margin:1, errorCorrectionLevel:'M', color:{dark:'#24252b',light:'#ffffff'}}),
  ]);
  const canvas = document.createElement('canvas'); canvas.width=720; canvas.height=900;
  const ctx = canvas.getContext('2d');
  ctx.fillStyle='#f4f0e9'; ctx.fillRect(0,0,720,900);
  ctx.fillStyle='#ffffff'; ctx.beginPath();ctx.roundRect(28,28,664,844,24);ctx.fill();
  ctx.fillStyle='#74716d';ctx.font='18px system-ui, sans-serif';ctx.fillText('小宇宙播放器  /  播客分享',60,74);
  ctx.save();ctx.beginPath();ctx.roundRect(60,106,144,144,18);ctx.clip();
  if (cover) {
    const size=Math.min(cover.naturalWidth,cover.naturalHeight);
    ctx.drawImage(cover,(cover.naturalWidth-size)/2,(cover.naturalHeight-size)/2,size,size,60,106,144,144);
  } else {
    ctx.fillStyle='#ede7dc';ctx.fillRect(60,106,144,144);ctx.fillStyle='#84775f';ctx.font='bold 40px system-ui, sans-serif';
    ctx.fillText([...String(episodePodcast(episode) || '播客')].slice(0,2).join(''),84,194);
  }
  ctx.restore();ctx.fillStyle='#67635d';ctx.font='22px system-ui, sans-serif';
  lines(ctx,episodePodcast(episode) || '播客单集',396,2).forEach((line,i)=>ctx.fillText(line,232,164+i*32));
  ctx.fillStyle='#24252b';ctx.font='bold 34px system-ui, sans-serif';
  lines(ctx,episode.title || '未命名单集',600,3).forEach((line,i)=>ctx.fillText(line,60,308+i*49));
  if (position !== null) {
    ctx.fillStyle='#efebe4';ctx.beginPath();ctx.roundRect(60,438,600,54,10);ctx.fill();
    ctx.fillStyle='#544b3c';ctx.font='23px system-ui, sans-serif';ctx.fillText(`从 ${shareTime(position)} 开始听`,80,473);
  }
  ctx.fillStyle='#e8e4dd';ctx.fillRect(60,524,600,1);
  ctx.drawImage(qr,60,559,220,220);
  ctx.fillStyle='#24252b';ctx.font='bold 23px system-ui, sans-serif';ctx.fillText('扫码打开本期节目',310,642);
  ctx.fillStyle='#77736d';ctx.font='18px system-ui, sans-serif';
  ctx.fillText(position !== null ? `已标记 ${shareTime(position)}` : '在小宇宙收听完整节目',310,681);
  ctx.fillStyle='#9a9690';ctx.font='16px system-ui, sans-serif';ctx.fillText('把值得听的，分享给在意的人。',60,826);
  return canvas.toDataURL('image/png');
}

export function ShareDialog({episode, position, onClose}) {
  const [mode,setMode] = useState('menu');
  const [atTime,setAtTime] = useState(false);
  const [busy,setBusy] = useState(false);
  const [message,setMessage] = useState('');
  const [error,setError] = useState('');
  const [card,setCard] = useState('');
  const root = useRef(null), pending=useRef(false), alive=useRef(true);
  const canShareTime = position !== null && Number.isFinite(position);
  const run = async task => {
    if (pending.current) return;
    pending.current=true;setBusy(true);setError('');setMessage('');
    try {await task()} catch(err) {if(alive.current)setError(err.message || '操作未完成，请重试')}
    finally {pending.current=false;if(alive.current)setBusy(false)}
  };
  useEffect(()=>{
    alive.current=true;
    const before=document.activeElement;
    root.current?.querySelector('button')?.focus();
    return()=>{alive.current=false;if(before?.isConnected)before.focus()};
  },[]);
  useEffect(()=>{root.current?.querySelector('button')?.focus()},[mode]);
  useEffect(()=>{
    if(mode!=='card')return;
    let active=true;setCard('');setError('');
    createShareCard(episode,atTime&&canShareTime?position:null).then(value=>{if(active)setCard(value)}).catch(()=>{if(active)setError('分享卡生成失败，请稍后重试')});
    return()=>{active=false};
  },[mode,episode,atTime,position,canShareTime]);
  const choose = next => {setMode(next);setMessage('');setError('');setAtTime(false)};
  const timeChoice = canShareTime ? h('div',{className:'xyz-share-timing','aria-label':'分享范围'},
    h('button',{type:'button',className:'xyz-btn','aria-pressed':!atTime,disabled:busy,onClick:()=>setAtTime(false)},'整期节目'),
    h('button',{type:'button',className:'xyz-btn','aria-pressed':atTime,disabled:busy,onClick:()=>setAtTime(true)},`当前时间点 · ${shareTime(position)}`)) : null;
  const keyDown = e => {
    if(e.key==='Escape'){e.stopPropagation();if(!busy)onClose()}
    if(e.key==='Tab') {
      const items=[...root.current.querySelectorAll('button:not([disabled])')];const first=items[0],last=items.at(-1);
      if(e.shiftKey&&document.activeElement===first){e.preventDefault();last?.focus()}
      else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first?.focus()}
    }
  };
  return h('div',{className:'xyz-share-backdrop',onMouseDown:e=>{if(e.target===e.currentTarget&&!busy)onClose()}},
    h('section',{className:'xyz-share-dialog',role:'dialog','aria-modal':true,'aria-label':'分享节目',ref:root,onKeyDown:keyDown},
      h('div',{className:'xyz-share-head'},
        h('strong',null,mode==='menu'?'分享节目':mode==='card'?'分享卡片':'复制链接'),
        h('button',{type:'button',className:'xyz-btn',disabled:busy,'aria-label':'关闭分享',onClick:onClose},'×')),
      h('div',{className:'xyz-share-title'},episode.title),
      mode==='menu'?h('div',{className:'xyz-share-options'},
        h('button',{type:'button',className:'xyz-btn',onClick:()=>choose('link')},'复制链接'),
        h('button',{type:'button',className:'xyz-btn',onClick:()=>choose('card')},'分享卡片')):
        h(React.Fragment,null,timeChoice,
          mode==='card'?h(React.Fragment,null,
            card?h('img',{className:'xyz-share-card',src:card,alt:'节目二维码分享卡'}):!error?h('div',{className:'xyz-hint'},'正在生成分享卡…'):null,
            h('div',{className:'xyz-share-actions'},
              h('button',{type:'button',className:'xyz-btn',disabled:busy||!card,onClick:()=>run(async()=>{const saved=await window.mygo.call('Backend.SaveShareCard',card);if(alive.current)setMessage(saved?'分享卡已保存':'已取消保存')})},'保存卡片'),
              h('button',{type:'button',className:'xyz-btn primary',disabled:busy||!card,onClick:()=>run(async()=>{await window.mygo.call('Backend.CopyShareCard',card);if(alive.current)setMessage('卡片已复制，可粘贴到聊天窗口')})},'复制卡片'))):
          h(React.Fragment,null,
            h('div',{className:'xyz-hint'},atTime?'复制带当前时间点的节目链接。':'复制整期节目链接。'),
            h('button',{type:'button',className:'xyz-btn primary',disabled:busy,onClick:()=>run(async()=>{await window.mygo.call('Backend.CopyEpisodeLinkAtTime',episode.eid,position||0,atTime&&canShareTime);if(alive.current)setMessage(atTime?'带时间点的链接已复制':'节目链接已复制')})},busy?'正在复制…':'复制链接')),
          h('button',{type:'button',className:'xyz-btn xyz-share-back',disabled:busy,onClick:()=>choose('menu')},'‹ 返回')),
      message?h('div',{className:'xyz-hint',role:'status'},message):null,
      error?h('div',{className:'xyz-err',role:'alert'},error):null));
}
