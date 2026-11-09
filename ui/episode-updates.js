import React from 'react';
import {queueEpisode} from './episode.js';
const h = React.createElement;
const {useState, useEffect, useRef} = React;

/** Polling reads local state only; the Go scheduler owns network checks. */
export function EpisodeUpdates({player, call, EpisodeRow, subscriptionCount, onSubscriptions, relTime}) {
  const [feed, setFeed] = useState({episodes: [], failures: []});
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const alive = useRef(true);
  const lock = useRef(false);
  const currentPlayer = useRef(player);
  currentPlayer.current = player;
  useEffect(() => {
    alive.current = true;
    const refresh = async (initial = false) => {
      if (lock.current) return;
      try {
        const data = await call('episodeUpdates');
        if (alive.current && !lock.current) setFeed(data);
      } catch (error) {if (initial && alive.current) setMessage(error.message)}
    };
    void refresh(true);
    const timer = setInterval(() => {void refresh()}, 5000);
    return () => {alive.current = false; clearInterval(timer)};
  }, []);
  const run = async task => {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setMessage('');
    try {await task()} catch (error) {if (alive.current) setMessage(error.message)}
    finally {lock.current = false; if (alive.current) setBusy(false)}
  };
  const check = () => run(async () => {
    const data = await call('checkEpisodeUpdates');
    if (!alive.current) return;
    setFeed(data);
    setMessage(data.remaining ? `还有 ${data.remaining} 个节目待检查，可继续检查。` : '检查完成，近期检查过的节目会暂时跳过。');
  });
  const dismiss = eids => run(async () => {
    const data = await call('dismissEpisodeUpdates', {eids});
    if (alive.current) setFeed(data);
  });
  const add = (episodes,where='end') => run(async () => {
    // Use the current player snapshot, preserving playback and existing queue order.
    const current = currentPlayer.current;
    const ids = new Set(current.queue.map(ep => ep.eid));
    const fresh = episodes.filter(ep => {if (ids.has(ep.eid)) return false; ids.add(ep.eid); return true});
    const next = [...current.queue];
    const currentIndex=next.findIndex(ep=>ep.eid===current.current?.eid);
    next.splice(where==='next'?currentIndex+1:next.length,0,...fresh);
    current.setQueue(next);
    // Keep a standalone playing episode's index (-1) unchanged.
    await call('saveQueue', {queue: next.map(queueEpisode), index: current.index});
    const data = await call('dismissEpisodeUpdates', {eids: episodes.map(ep => ep.eid)});
    if (alive.current) {
      setFeed(data);
      setMessage(`已加入 ${fresh.length} 集${episodes.length > fresh.length ? `，${episodes.length-fresh.length} 集已在播放列表` : ''}`);
    }
  });
  const queued = new Set(player.queue.map(ep => ep.eid));
  const episodes = (feed.episodes || []).filter(ep => !queued.has(ep.eid));
  const working = busy || !!feed.checking;
  return h('div', {className: 'xyz-episode-updates'},
    h('div', {className: 'xyz-feed-heading'},
      h('div', {className:'xyz-feed-title'}, '订阅动态', episodes.length ? h('span',{className:'xyz-feed-count'},episodes.length) : null),
      h('button', {type:'button', className:'xyz-btn xyz-subscriptions-entry', onClick:onSubscriptions}, `全部订阅 ${subscriptionCount} ›`)),
    h('div', {className:'xyz-feed-toolbar'},
      h('span', {className:'xyz-hint xyz-feed-status'}, feed.checking ? '正在检查更新…' : feed.checkedAt ? `${relTime(feed.checkedAt).replace(/更新$/, '')}检查` : '尚未检查更新'),
      h('div',{className:'xyz-feed-actions'},
        h('button', {type:'button', className:'xyz-btn xyz-feed-check', disabled:working || !subscriptionCount, onClick:check}, working ? '检查中…' : '检查更新'),
        episodes.length ? h('button', {type:'button', className:'xyz-btn primary xyz-feed-add', disabled:busy, onClick:()=>add(episodes)}, '全部加入') : null)),
    message ? h('div', {className:'xyz-hint', role:'status'}, message) : null,
    feed.failures?.length ? h('div', {className:'xyz-hint', role:'status'}, `${feed.failures.length} 个节目未能检查，已保留原有记录，下次可重试。`) : null,
    !episodes.length ? h('div', {className:'xyz-empty'}, subscriptionCount ? '暂无' : '还没有本地订阅，点「全部订阅」手动拉取一次。') :
      h('div', {className:'xyz-list'}, ...episodes.map(ep => h('div', {key:ep.eid, className:'xyz-feed-item'},
        h(EpisodeRow, {ep:{...ep, image:ep.subscriptionImage || ep.image, podcastTitle:ep.subscriptionTitle || ep.podcastTitle}, player, onAdd:where=>add([ep],where), addDisabled:busy, subtitle:`${ep.subscriptionTitle || ep.podcastTitle || ''}${ep.isCoCreated ? ' · 共创' : ''}${ep.pubDate ? ` · ${relTime(ep.pubDate)}` : ''}${ep.duration ? ` · ${Math.round(ep.duration/60)} 分钟` : ''}`}),
        h('button', {type:'button', className:'xyz-btn xyz-feed-ignore', disabled:busy, title:`忽略：${ep.title}`, onClick:()=>dismiss([ep.eid])}, '忽略')))),
    h('div', {className:'xyz-hint xyz-feed-footnote'}, '仅检查本地订阅 · 订阅列表只在你手动刷新时同步'));
}


export function EpisodeUpdateSettings({call}) {
  const [automatic, setAutomatic] = useState(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const pending = useRef(false);
  useEffect(() => {
    let alive = true;
    call('episodeUpdates').then(data => {if (alive) setAutomatic(data.automatic !== false)}).catch(err => {if (alive) setError(err.message)});
    return () => {alive = false};
  }, []);
  const toggle = async () => {
    if (pending.current || automatic === null) return;
    pending.current = true; setBusy(true); setError('');
    try {
      const data = await call('setEpisodeUpdateAutomatic', {enabled: !automatic});
      setAutomatic(data.automatic);
    } catch (err) {setError(err.message)}
    finally {pending.current = false; setBusy(false)}
  };
  return h('section', {className:'xyz-section', 'aria-label':'节目更新'},
    h('div', {className:'xyz-section-title'}, '节目更新'),
    h('div', {className:'xyz-setting-row'},
      h('div', {className:'xyz-setting-label'},
        h('div', {className:'xyz-setting-name'}, '自动检查新单集'),
        h('div', {className:'xyz-setting-desc'}, '每小时检查本地订阅，不刷新订阅列表')),
      h('button', {type:'button', role:'switch', 'aria-label':'自动检查新单集', 'aria-checked':automatic === true, disabled:busy || automatic === null, className:'xyz-btn', onClick:toggle}, automatic === null ? '读取中…' : automatic ? '已开启' : '已关闭')),
    error ? h('div', {className:'xyz-hint', role:'status'}, error) : null);
}
