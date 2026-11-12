import React from 'react';
import {queueEpisode} from './episode.js';
const h = React.createElement;
const {useState,useEffect,useRef} = React;

// Preserve event order across asynchronous bridge calls. Importing a queue never
// reports playback; only a real media play event starts a listening session.
export function createPlaybackReporter(call) {
  let pending = Promise.resolve();
  const sessions = new Map();
  return (event, episode, position) => {
    if (!episode?.eid || !Number.isFinite(position)) return pending;
    const previous = sessions.get(episode.eid);
    if (event === 'playing') sessions.set(episode.eid, 'playing');
    else {
      if (!previous || previous === 'ended' || (event === 'paused' && previous === 'paused')) return pending;
      sessions.set(episode.eid, event);
    }
    const body = {event, episode:queueEpisode(episode), position:Math.max(0,Math.round(position)), playedAt:new Date().toISOString()};
    pending = pending.catch(()=>{}).then(()=>call('playbackEvent',body));
    // A local persistence failure must be visible instead of silently pretending
    // the cloud task was saved. The next media event can still proceed.
    pending.catch(error=>window.dispatchEvent(new CustomEvent('xyz-cloud-error',{detail:error.message})));
    return pending;
  };
}

export function PullCloudPlaylist({call,player,loggedIn,onUpdated}) {
  const [busy,setBusy] = useState(false), [message,setMessage] = useState('');
  const latest = useRef(player); latest.current = player;
  const lock = useRef(false);
  const messageTimer = useRef(null);
  useEffect(()=>()=>clearTimeout(messageTimer.current),[]);
  const pull = async () => {
    if(lock.current)return;
    clearTimeout(messageTimer.current);
    lock.current=true;setBusy(true);setMessage('');
    try {
      await latest.current.flushLocalData(false);
      const snapshot=latest.current.queue, currentEid=latest.current.current?.eid || '';
      const baseline={...(latest.current.progressRef?.current || {})};
      const protectedEid=latest.current.protectedProgressEid?.() || '';
      const data = await call('pullCloudPlaylist',{currentEid,protectedEid});
      const p=latest.current;
      // The backend only previews. A changed local queue wins over a late result.
      if(p.queue !== snapshot || (p.current?.eid || '') !== currentEid) {
        setMessage('列表或当前节目已变化，本次未合并，请重新拉取');
        return;
      }
      const queue=[...(data.queue || [])];
      const saved=await call('saveQueue',{queue:queue.map(queueEpisode),index:p.current ? queue.findIndex(ep=>ep.eid===p.current.eid) : -1,cloudImportId:data.cloudImportId,currentEid:p.current?.eid || '',playingEid:p.playing ? p.current?.eid || '' : '',protectedEid:p.protectedProgressEid?.() || ''});
      const committedQueue=saved.queue || queue;
      // Do not apply a late response over edits made while committing.
      if(latest.current.queue===snapshot && (latest.current.current?.eid || '')===currentEid) {
        latest.current.setQueue(committedQueue);
        const currentIndex=latest.current.current ? committedQueue.findIndex(ep=>ep.eid===latest.current.current.eid) : -1;
        latest.current.setIndex(currentIndex);
        if(latest.current.current && currentIndex<0) {
          if(latest.current.discardFinishedCurrent)latest.current.discardFinishedCurrent();
          else latest.current.setCurrent?.(null);
        }
      }
      latest.current.applyImportedProgress?.(saved.importedProgress || {},baseline);
      await onUpdated?.();
      const removed=queue.length-committedQueue.length;
      setMessage(data.progressWarning || `云端列表和收听进度已合并${removed>0 ? `，移除 ${removed} 期已听完节目` : ''}`);
      if(!data.progressWarning)messageTimer.current=setTimeout(()=>setMessage(''),3000);
    } catch(error){setMessage(error.message)}
    finally{lock.current=false;setBusy(false)}
  };
  const currentIndex=player.queue.findIndex(ep=>ep.eid===player.current?.eid);
  return h('div',{style:{display:'flex',flexDirection:'column',gap:6}},
    h('div',{style:{display:'flex',alignItems:'center',justifyContent:'space-between',gap:12,minHeight:28}},
      h('span',{className:'xyz-hint'},`${player.queue.length} 集${currentIndex>=0 ? ` · 当前第 ${currentIndex+1} 集` : ''}`),
      h('button',{className:'xyz-btn',style:{display:'inline-flex',alignItems:'center',gap:5,background:'transparent',border:'none',fontSize:12,padding:'4px 0 4px 8px',flexShrink:0},disabled:busy||!loggedIn,onClick:pull,title:'从云端拉取并合并到本地播放列表'},
        h('svg',{width:13,height:13,viewBox:'0 0 24 24',fill:'none',stroke:'currentColor',strokeWidth:1.8,strokeLinecap:'round',strokeLinejoin:'round','aria-hidden':true},
          h('path',{d:'M20 7v5h-5M4 17v-5h5'}),h('path',{d:'M6.1 7a7 7 0 0 1 11.6-1L20 9M4 15l2.3 3A7 7 0 0 0 17.9 17'})),
        busy?'拉取中…':'拉取云端列表')),
    message?h('div',{className:'xyz-hint',role:'status'},message):null);
}

