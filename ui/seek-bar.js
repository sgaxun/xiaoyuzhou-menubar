import React from 'react';
const h = React.createElement;

/** Hover state stays here so pointer movement does not rerender the player. */
export function SeekBar({episodeId, total, position, seekable, onSeek, formatTime}) {
  const [preview, setPreview] = React.useState(null);
  const duration = Number.isFinite(total) && total > 0 ? total : 0;
  const current = Math.max(0, Math.min(duration, Number(position) || 0));
  const ratioAt = event => {
    const rect = event.currentTarget.getBoundingClientRect();
    return rect.width > 0 ? Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)) : 0;
  };
  const previewAt = event => {
    if (!duration || event.pointerType === 'touch') return;
    setPreview({episodeId, ratio: ratioAt(event)});
  };
  const showing = duration > 0 && preview && preview.episodeId === episodeId;
  const keySeek = event => {
    if (!seekable || !duration) return;
    const next = {ArrowLeft:current-5, ArrowRight:current+5, Home:0, End:duration}[event.key];
    if (next === undefined) return;
    event.preventDefault();
    onSeek(Math.max(0, Math.min(duration, next)));
  };
  return h('div', {
    className:'xyz-seek', role:'slider', tabIndex:0,
    'aria-label':'播放进度', 'aria-valuemin':0, 'aria-valuemax':duration,
    'aria-valuenow':current, 'aria-valuetext':`${formatTime(current)} / ${formatTime(duration)}`,
    'aria-disabled':!seekable,
    onPointerEnter:previewAt, onPointerMove:previewAt,
    onPointerLeave:()=>setPreview(null), onPointerCancel:()=>setPreview(null), onBlur:()=>setPreview(null),
    onClick:seekable && duration ? event=>onSeek(ratioAt(event)*duration) : undefined,
    onKeyDown:keySeek,
  },
    h('div', {className:'xyz-seek-track', 'aria-hidden':true}, h('i', {style:{width:`${duration ? current/duration*100 : 0}%`}})),
    showing ? h('span', {className:'xyz-seek-preview', 'aria-hidden':true, style:{left:`clamp(32px, ${preview.ratio*100}%, calc(100% - 32px))`}}, formatTime(preview.ratio*duration)) : null);
}
