import React from 'react';

const h = React.createElement;
const {useState, useRef, useEffect, useCallback} = React;
const initialStatus = {
  version: '', enabled: false, automatic: false, phase: 'idle',
  availableVersion: '', notes: '', message: '', downloaded: 0, total: 0, lastChecked: '',
};
const errorText = error => error?.message || String(error);
const formatDownloadSize = (value, reference=value) => {
  const unit=reference>=1_000_000?1_000_000:1_000;
  return `${(value/unit).toFixed(1)} ${unit===1_000_000?'MB':'KB'}`;
};


async function call(action, body) {
  const result = await window.mygo.call('Backend.Request', action, body);
  if (!result || result.ok === false || result.error) {
    throw new Error(result?.error || '保存播放状态失败');
  }
  return result;
}

/** Mount once alongside the player, outside the settings panel. */
export function useUpdates() {
  const [status, setStatus] = useState(initialStatus);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const mounted = useRef(false);
  const pending = useRef(false);
  const generation = useRef(0);

  // All native requests share one lock, including polling and restart saves.
  const run = useCallback(async task => {
    if (!mounted.current || pending.current) return null;
    pending.current = true;
    const token = generation.current;
    setBusy(true);
    setError('');
    try {
      const next = await task();
      if (mounted.current && generation.current === token) setStatus(next);
      return next;
    } catch (err) {
      if (mounted.current && generation.current === token) setError(errorText(err));
      return null;
    } finally {
      pending.current = false;
      if (mounted.current) setBusy(false);
    }
  }, []);

  const refresh = useCallback(() => run(() => window.mygo.call('Backend.UpdateStatus')), [run]);
  const check = useCallback(() => run(() => window.mygo.call('Backend.CheckUpdates')), [run]);
  const setAutomatic = useCallback(value => run(() => window.mygo.call('Backend.SetAutomaticUpdates', !!value)), [run]);
  const download = useCallback(() => run(() => window.mygo.call('Backend.DownloadUpdate')), [run]);
  const restart = useCallback(player => run(async () => {
    if (status.phase !== 'ready') throw new Error('更新尚未准备好，请重新检查');
    if (!player || !Array.isArray(player.queue)) throw new Error('无法读取播放状态，未重启');
    const audio = player.audioRef?.current;
    if (player.current && !audio) throw new Error('无法读取播放进度，未重启');
    audio?.pause();
    if (player.current) {
      await call('progress', {
        ...player.current,
        position: audio.currentTime,
        duration: Number.isFinite(audio.duration) ? audio.duration : player.current.duration,
      });
    }
    await call('saveQueue', {
      queue: player.queue.map(({eid, pid, title, podcastTitle, image, duration}) =>
        ({eid, pid, title, podcastTitle, image, duration})),
      index: player.index,
    });
    await call('savePrefs', {rate: player.rate, volume: player.volume});
    await window.mygo.call('Backend.RestartForUpdate');
    return status;
  }), [run, status.phase]);

  useEffect(() => {
    mounted.current = true;
    generation.current += 1;
    void refresh();
    return () => { mounted.current = false; generation.current += 1; };
  }, [refresh]);

  useEffect(() => {
    // Transport failures remain visible until an explicit retry.
    if (error || status.phase === 'disabled') return undefined;
    const interval = setInterval(() => { void refresh(); },
      status.phase === 'checking' || status.phase === 'downloading' ? 1000 : 5000);
    return () => clearInterval(interval);
  }, [refresh, status.phase, error]);

  return {status, error, busy, refresh, check, setAutomatic, download, restart};
}