export function CloudSyncSettings({call,loggedIn}) {
  const [sync,setSync]=useState(null),[busy,setBusy]=useState(false),[error,setError]=useState('');
  const lock=useRef(false);
  useEffect(()=>{
    let alive=true;
    // This reads the on-disk status only, never the remote service.
    const refresh=()=>call('state').then(data=>{if(alive)setSync(data.state?.cloudSync||{})}).catch(e=>{if(alive)setError(e.message)});
    refresh();const timer=setInterval(refresh,5000);
    const onError=e=>{if(alive)setError(e.detail)};
    window.addEventListener('xyz-cloud-error',onError);
    return()=>{alive=false;clearInterval(timer);window.removeEventListener('xyz-cloud-error',onError)};
  },[call]);
  const run=async(action,body)=>{
    if(lock.current)return;lock.current=true;setBusy(true);setError('');
    try{await call(action,body);const data=await call('state');setSync(data.state?.cloudSync||{})}
    catch(e){setError(e.message)}finally{lock.current=false;setBusy(false)}
  };
  return h('div',{className:'xyz-section'},
    h('div',{className:'xyz-section-title'},'云端收听同步'),
    h('div',{className:'xyz-setting-row'},
      h('div',{className:'xyz-setting-label'},h('div',{className:'xyz-setting-name'},'自动上传进度与历史'),h('div',{className:'xyz-setting-desc'},'播完或暂停满 5 分钟后上传，提前续播取消。两次上传至少间隔 5 分钟。')),
      h('button',{className:'xyz-btn',disabled:busy||!loggedIn||!sync,onClick:()=>run('cloudSyncSettings',{enabled:!sync?.enabled})},sync?.enabled?'已开启':'已关闭')),
    h('div',{className:'xyz-hint',role:'status'},sync?.lastSuccessAt?`最近成功：${new Date(sync.lastSuccessAt).toLocaleString()}`:'尚未上传'),
    sync?.lastError?h('div',{className:'xyz-hint'},`上传失败：${sync.lastError}`):null,
    sync?.uploading ? h('div',{className:'xyz-hint',role:'status'},'正在上传收听记录…') : null,
    sync?.waiting ? h('div',{className:'xyz-hint'},`等待上传 ${sync.waiting} 项${sync.nextUploadAt ? ` · 最早 ${new Date(sync.nextUploadAt).toLocaleTimeString([], {hour:'2-digit',minute:'2-digit'})}` : ''}`) : null,
    sync?.blocked ? h('div',{className:'xyz-hint'},'自动上传已暂停，请手动重试恢复。') : null,
    sync?.failed ? h('button',{className:'xyz-btn',disabled:busy||!loggedIn||!sync?.enabled||!!sync?.uploading,onClick:()=>run('retryCloudSync',{})},busy?'处理中…':`重试失败记录（${sync.failed}）`) : null,
    error?h('div',{className:'xyz-err',role:'alert'},error):null);
}
