import React from 'react';

const h = React.createElement;
const {useState, useRef, useEffect, useCallback} = React;
const initialStatus = {
  version: '', enabled: false, automatic: false, phase: 'idle',
  availableVersion: '', notes: '', message: '', downloaded: 0, total: 0, lastChecked: '',
};
const errorText = error => error?.message || String(error);

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
  const row = (label, control) => h('div', {className: 'xyz-setting-row'},
    h('div', {className: 'xyz-setting-name'}, label), control);
  const button = (label, onClick, disabled = working) => h('button', {
    type: 'button', className: 'xyz-btn', disabled, onClick,
  }, label);
  return h('div', {className: 'xyz-section'},
    h('div', {className: 'xyz-section-title'}, '软件更新'),
    row('当前版本', h('span', null, status.version || '读取中…')),
    row('自动检查更新', h('input', {
      type: 'checkbox', 'aria-label': '自动检查更新', checked: !!status.automatic,
      disabled: working || !status.enabled,
      onChange: event => updates.setAutomatic(event.target.checked),
    })),
    row(status.phase === 'current' ? '已是最新版本' : '检查新版本',
      button(status.phase === 'checking' ? '检查中…' : '检查更新', updates.check,
        working || status.phase === 'disabled')),
    status.phase === 'disabled' && h('div', {className: 'xyz-setting-desc'}, status.message || '此版本不支持在线更新'),
    status.availableVersion && h('div', null, `新版本 ${status.availableVersion}`),
    status.notes && h('div', {className: 'xyz-setting-desc', style: {whiteSpace: 'pre-wrap', overflowWrap: 'anywhere'}}, status.notes),
    status.phase === 'downloading' && h('div', {'aria-live': 'polite'},
      h('div', null, percent === null ? `正在下载：${downloaded} 字节` : `正在下载：${percent}%（${downloaded} / ${total} 字节）`),
      h('progress', {max: total || undefined, value: total ? Math.min(downloaded, total) : undefined, 'aria-label': '更新下载进度'})),
    status.phase === 'available' && button('下载更新', updates.download),
    status.phase === 'ready' && button('保存进度并重启安装', () => updates.restart(player)),
    !failure && status.message && status.phase !== 'disabled' && h('div', {className: 'xyz-setting-desc'}, status.message),
    failure && h('div', {role: 'status'},
      h('div', null, failure), button('重试', error ? updates.refresh : status.availableVersion ? updates.download : updates.check, busy)),
  );
}

export function UpdateBanner({updates, onOpen}) {
  const {status} = updates;
  if (status.phase !== 'available' && status.phase !== 'ready') return null;
  return h('div', {className: 'xyz-setting-row'},
    h('span', {className: 'xyz-setting-desc'}, status.phase === 'ready'
      ? `更新 ${status.availableVersion || ''} 已下载` : `有新版本 ${status.availableVersion || ''}`),
    h('button', {type: 'button', className: 'xyz-btn', onClick: onOpen}, '查看更新'));
}