export function UpdateSettings({updates, player}) {
  const {status, error, busy} = updates;
  const working = busy || status.phase === 'checking' || status.phase === 'downloading';
  const failure = error || (status.phase === 'error' ? status.message || '更新失败，请重试' : '');
  const downloaded = Math.max(0, Number(status.downloaded) || 0);
  const total = Math.max(0, Number(status.total) || 0);
  const percent = total ? Math.min(100, Math.round(downloaded / total * 100)) : null;
  const [linkError,setLinkError]=useState('');
  const openRepository=async()=>{
    setLinkError('');
    try{await window.mygo.call('Backend.OpenLink','https://github.com/sgaxun/xiaoyuzhou-player')}
    catch{setLinkError('仓库链接未能打开，请重试。')}
  };
  const action = status.phase==='ready'
    ? ['重启安装',()=>updates.restart(player),'保存进度并重启安装']
    : status.phase==='available'
      ? ['下载更新',updates.download,'下载更新']
      : failure
        ? ['重试',error?updates.refresh:status.availableVersion?updates.download:updates.check,'重试更新']
        : [status.phase==='checking'?'检查中…':status.phase==='downloading'?'下载中…':'重新检查',updates.check,'重新检查'];
  const subtitle = failure ? '' : ({current:'已是最新版本',checking:'正在检查新版本…',available:`可更新至 ${status.availableVersion}`,downloading:`正在获取 ${status.availableVersion||'新版本'}`,ready:'更新已就绪，重启后安装',disabled:status.message||'当前无法在线更新'}[status.phase]||'保持应用为最新版本');
  const divider={borderTop:'1px solid var(--app-border)',paddingTop:10};
  return h('section', {className:'xyz-section','aria-label':'软件更新'},
    h('div', {className:'xyz-section-title'},'软件更新'),
    h('div',{style:{background:'var(--app-hover)',border:'1px solid var(--app-border)',borderRadius:10,padding:12,display:'flex',flexDirection:'column',gap:12}},
      h('div',{style:{display:'flex',justifyContent:'space-between',alignItems:'center',gap:12}},
        h('div',{style:{display:'flex',flexDirection:'column',gap:4,minWidth:0}},
          h('div',{style:{fontSize:13,fontWeight:600}},`版本 ${status.version||'读取中…'}`),
          subtitle?h('div',{style:{fontSize:11,opacity:.6}},subtitle):null),
        h('div',{style:{display:'flex',alignItems:'center',justifyContent:'flex-end',flexWrap:'wrap',gap:8,flexShrink:0}},
          (status.phase==='available'||status.phase==='ready'||failure)?h('button',{type:'button',className:'xyz-btn','aria-label':'重新检查',title:status.phase==='ready'?'更新已就绪，请先安装':'重新获取最新版本',disabled:working||status.phase==='disabled'||status.phase==='ready',onClick:updates.check,style:{fontSize:11,opacity:.75,padding:'5px 0'}},'重新检查'):null,
          h('button',{type:'button',className:'xyz-btn',title:action[2],disabled:working||status.phase==='disabled',onClick:action[1],style:{flexShrink:0,border:'1px solid var(--app-border)',padding:'5px 10px'}},action[0]))),
      status.notes&&status.availableVersion?h('details',{style:{fontSize:11,...divider}},
        h('summary',{style:{cursor:'pointer',opacity:.7}},'更新说明'),
        h('div',{style:{whiteSpace:'pre-wrap',overflowWrap:'anywhere',lineHeight:1.7,opacity:.75,paddingTop:8}},status.notes)):null,
    status.phase === 'downloading' && h('div', {style:{display:'flex',flexDirection:'column',gap:8,padding:'12px 0'}},
      h('div',{style:{display:'flex',justifyContent:'space-between',fontSize:12}},
        h('span',null,'正在下载更新'),
        h('span',{style:{fontVariantNumeric:'tabular-nums',fontWeight:600}},percent===null?'下载中…':`${percent}%`)),
      h('div',{role:'progressbar','aria-label':'更新下载进度','aria-valuemin':0,'aria-valuemax':100,'aria-valuenow':percent===null?undefined:percent,
        'aria-valuetext':total?`${formatDownloadSize(Math.min(downloaded,total),total)} / ${formatDownloadSize(total)}`:formatDownloadSize(downloaded),
        style:{height:5,width:'100%',borderRadius:3,overflow:'hidden',background:'var(--app-track)'}},
        h('div',{style:{height:'100%',width:percent===null?'30%':`${percent}%`,borderRadius:'inherit',background:'var(--app-fg)',transition:'width .25s ease',animation:percent===null?'xyz-update-indeterminate 1.2s ease-in-out infinite':undefined}})),
      h('div',{style:{display:'flex',justifyContent:'space-between',fontSize:11,opacity:.65,fontVariantNumeric:'tabular-nums'}},
        h('span',null,total?`${formatDownloadSize(Math.min(downloaded,total),total)} / ${formatDownloadSize(total)}`:formatDownloadSize(downloaded)),
        h('span',null,'下载后自动校验')),
      percent===null?h('style',null,'@keyframes xyz-update-indeterminate{0%{transform:translateX(-100%)}100%{transform:translateX(430%)}}'):null),
      failure?h('div',{role:'status',style:{fontSize:11,lineHeight:1.5}},failure):null,
      h('div',{style:{...divider,display:'flex',alignItems:'center',justifyContent:'space-between',gap:12}},
        h('label',{style:{display:'flex',alignItems:'center',gap:8,fontSize:11,cursor:'pointer'}},
          h('button',{type:'button',role:'switch','aria-label':'自动检查更新','aria-checked':!!status.automatic,disabled:working||!status.enabled,
            onClick:()=>updates.setAutomatic(!status.automatic),
            style:{width:30,height:18,padding:2,border:0,borderRadius:10,background:status.automatic?'var(--app-fg)':'var(--app-track)',cursor:'inherit',opacity:(working||!status.enabled)?0.45:1,flexShrink:0}},
            h('span',{style:{display:'block',width:14,height:14,borderRadius:'50%',background:status.automatic?'var(--app-bg)':'var(--app-fg)',transform:status.automatic?'translateX(12px)':'translateX(0)',transition:'transform .15s ease'}})),
          '自动检查'),
        h('button',{type:'button',className:'xyz-btn',onClick:openRepository,title:'用默认浏览器打开项目仓库',style:{fontSize:11,padding:'2px 0',opacity:.75}},'GitHub 仓库 ↗')),
      linkError?h('div',{role:'status',style:{fontSize:11}},linkError):null,
    ),
  );
}

export function UpdateBanner({updates, onOpen}) {
  const {status} = updates;
  const [dismissed, setDismissed] = useState('');
  const noticeKey = `${status.availableVersion}:${status.phase}`;
  if ((status.phase !== 'available' && status.phase !== 'ready') || dismissed === noticeKey) return null;
  return h('aside', {className: 'xyz-update-banner', 'aria-label': '版本更新'},
    h('span', {className: 'xyz-update-spark', 'aria-hidden': true}, '✦'),
    h('span', {className: 'xyz-update-label'}, status.phase === 'ready'
      ? `版本 ${status.availableVersion || ''} 已就绪` : `发现新版本 ${status.availableVersion || ''}`),
    h('button', {type: 'button', className: 'xyz-btn xyz-update-action', onClick: onOpen}, status.phase === 'ready' ? '查看安装' : '更新'),
    h('button', {type: 'button', className: 'xyz-btn xyz-update-dismiss', 'aria-label': '暂时关闭更新提醒', title: '暂时关闭', onClick: () => setDismissed(noticeKey)}, '×'));
}
