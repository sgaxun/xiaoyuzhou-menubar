import React from 'react';
import {PodcastContext,PodcastLink} from './podcast-link.js';
import {PodcastDetails} from './podcast-details.js';
import {CoverPreview, openCover} from './cover-preview.js';
import {queueEpisode,episodeImage,episodePodcast} from './episode.js';
import {SeekBar} from './seek-bar.js';
import {Transcript} from './transcript.js';
import {Comments} from './comments.js';
import {PinWindow} from './pin-window.js';
import {MenubarTranscript,MenubarTranscriptSettings} from './menubar-transcript.js';
import {FullTitle} from './full-title.js';
import {QueueAdd} from './queue-add.js';
import {BackupSettings, SleepTimer, EpisodeActions} from './listening-tools.js';
import {ShareDialog} from './share.js';
import {EpisodeUpdates, EpisodeUpdateSettings} from './episode-updates.js';
import {useUpdates,UpdateSettings,UpdateBanner} from './updates.js';
import DOMPurify from 'dompurify';
import QRCode from 'qrcode';
import {createRoot} from 'react-dom/client';
async function request(action,body) {
 const data=await window.mygo.call('Backend.Request',action,body===undefined?null:body);
 return data;
}
document.addEventListener('keydown',e=>{if(e.key==='Escape'||(e.metaKey&&e.key==='w')){e.preventDefault();window.mygo.call('Backend.Hide')}});
const NotesContext=React.createContext(()=>{});
const playerRef = {current:null}; const listeners=new Set(); const mountedRef={current:false}; const mountedListeners=new Set();

    const h = (tag, props, ...children) => {
      if (tag === 'img' && props?.alt !== '小宇宙登录二维码' && !props?.className?.includes('xyz-avatar')) {
        props = {...props, role: 'button', tabIndex: 0, 'aria-label': '放大封面',
          style: {...props.style, cursor: 'zoom-in'}, onClick: openCover,
          onKeyDown: event => {if (event.key === 'Enter' || event.key === ' ') openCover(event);}};
      }
      return React.createElement(tag, tag === 'button' && props?.title ? {...props, 'aria-label': props['aria-label'] || props.title} : props, ...children);
    };
    const { useState, useEffect, useRef, useCallback, useMemo } = React;

    const API = '/xiaoyuzhou/api';
    const RATES = [0.75, 1, 1.25, 1.5, 2];

    // 用 SVG 画图标：字符图标（▶ ❚❚ ⏮）在不同字体下会渲染成方块。
    const svg = (children, size = 14) =>
      React.createElement(
        'svg',
        {
          width: size, height: size, viewBox: '0 0 24 24', fill: 'currentColor',
          'aria-hidden': true, style: { display: 'block', flex: '0 0 auto' },
        },
        ...children,
      );

    const IconPlay = (size) => svg([React.createElement('path', { key: 'p', d: 'M8 5.14v13.72a1 1 0 0 0 1.53.85l10.5-6.86a1 1 0 0 0 0-1.7L9.53 4.29A1 1 0 0 0 8 5.14z' })], size);
    const IconPause = (size) => svg([
      React.createElement('rect', { key: 'a', x: 7, y: 5, width: 3.6, height: 14, rx: 1 }),
      React.createElement('rect', { key: 'b', x: 13.4, y: 5, width: 3.6, height: 14, rx: 1 }),
    ], size);
    const IconPrev = (size) => svg([
      React.createElement('rect', { key: 'a', x: 5, y: 5, width: 2.6, height: 14, rx: 1 }),
      React.createElement('path', { key: 'b', d: 'M19 6.2v11.6a1 1 0 0 1-1.55.83L9 12.83a1 1 0 0 1 0-1.66l8.45-5.8A1 1 0 0 1 19 6.2z' }),
    ], size);
    const IconNext = (size) => svg([
      React.createElement('path', { key: 'b', d: 'M5 6.2v11.6a1 1 0 0 0 1.55.83L15 12.83a1 1 0 0 0 0-1.66L6.55 5.37A1 1 0 0 0 5 6.2z' }),
      React.createElement('rect', { key: 'a', x: 16.4, y: 5, width: 2.6, height: 14, rx: 1 }),
    ], size);
    const IconJump = (seconds, forward) => svg([
      h('g', {key:'ring', transform: forward ? undefined : 'translate(24 0) scale(-1 1)'},
        h('path',{d:'M17.8 5.8A8.7 8.7 0 1 0 20.6 13',fill:'none',stroke:'currentColor',strokeWidth:1.7,strokeLinecap:'round'}),
        h('path',{d:'M17.8 1.8v4.7h-4.7',fill:'none',stroke:'currentColor',strokeWidth:1.7,strokeLinecap:'round',strokeLinejoin:'round'})),
      h('text',{key:'number',x:12,y:15.6,textAnchor:'middle',fontSize:9,fontWeight:600,fontFamily:'system-ui, sans-serif',fill:'currentColor'},String(seconds))
    ],24);

    const IconList = (size) => svg([
      React.createElement('path', { key: 'a', d: 'M4 6h16M4 12h16M4 18h10', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round', fill: 'none' }),
    ], size);
    const IconMic = (size) => svg([
      React.createElement('path', { key: 'a', d: 'M12 3a3 3 0 0 1 3 3v6a3 3 0 0 1-6 0V6a3 3 0 0 1 3-3z' }),
      React.createElement('path', { key: 'b', d: 'M6 11a6 6 0 0 0 12 0M12 17v4M8.5 21h7', stroke: 'currentColor', strokeWidth: 1.8, fill: 'none', strokeLinecap: 'round' }),
    ], size);

    async function call(action, body) {
      const data = await request(action, body);
      if (!data.ok && data.error) { const e = new Error(data.error); e.needsLogin = !!data.needsLogin; throw e; }
      return data;
    }
    async function apiFetch(url) {
      const action = String(url).split('/api/')[1];
      const data = await call(action);
      return {ok: data.ok, json: async () => data};
    }

    /** "3 分钟前" 这样的相对时间，用于展示缓存年龄。 */
    const relTime = (ts) => {
      if (!ts) return '';
      const time = typeof ts === 'number' ? ts : Date.parse(ts);
      if (!Number.isFinite(time)) return '时间未知';
      const diff = Math.max(0, Date.now() - time);
      if (diff < 60000) return '刚刚更新';
      const min = Math.floor(diff / 60000);
      if (min < 60) return `${min} 分钟前更新`;
      const hr = Math.floor(min / 60);
      if (hr < 24) return `${hr} 小时前更新`;
      return `${Math.floor(hr / 24)} 天前更新`;
    };

    const mmss = (sec) => {
      const s = Math.max(0, Math.floor(Number(sec) || 0));
      const m = Math.floor(s / 60);
      const r = s % 60;
      if (m >= 60) return `${Math.floor(m / 60)}:${String(m % 60).padStart(2, '0')}:${String(r).padStart(2, '0')}`;
      return `${m}:${String(r).padStart(2, '0')}`;
    };

    const imageOf = (url) => url || '';

    // ---- 样式（仅使用 --dsw-alias-* 主题变量） ----
    const styles = `
    .xyz-bar{display:flex;align-items:center;gap:10px;width:100%;padding:6px 10px;border-radius:10px;
      background:var(--dsw-alias-bg-layer-2,rgba(127,127,127,.06));border:1px solid var(--dsw-alias-border-l1,rgba(127,127,127,.16));
      font-size:12px;color:var(--dsw-alias-label-primary,inherit);box-sizing:border-box}
    .xyz-bar img{width:30px;height:30px;border-radius:6px;object-fit:cover;flex:0 0 auto;background:var(--dsw-alias-bg-layer-2,rgba(127,127,127,.12))}
    .xyz-meta{min-width:0;flex:1 1 auto;display:flex;flex-direction:column;gap:2px}
    .xyz-title{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:600;font-size:12px}
    .xyz-sub{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;opacity:.65;font-size:11px}
    .xyz-btn{appearance:none;border:none;background:transparent;color:inherit;cursor:pointer;border-radius:7px;
      padding:4px 7px;font-size:12px;line-height:1;display:inline-flex;align-items:center;justify-content:center;gap:4px}
    .xyz-btn:hover{background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 10%, transparent)}
    .xyz-btn[disabled]{opacity:.4;cursor:default}
    .xyz-btn.primary{
      background:var(--dsw-alias-brand-primary,#5b6bf5);
      color:var(--dsw-alias-bg-base,#fff);font-weight:600}
    .xyz-btn.primary:hover{background:var(--dsw-alias-brand-primary,#5b6bf5);filter:brightness(1.08)}
    .xyz-progress{position:relative;height:4px;border-radius:2px;background:var(--dsw-alias-bg-layer-2,rgba(127,127,127,.2));
      flex:0 1 160px;cursor:pointer;min-width:70px}
    .xyz-progress>i{position:absolute;inset:0 auto 0 0;background:var(--dsw-alias-brand-primary,#5b6bf5);border-radius:2px}
    .xyz-time{font-variant-numeric:tabular-nums;opacity:.7;font-size:11px;white-space:nowrap}
    .xyz-panel{position:fixed;left:264px;bottom:16px;width:min(440px,calc(100vw - 288px));
      max-width:calc(100vw - 24px);
      height:min(600px,calc(100vh - 96px));max-height:calc(100vh - 96px);min-height:300px;
      display:flex;flex-direction:column;border-radius:14px;overflow:hidden;z-index:2147483000;pointer-events:auto;
      background:var(--dsw-alias-bg-overlay,#1e1e22);color:var(--dsw-alias-label-primary,#eee);
      border:1px solid var(--dsw-alias-border-l2,rgba(127,127,127,.35));box-shadow:0 18px 48px rgba(0,0,0,.45)}
    .xyz-panel-head{display:flex;align-items:center;gap:8px;padding:10px 12px;flex:0 0 auto;border-bottom:1px solid var(--dsw-alias-border-l1,rgba(127,127,127,.2))}
    .xyz-tabs{display:flex;gap:4px;flex:1 1 auto}
    .xyz-tab{appearance:none;border:none;background:transparent;color:inherit;cursor:pointer;padding:5px 10px;border-radius:7px;font-size:12px;opacity:.7}
    .xyz-tab[data-active="true"]{background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 12%, transparent);opacity:1;font-weight:600}
    .xyz-panel-body{overflow-y:auto;overflow-x:hidden;padding:10px 12px 14px;flex:1 1 auto;min-height:0}
    .xyz-input{width:100%;box-sizing:border-box;padding:8px 10px;border-radius:8px;font-size:12px;
      border:1px solid var(--dsw-alias-border-l2,rgba(127,127,127,.24));
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 5%, transparent);color:inherit}
    .xyz-row{display:flex;align-items:center;gap:10px;padding:7px 4px;border-radius:8px;cursor:pointer}
    .xyz-row:hover{background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 7%, transparent)}
    .xyz-row img{width:38px;height:38px;border-radius:8px;object-fit:cover;flex:0 0 auto;background:var(--dsw-alias-bg-layer-2,rgba(127,127,127,.12))}
    .xyz-row-main{min-width:0;flex:1 1 auto;display:flex;flex-direction:column;gap:2px}
    .xyz-row-t{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;font-weight:600}
    .xyz-row-s{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:11px;opacity:.62}
    .xyz-hint{font-size:11px;opacity:.62;line-height:1.6}
    .xyz-err{font-size:11px;color:#e5484d;line-height:1.6}
    .xyz-empty{font-size:12px;opacity:.6;padding:14px 4px;text-align:center}
    .xyz-chip{display:inline-flex;align-items:center;gap:4px;font-size:10px;padding:1px 6px;border-radius:999px;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 12%, transparent);opacity:.85}
    .xyz-list{display:flex;flex-direction:column;gap:1px}
    .xyz-card-wrap{width:100%;box-sizing:border-box;padding:0 8px 8px;align-self:stretch;min-width:0}
    .xyz-card{display:flex;flex-direction:column;gap:7px;padding:10px;border-radius:12px;
      width:100%;box-sizing:border-box;align-self:stretch;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 6%, transparent);
      border:1px solid var(--dsw-alias-border-l1,rgba(127,127,127,.16));
      font-size:12px;color:var(--dsw-alias-label-primary,inherit);box-sizing:border-box;min-width:0}
    .xyz-card-top{display:flex;align-items:center;gap:9px;min-width:0;cursor:pointer;border-radius:6px}
    .xyz-cover{width:36px;height:36px;border-radius:7px;object-fit:cover;flex:0 0 auto;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 10%, transparent)}
    .xyz-cover.empty{display:grid;place-items:center;opacity:.65}
    .xyz-card-text{min-width:0;flex:1 1 auto;display:flex;flex-direction:column;gap:2px}
    .xyz-card-title{font-size:12px;font-weight:600;line-height:1.3;overflow:hidden;
      display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;word-break:break-word;
      max-height:2.6em}
    .xyz-card-sub{font-size:11px;line-height:1.3;opacity:.6;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .xyz-card-duration{align-self:flex-end;flex:0 0 auto;font-size:11px;line-height:1.3;opacity:.55;font-variant-numeric:tabular-nums;white-space:nowrap}
    .xyz-card-sub.err{color:var(--dsw-alias-state-error-primary,#e5484d);opacity:1;white-space:normal;
      display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical}
    .xyz-seek{position:relative;height:15px;margin:-6px 0;cursor:pointer;flex-shrink:0}
    .xyz-seek-track{position:absolute;left:0;right:0;top:50%;transform:translateY(-50%);height:3px;border-radius:2px;overflow:hidden}
    .xyz-seek:hover .xyz-seek-track{height:4px}
    .xyz-seek-track>i{display:block;height:100%;border-radius:inherit}
    .xyz-seek-preview{position:absolute;bottom:calc(100% + 2px);transform:translateX(-50%);width:64px;box-sizing:border-box;padding:4px 6px;border-radius:5px;background:var(--app-play);color:var(--app-play-ink);font-size:11px;font-weight:500;line-height:16px;text-align:center;font-variant-numeric:tabular-nums;white-space:nowrap;pointer-events:none;z-index:2;box-shadow:0 2px 8px #0002}
    .xyz-card-foot{display:flex;align-items:center;justify-content:space-between;gap:6px;min-height:28px}
    .xyz-tnum{font-size:10.5px;font-variant-numeric:tabular-nums;opacity:.5;white-space:nowrap;flex:0 0 auto}
    .xyz-controls{display:flex;align-items:center;gap:0;flex:0 0 auto}
    .xyz-icon-btn{appearance:none;border:none;background:transparent;color:inherit;cursor:pointer;
      width:26px;height:26px;border-radius:7px;display:inline-flex;align-items:center;justify-content:center;padding:0;
      opacity:.82;transition:background .12s,opacity .12s,transform .12s}
    .xyz-icon-btn:active{transform:scale(.94)}
    .xyz-icon-btn:hover{background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 10%, transparent);opacity:1}
    .xyz-icon-btn:disabled{opacity:.28;cursor:default;background:transparent}
    .xyz-icon-btn.accent{
      background:var(--dsw-alias-brand-primary,#5b6bf5);
      color:var(--dsw-alias-bg-base,#fff);
      opacity:1;width:28px;height:28px;border-radius:14px}
    .xyz-icon-btn.accent:hover{filter:brightness(1.08);background:var(--dsw-alias-brand-primary,#5b6bf5)}
    .xyz-icon-btn.accent:disabled{opacity:.4}
    .xyz-rate{appearance:none;border:none;background:transparent;color:inherit;cursor:pointer;
      font-size:10.5px;font-variant-numeric:tabular-nums;padding:0 3px;height:26px;border-radius:6px;opacity:.6;
      min-width:24px;text-align:center}
    .xyz-rate:hover{background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 10%, transparent);opacity:1}
    .xyz-rail{display:flex;align-items:center;justify-content:center;padding:4px 0}
    .xyz-toolbar{display:flex;align-items:center;gap:6px;padding-bottom:2px}
    .xyz-section{display:flex;flex-direction:column;gap:8px}
    .xyz-section-title{font-size:11px;font-weight:600;opacity:.5;letter-spacing:.04em}
    .xyz-account{display:flex;align-items:center;gap:10px;padding:8px;border-radius:10px;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 6%, transparent)}
    .xyz-avatar{width:34px;height:34px;border-radius:17px;object-fit:cover;flex:0 0 auto;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 10%, transparent)}
    .xyz-avatar.empty{display:grid;place-items:center;opacity:.6}
    .xyz-account-main{flex:1 1 auto;min-width:0;display:flex;flex-direction:column;gap:2px}
    .xyz-account-name{font-size:12.5px;font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .xyz-account-sub{font-size:11px;opacity:.55}
    .xyz-setting-row{display:flex;align-items:center;gap:10px;justify-content:space-between}
    .xyz-setting-label{min-width:0;display:flex;flex-direction:column;gap:2px}
    .xyz-setting-name{font-size:12px}
    .xyz-setting-desc{font-size:11px;opacity:.5}
    .xyz-seg{display:inline-flex;gap:2px;padding:2px;border-radius:8px;flex:0 0 auto;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 7%, transparent)}
    .xyz-seg-item{appearance:none;border:none;background:transparent;color:inherit;cursor:pointer;
      font-size:11px;font-variant-numeric:tabular-nums;padding:3px 7px;border-radius:6px;opacity:.6}
    .xyz-seg-item:hover{opacity:.9}
    .xyz-seg-item[data-active="true"]{background:var(--dsw-alias-brand-primary,#5b6bf5);
      color:var(--dsw-alias-bg-base,#fff);opacity:1;font-weight:600}
    .xyz-search{display:flex;align-items:center;gap:6px;padding:0 8px;height:30px;border-radius:8px;
      border:1px solid var(--dsw-alias-border-l1,rgba(127,127,127,.18));
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 5%, transparent);
      transition:border-color .12s}
    .xyz-search:focus-within{border-color:var(--dsw-alias-brand-primary,#5b6bf5)}
    .xyz-search-icon{opacity:.45;flex:0 0 auto}
    .xyz-search-input{flex:1 1 auto;min-width:0;appearance:none;border:none;outline:none;background:transparent;
      color:var(--dsw-alias-label-primary,#ececef);font-size:12px;padding:0;height:100%;font-family:inherit;
      -webkit-text-fill-color:var(--dsw-alias-label-primary,#ececef)}
    .xyz-search-input::placeholder{color:inherit;opacity:.4}
    .xyz-search-input::-webkit-search-cancel-button{display:none}
    .xyz-search-clear{appearance:none;border:none;background:transparent;color:inherit;cursor:pointer;
      width:18px;height:18px;border-radius:9px;display:inline-flex;align-items:center;justify-content:center;
      padding:0;opacity:.45;flex:0 0 auto}
    .xyz-search-clear:hover{opacity:.9;background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 12%, transparent)}
    .xyz-row-hover{padding:7px 6px}
    .xyz-row-hover:hover{background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 7%, transparent)}
    .xyz-row-action{appearance:none;border:none;background:transparent;color:inherit;cursor:pointer;
      width:26px;height:26px;border-radius:6px;display:inline-flex;align-items:center;justify-content:center;
      opacity:.28;transition:opacity .12s,background .12s;flex:0 0 auto}
    .xyz-row-hover:hover .xyz-row-action{opacity:.85}
    .xyz-row-action:hover{opacity:1 !important;background:var(--dsw-alias-brand-primary,#5b6bf5);color:#fff}
    .xyz-row-action.added{opacity:1;color:var(--dsw-alias-brand-primary,#5b6bf5)}
    .xyz-row-action.added:hover{background:transparent !important;color:var(--dsw-alias-brand-primary,#5b6bf5)}
    .xyz-row-chevron{display:inline-flex;align-items:center;opacity:0;transition:opacity .12s;flex:0 0 auto}
    .xyz-row-hover:hover .xyz-row-chevron{opacity:.45}
    .xyz-resize{position:absolute;right:0;bottom:0;width:16px;height:16px;cursor:nwse-resize;
      display:flex;align-items:flex-end;justify-content:flex-end;opacity:.28;transition:opacity .12s}
    .xyz-resize:hover{opacity:.7}
    .xyz-resize::after{content:'';width:8px;height:8px;margin:0 3px 3px 0;
      border-right:2px solid currentColor;border-bottom:2px solid currentColor;border-radius:0 0 2px 0}
    .xyz-card-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}
    .xyz-tile{appearance:none;text-align:left;cursor:pointer;display:flex;flex-direction:column;gap:5px;
      padding:10px;border-radius:10px;font:inherit;color:inherit;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 6%, transparent);
      border:1px solid var(--dsw-alias-border-l1,rgba(127,127,127,.16));
      transition:background .12s,border-color .12s}
    .xyz-tile:hover{background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 10%, transparent);
      border-color:var(--dsw-alias-border-l2,rgba(127,127,127,.3))}
    .xyz-tile-title{font-size:12.5px;font-weight:600;line-height:1.3}
    .xyz-tile-sub{font-size:11px;opacity:.5}
    .xyz-tile-thumbs{display:flex;gap:3px;margin-top:2px}
    .xyz-tile-thumbs img{width:26px;height:26px;border-radius:5px;object-fit:cover;flex:0 0 auto;
      background:color-mix(in srgb, var(--dsw-alias-label-primary,#888) 10%, transparent)}
    .xyz-rank{width:18px;flex:0 0 auto;text-align:center;font-size:11px;font-weight:600;
      opacity:.35;font-variant-numeric:tabular-nums}
    .xyz-row:hover .xyz-rank{opacity:.7}
    .xyz-grip{display:inline-flex;align-items:center;justify-content:center;width:12px;flex:0 0 auto;
      opacity:.22;cursor:grab;color:inherit}
    .xyz-row:hover .xyz-grip{opacity:.5}
    .xyz-grip:active{cursor:grabbing}
    .xyz-dragging{opacity:.4}
    .xyz-drop-target{box-shadow:inset 0 2px 0 0 var(--dsw-alias-brand-primary,#5b6bf5)}
    .xyz-row-active{background:color-mix(in srgb, var(--dsw-alias-brand-primary,#5b6bf5) 12%, transparent)}
    .xyz-row-active .xyz-row-t{color:var(--dsw-alias-brand-primary,#5b6bf5)}
    .xyz-empty{padding:22px 8px;text-align:center;font-size:12px;opacity:.55;line-height:1.7}
    @media (max-width:760px){
      .xyz-panel{left:12px;width:calc(100vw - 24px)}
    }
    `;

    function Styles() {
      return h('style', { 'data-xyz': 'style' }, styles + '\n' + hostStyle.textContent);
    }

    function usePlayer() {
      const audioRef = useRef(null);
      const [current, setCurrent] = useState(null);
      const [queue, setQueue] = useState([]);
      const [index, setIndex] = useState(0);
      const [playing, setPlaying] = useState(false);
      const [position, setPosition] = useState(0);
      const [duration, setDuration] = useState(0);
      const [rate, setRate] = useState(1);
      const [volume, setVolume] = useState(0.8);
      const [error, setError] = useState('');
      const [library,setLibrary] = useState({bookmarks:{},completed:{},podcastRates:{}});
      const libraryRef=useRef(library), libraryWrites=useRef(Promise.resolve());
      const [playStatus,setPlayStatus]=useState('idle');
      const playRequestRef=useRef(0);
      const rateRef=useRef(rate);rateRef.current=rate;
      const [sleepMode,setSleepModeState]=useState('off');
      const sleepRef=useRef({mode:'off',deadline:0});
      const [sleepRemaining,setSleepRemaining]=useState(0);
      const setSleepMode=useCallback(value=>{
        const minutes=Number(value);
        const valid=Number.isFinite(minutes)&&minutes>0&&Number.isSafeInteger(Math.round(Date.now()+minutes*60000))&&Date.now()+minutes*60000<=8640000000000000;
        const mode=value==='end'?'end':(valid?String(minutes):'off');
        sleepRef.current={mode,deadline:mode!=='off'&&mode!=='end'?Date.now()+minutes*60000:0};
        setSleepModeState(mode);setSleepRemaining(mode!=='off'&&mode!=='end'?minutes*60:0);
      },[]);
      useEffect(()=>{
        const tick=()=>{const t=sleepRef.current;if(!t.deadline)return;
          setSleepRemaining(Math.max(0,Math.ceil((t.deadline-Date.now())/1000)));
          if(Date.now()>=t.deadline){playRequestRef.current++;audioRef.current?.pause();setPlayStatus('paused');setSleepMode('off')}
        };
        const timer=setInterval(tick,1000);return()=>clearInterval(timer);
      },[setSleepMode]);
      const hydrateLibrary=useCallback(value=>{const next={bookmarks:{},completed:{},podcastRates:{},...(value||{})};libraryRef.current=next;setLibrary(next)},[]);
      const changeLibrary=useCallback(update=>{
        const next=update(libraryRef.current);libraryRef.current=next;setLibrary(next);
        const pending=libraryWrites.current.catch(()=>{}).then(()=>call('saveLibrary',next));
        libraryWrites.current=pending;
        pending.catch(err=>setError(`本地数据保存失败：${err.message}`));
        return pending;
      },[]);
      const enqueue=useCallback((ep,where='end')=>{
        if(!ep?.eid)return;
        setQueue(prev=>{
          if(currentRef.current?.eid===ep.eid)return prev;
          const next=prev.filter(item=>item.eid!==ep.eid);
          const currentIndex=next.findIndex(item=>item.eid===currentRef.current?.eid);
          const at=where==='next'?currentIndex+1:next.length;
          next.splice(at,0,ep);setIndex(currentIndex);return next;
        });
      },[]);
      const lastSent = useRef(0);
      const currentRef = useRef(null);
      currentRef.current = current;
      // 本地播放进度表：{ eid: { position, updatedAt } }，由 PlayerDock 注入。
      // 用于重启后从上次位置继续。
      const progressRef = useRef({});
      // A chosen position can precede loading the media. It must survive metadata
      // and pause events until the audio element actually reaches that position.
      const pendingSeekRef = useRef(null);
      // 链接失效后自动换新链接的重试次数（每个单集最多 2 次，避免死循环）
      const retryRef = useRef({});

      // Show the saved resume point before loading audio, without autoplaying.
      useEffect(() => {
        const audio=audioRef.current;
        if(current?.eid && audio?.dataset.xyzEid===current.eid && audio.readyState>0)return;
        const saved=Number(progressRef.current?.[current?.eid]?.position)||Number(current?.resumeAt)||0;
        const total=Number(current?.duration)||0;
        setPosition(Math.max(0,total>0?Math.min(saved,total):saved));
        setDuration(0);
      }, [current?.eid]);

      const sendProgress = useCallback((ep, pos, forced, reset = false) => {
        if (!ep || !ep.eid) return;
        const pending = pendingSeekRef.current;
        if (!reset && pending?.eid === ep.eid) pos = pending.position;
        if(Number.isFinite(pos)) {
          const previous=Number(progressRef.current?.[ep.eid]?.position)||0;
          progressRef.current[ep.eid]={position:reset?pos:Math.max(previous,pos)};
        }
        const now = Date.now();
        if (!forced && now - lastSent.current < 15000) return;
        lastSent.current = now;
        return call('progress', {
          ...queueEpisode(ep),
          position: Math.round(pos),
          reset,
        }).catch(() => {});
      }, []);

      const play = useCallback(
        async (episode, resumeAt, options = {}) => {
          const requestId=++playRequestRef.current;
          setError('');
          setPlayStatus('loading');
          const audio = audioRef.current;
          if (!audio) return;
          setCurrent(episode);
          currentRef.current=episode;
          const programKey=episode.subscriptionPid||episode.pid;
          const effectiveRate=Number(libraryRef.current.podcastRates[programKey])||rate;
          setRate(effectiveRate);

          const allowResume = options.resume !== false;
          if (!allowResume && episode.eid) {
            pendingSeekRef.current = null;
            // 显式从头播放：把这一集的进度归零，否则"只前进"保护会让它停在旧位置。
            if (progressRef.current?.[episode.eid]) progressRef.current[episode.eid].position = 0;
            call('progress', {
              eid: episode.eid,
              pid: episode.pid,
              title: episode.title,
              podcastTitle: episode.podcastTitle,
              image: episode.image,
              duration: episode.duration,
              position: 0,
              reset: true,
            }).catch(() => {});
          }
          const savedPos = allowResume
            ? Number(progressRef.current?.[episode.eid]?.position) || 0
            : 0;
          let start = Number.isFinite(resumeAt)
            ? resumeAt
            : savedPos || episode.resumeAt || 0;
          if(Number.isFinite(resumeAt))pendingSeekRef.current={eid:episode.eid,position:Math.max(0,resumeAt)};
          audio.playbackRate = effectiveRate;
          audio.volume = volume;

          /** 等某个媒体事件（或条件已满足）。 */
          const waitFor = (event, timeout) =>
            new Promise((resolve) => {
              if (event === 'loadedmetadata' && audio.readyState >= 1) return resolve(true);
              if (event === 'playing' && !audio.paused && audio.readyState >= 3) return resolve(true);
              let settled = false;
              const done = (ok) => {
                if (settled) return;
                settled = true;
                clearTimeout(timer);
                clearInterval(poll);
                audio.removeEventListener(event, onEvent);
                resolve(ok);
              };
              const onEvent = () => done(true);
              const timer = setTimeout(() => done(false), timeout);
              audio.addEventListener(event, onEvent, { once: true });
              const poll = setInterval(() => {
                if (event === 'loadedmetadata' && audio.readyState >= 1) done(true);
                if (event === 'playing' && !audio.paused && audio.readyState >= 3) done(true);
              }, 150);
            });

          /**
           * 把位置设到 sec 并等它真正落位。
           *
           * 关键：seekable 为空时设置 currentTime 会被浏览器**直接忽略**，
           * 表现为 seeking 正常切换但位置纹丝不动。所以必须先等 seekable 覆盖目标。
           */
          const seekTo = (sec, timeoutMs) =>
            new Promise((resolve) => {
              const el = audioRef.current || audio;
              const deadline = Date.now() + timeoutMs;

              const seekableReady = () => {
                if (!el.seekable || el.seekable.length === 0) return false;
                const end = el.seekable.end(el.seekable.length - 1);
                return end > sec;
              };

              const waitSeekable = () =>
                new Promise((done) => {
                  const check = () => {
                    if (seekableReady()) return done(true);
                    if (Date.now() > deadline) return done(false);
                    setTimeout(check, 120);
                  };
                  check();
                });

              const applyAndWait = () =>
                new Promise((done) => {
                  let tries = 0;
                  const tick = () => {
                    if (Math.abs(el.currentTime - sec) < 2 && !el.seeking) return done(true);
                    if (tries++ > 30 || Date.now() > deadline) {
                      return done(false);
                    }
                    setTimeout(tick, 100);
                  };
                  try {
                    el.currentTime = sec;
                    setPosition(sec);
                  } catch (e) {
                    return done(false);
                  }
                  tick();
                });

              (async () => {
                if (!(await waitSeekable())) return resolve(false);
                resolve(await applyAndWait());
              })();
            });

          // 音频源：优先直连小宇宙 CDN。
          // 实测经本地代理的流在本环境下 seekable 为 [0,0]，任何 seek 都会被忽略，
          // 导致"续播无效"；直连 CDN 的 seekable 正常。
          // 代理仅作为兜底（例如直连被拒时）。
          let url = `${API.replace('/api', '/audio')}?eid=${encodeURIComponent(episode.eid)}`;
          try {
            const r = await apiFetch(`${API}/audioUrl?eid=${encodeURIComponent(episode.eid)}`);
            const d = await r.json();
            if (d?.ok && d.url) {
              url = d.url;
            } else if (d?.paid) {
              // 付费且未购买：明确提示，而不是静默从头播
              setError(d.error || '该单集需先在小宇宙 App 内购买');setPlayStatus('error');
              return;
            }
          } catch {
            /* 取不到就退回代理 */
          }
          if(requestId!==playRequestRef.current)return;
          const sameEpisode = audio.dataset.xyzEid === episode.eid && audio.readyState > 0;
          if (!sameEpisode) {
            audio.dataset.xyzEid = episode.eid;
            audio.src = url;
            await waitFor('loadedmetadata', 8000);
          }

          if(requestId!==playRequestRef.current)return;
          const requested = pendingSeekRef.current;
          if (allowResume && requested?.eid === episode.eid) start = requested.position;
          if (start > 0 || (allowResume && requested?.eid === episode.eid)) {

            // 同源同集时直接 seek 即可；数据已在，落位很快。
            let ok = await seekTo(start, 2500);

            // 兜底：数据尚未加载时，先静音起播触发加载，再 seek。
            if (!ok) {
              const wasMuted = audio.muted;
              audio.muted = true;
              try {
                await audio.play();
              } catch {
                /* 忽略 */
              }
              await waitFor('playing', 3000);
              ok = await seekTo(start, 4000);
              audio.muted = wasMuted;
            }

          }

          if(requestId!==playRequestRef.current)return;
          try {
            await audio.play();
          } catch (err) {
            setError(`无法播放：${err.message}`);setPlayStatus('error');
          }

          // play() 会重置倍速，这里重新应用。
          audio.playbackRate = effectiveRate;
          audio.volume = volume;

        },
        [rate, volume],
      );

      /** 播放队列中的第 i 集。 */
      const playAt = useCallback(
        (list, i, options = {}) => {
          setQueue(list);
          setIndex(i);
          const ep = list[i];
          if (ep) play(ep, undefined, options);
        },
        [play],
      );

      const toggle = useCallback(() => {
        const audio = audioRef.current;
        if (!audio) return;
        // 恢复播放列表后还没加载过音频：点播放时先加载当前这一集
        if (!audio.getAttribute('src') && currentRef.current) {
          play(currentRef.current);
          return;
        }
        if (audio.paused) audio.play().catch((e) => setError(e.message));
        else audio.pause();
      }, [play]);

      const step = useCallback(
        (delta) => {
          const next = index + delta;
          if (next < 0 || next >= queue.length) return;
          setIndex(next);
          play(queue[next]);
        },
        [index, queue, play],
      );

      const seek = useCallback((sec) => {
        const audio = audioRef.current;
        const episode = currentRef.current;
        if (!audio || !episode?.eid || !Number.isFinite(sec)) return;
        const loaded = audio.dataset.xyzEid === episode.eid && audio.readyState >= 1;
        const total = loaded && Number.isFinite(audio.duration) ? audio.duration : Number(episode.duration) || 0;
        const target = Math.max(0, total > 0 ? Math.min(sec, total) : sec);
        pendingSeekRef.current = {eid:episode.eid, position:target};
        setPosition(target);
        // An explicit seek may go backwards; save that choice rather than the
        // maximum historical position. Do not fetch or play unloaded audio here.
        sendProgress(episode, target, true, true);
        if (loaded) {
          try { audio.currentTime = target; } catch { /* Apply when media is ready. */ }
        }
      }, [sendProgress]);

      const changeRate=useCallback(value=>{
        if(!RATES.includes(value))return;
        setRate(value);
        const ep=currentRef.current,key=ep?.subscriptionPid||ep?.pid;
        if(key)changeLibrary(prev=>({...prev,podcastRates:{...prev.podcastRates,[key]:value}}));
        call('savePrefs',{rate:value}).catch(()=>{});
      },[changeLibrary]);
      const cycleRate = useCallback(() => {
        const idx = RATES.indexOf(rate);
        const next = RATES[(idx + 1) % RATES.length];
        changeRate(next);
      }, [rate, changeRate]);

      const changeVolume = useCallback(
        (v) => {
          setVolume(v);
          if (audioRef.current) audioRef.current.volume = v;
          call('savePrefs', { rate, volume: v }).catch(() => {});
        },
        [rate],
      );

      // 倍速与音量统一同步到 audio 元素。
      // 任何修改路径（播放条按钮、设置页、启动恢复）都走这里，
      // 避免出现"界面显示 2x、实际还在 1x"的情况。
      useEffect(() => {
        const audio = audioRef.current;
        if (!audio) return;
        if (audio.playbackRate !== rate) audio.playbackRate = rate;
      }, [rate]);

      useEffect(() => {
        const audio = audioRef.current;
        if (!audio) return;
        if (Math.abs(audio.volume - volume) > 0.001) audio.volume = volume;
      }, [volume]);

      useEffect(() => {
        const audio = audioRef.current;
        if (!audio) return undefined;
        const onTime = () => {
          const pending = pendingSeekRef.current;
          if (pending?.eid === currentRef.current?.eid) {
            if (audio.dataset.xyzEid !== pending.eid || audio.readyState < 1 || Math.abs(audio.currentTime-pending.position) > 1) return;
            pendingSeekRef.current = null;
          }
          setPosition(audio.currentTime);
          if (audio.currentTime > 5) sendProgress(currentRef.current, audio.currentTime, false);
        };
        const applyPendingSeek = () => {
          const pending = pendingSeekRef.current;
          if (pending?.eid !== currentRef.current?.eid || audio.dataset.xyzEid !== pending?.eid || audio.readyState < 1) return;
          try { audio.currentTime = pending.position; } catch { /* Wait for canplay. */ }
        };
        const onMeta = () => {
          setDuration(audio.duration || currentRef.current?.duration || 0);
          applyPendingSeek();
        };
        const onPlay = () => {
          setPlaying(true);
          const eid = currentRef.current?.eid;
          if (eid) retryRef.current[eid] = 0;
          // 起播瞬间再校正一次倍速，兜住浏览器的延迟重置。
          if (audio.playbackRate !== rateRef.current) audio.playbackRate = rateRef.current;
        };
        const onPause = () => {
          setPlayStatus('paused');
          setPlaying(false);
          if (currentRef.current && audio.dataset.xyzEid===currentRef.current.eid) sendProgress(currentRef.current, audio.currentTime, true);
        };
        const onEnded = () => {
          setPlaying(false);
          setPlayStatus('paused');
          const finished = currentRef.current;
          if (finished) {
            sendProgress(finished, audio.duration || 0, true);
            // 播完即清理该集缓存（以及不再需要的节目缓存）
            call(`dropCache?eid=${encodeURIComponent(finished.eid)}&pid=${encodeURIComponent(finished.pid || '')}`).catch(() => {});
          }

          // 播完即从播放列表移除，然后接上下一集。
          const currentIdx = queue.findIndex((e) => finished && e.eid === finished.eid);
          const dropIdx = currentIdx;
          const remaining = queue.filter((_, i) => i !== dropIdx);

          // 移除后，原位置就是"下一个"；若已越过末尾则说明这集是最后一个。
          const nextIdx = Math.max(0,dropIdx);
          if (nextIdx >= remaining.length) {
            if(sleepRef.current.mode==='end')setSleepMode('off');
            setQueue(remaining);
            setIndex(-1);
            setCurrent(null);
            setPosition(0);
            return;
          }

          setQueue(remaining);
          setIndex(nextIdx);
          if(sleepRef.current.mode==='end'){
            setSleepMode('off');audio.removeAttribute('src');delete audio.dataset.xyzEid;audio.load();setCurrent(remaining[nextIdx]);setPosition(0);return;
          }
          play(remaining[nextIdx]);
        };
        const onErr = async () => {
          if (!audio.error) return;
          const ep = currentRef.current;
          const eid = ep?.eid;

          // 付费单集的临时直链只有 4 小时。失效时重新换一个，并从原位置继续。
          if (eid) {
            const used = retryRef.current[eid] || 0;
            if (used < 2) {
              retryRef.current[eid] = used + 1;
              const resumeAt = audio.currentTime || 0;
              try {
                const r = await apiFetch(`${API}/audioUrl?eid=${encodeURIComponent(eid)}&refresh=1`);
                const d = await r.json();
                if (d?.ok && d.url && d.url !== audio.src) {
                  audio.src = d.url;
                  audio.dataset.xyzEid = eid;
                  audio.addEventListener('loadedmetadata', () => {
                    try {
                      audio.currentTime = resumeAt;
                    } catch {
                      /* 忽略 */
                    }
                  }, { once: true });
                  await audio.play().catch(() => {});
                  setError('');
                  return;
                }
              } catch {
                /* 落到下面的通用提示 */
              }
            }
          }
          setPlayStatus('error');setError('音频加载失败，可点击重试');
        };
        const onWaiting=()=>{if(!audio.paused)setPlayStatus('buffering')};
        const onPlaying=()=>{setPlayStatus('playing');setError('')};
        audio.addEventListener('waiting',onWaiting);audio.addEventListener('stalled',onWaiting);audio.addEventListener('playing',onPlaying);
        audio.addEventListener('timeupdate', onTime);
        audio.addEventListener('loadedmetadata', onMeta);
        audio.addEventListener('canplay', applyPendingSeek);
        audio.addEventListener('seeked', onTime);
        audio.addEventListener('play', onPlay);
        audio.addEventListener('pause', onPause);
        audio.addEventListener('ended', onEnded);
        audio.addEventListener('error', onErr);
        return () => {
          audio.removeEventListener('waiting',onWaiting);audio.removeEventListener('stalled',onWaiting);audio.removeEventListener('playing',onPlaying);
          audio.removeEventListener('timeupdate', onTime);
          audio.removeEventListener('loadedmetadata', onMeta);
          audio.removeEventListener('canplay', applyPendingSeek);
          audio.removeEventListener('seeked', onTime);
          audio.removeEventListener('play', onPlay);
          audio.removeEventListener('pause', onPause);
          audio.removeEventListener('ended', onEnded);
          audio.removeEventListener('error', onErr);
        };
      }, [index, queue, play, sendProgress, rate, volume]);

      // 离开页面时把进度落盘
      useEffect(() => {
        const flush = () => {
          const audio = audioRef.current;
          if (audio && currentRef.current) sendProgress(currentRef.current, audio.currentTime, true);
        };
        window.addEventListener('pagehide', flush);
        return () => window.removeEventListener('pagehide', flush);
      }, [sendProgress]);

      const flushLocalData=useCallback(async(pause=false)=>{
        if(pause){playRequestRef.current++;audioRef.current?.pause()}
        await libraryWrites.current;
        const ep=currentRef.current,a=audioRef.current;
        if(ep){const pos=pendingSeekRef.current?.eid===ep.eid?pendingSeekRef.current.position:(a?.dataset.xyzEid===ep.eid&&a.readyState>0?a.currentTime:(Number(progressRef.current?.[ep.eid]?.position)||0));await sendProgress(ep,pos,true)}
        await call('saveQueue',{queue:queue.map(queueEpisode),index});
      },[queue,index,sendProgress]);
      const reloadLocalState=useCallback(async()=>{
        await libraryWrites.current;
        const d=await call('state'), state=d.state||{};
        playRequestRef.current++;audioRef.current?.pause();if(audioRef.current){audioRef.current.removeAttribute('src');audioRef.current.load()}
        pendingSeekRef.current=null;progressRef.current=state.progress||{};
        hydrateLibrary(state.library);
        setQueue(state.queue||[]);const idx=Math.max(0,Math.min(Number(state.queueIndex)||0,(state.queue||[]).length-1));
        setIndex((state.queue||[]).length?idx:-1);setCurrent(state.queue?.[idx]||null);setPosition(0);setDuration(0);setError('');setPlayStatus('idle');
        if(Number(state.rate)>0)setRate(Number(state.rate));
        if(Number(state.volume)>=0&&Number(state.volume)<=1)setVolume(Number(state.volume));
        setSleepMode('off');
      },[hydrateLibrary,setSleepMode]);
      const retry=()=>{const ep=currentRef.current;if(!ep)return;retryRef.current[ep.eid]=0;const pos=pendingSeekRef.current?.eid===ep.eid?pendingSeekRef.current.position:position;const a=audioRef.current;a?.removeAttribute('src');a?.load();play(ep,pos)};
      return {
        library,hydrateLibrary,changeLibrary,enqueue,reloadLocalState,flushLocalData,
        playStatus,retry,sleepMode,setSleepMode,sleepRemaining,changeRate,
        audioRef, progressRef, current, setCurrent, queue, setQueue, index, setIndex,
        playing, position, duration, rate, volume, error, setError, setRate, setVolume,
        play, playAt, toggle, step, seek, cycleRate, changeVolume,
        sendProgress,
      };
    }

    function PlayerBar(props) {
      const { player, onOpen, wide, onTitleToggle, notesOpen } = props;
      const { current, playing, position, duration, error } = player;
      const total = duration || current?.duration || 0;
      const remaining = Math.max(0, total - (Number(position) || 0));
      // 收起成窄栏：一个图标按钮即可。
      if (!wide) {
        return h(
          'div',
          { className: 'xyz-rail' },
          h(Styles),
          h(
            'button',
            {
              className: 'xyz-icon-btn accent',
              title: current ? `${current.title} · ${playing ? '暂停' : '播放'}` : '小宇宙播客',
              onClick: () => (current ? player.toggle() : onOpen()),
            },
            current && playing ? IconPause(16) : IconMic(16),
          ),
        );
      }

      return h(
        'div',
        { className: 'xyz-card-wrap' },
        h(Styles),
        h(
        'div',
        { className: 'xyz-card' },

        // 封面 + 文本
        h(
          'div',
          { className: 'xyz-card-top' },
          current
            ? h('img', { className: 'xyz-cover', src: imageOf(episodeImage(current)), alt: '' })
            : h('div', { className: 'xyz-cover empty' }, IconMic(18)),
          h(
            'div',
            { className: 'xyz-card-text' },
            h('button',{type:'button',className:'xyz-card-title-link',onClick:onTitleToggle,'aria-label':'切换节目介绍','aria-expanded':!!notesOpen,title:notesOpen?'收起单集介绍':'查看单集介绍'},h(FullTitle,{as:'span',className:'xyz-card-title',text:current?current.title:'小宇宙播客'})),
            h(
              'div',
              { className: error ? 'xyz-card-sub err' : 'xyz-card-sub' },
              error ? error : current ? h(PodcastLink,{episode:current}) : '点开选择节目',
            ),
          ),
          h(PinWindow),
          current ? h('span', {className:'xyz-card-duration', 'aria-label':`剩余时间 ${total > 0 ? mmss(remaining) : '未知'}`, title:'剩余时间'}, total > 0 ? mmss(remaining) : '--:--') : null,
        ),

        error ? h('button',{className:'xyz-btn',onClick:player.retry},'重试播放') : null,
        // Hover preview is independent of playback; only clicking changes time.
        h(SeekBar, {episodeId:current?.eid,total,position,seekable:!!current && total>0,onSeek:player.seek,formatTime:mmss}),

        // 时间 + 控制
        h(
          'div',
          { className: 'xyz-card-foot' },
          h(
            'span',
            { className: 'xyz-tnum', title: `${mmss(position)} / ${mmss(total)}` },
            mmss(position),
          ),
          h(
            'div',
            { className: 'xyz-controls' },
            h('button', {
              className: 'xyz-icon-btn',
              title: '上一集',
              onClick: () => player.step(-1),
              disabled: player.index <= 0,
            }, IconPrev(14)),
            h('button', {className:'xyz-icon-btn xyz-jump',title:'快退15秒',disabled:!duration,onClick:()=>player.seek((player.audioRef.current?.currentTime||0)-15)},IconJump(15,false)),
            h('button', {
              className: 'xyz-icon-btn accent',
              title: playing ? '暂停' : '播放',
              onClick: player.toggle,
              disabled: !current,
            }, playing ? IconPause(15) : IconPlay(15)),
            h('button', {className:'xyz-icon-btn xyz-jump',title:'快进30秒',disabled:!duration,onClick:()=>player.seek((player.audioRef.current?.currentTime||0)+30)},IconJump(30,true)),
            h('button', {
              className: 'xyz-icon-btn',
              title: '下一集',
              onClick: () => player.step(1),
              disabled: player.index >= player.queue.length - 1,
            }, IconNext(14)),
          ),
          h('div',{className:'xyz-controls-aux'},
            h(SleepTimer,{player}),
            h('button', { className: 'xyz-rate', title: '倍速', onClick: player.cycleRate }, `${player.rate}×`),
            h('button', { className: 'xyz-icon-btn', title: '播放列表', onClick: onOpen }, IconList(15)),
          ),
        ),
      ),
      );
    }

    /**
     * 单集行：封面 + 标题 + 副标题 + 加入/移出播放列表。
     * 搜索结果、榜单、精选、历史都用它，避免同一套结构写六遍。
     */
    function EpisodeRow(props) {
      const { ep, player, rank, subtitle, onPlay, onAdd, addDisabled } = props;
      const showNotes=React.useContext(NotesContext);
      const playIt = () => {
        if (onPlay) onPlay();
        else {
          player.play(ep);
          player.setIndex(-1);
        }
      };
      const sub =
        subtitle !== undefined
          ? subtitle
          : `${episodePodcast(ep)}${ep.duration ? ` · ${mmss(ep.duration)}` : ''}`;
      return h(
        'div',
        { className: 'xyz-row xyz-row-hover','data-eid':ep.eid,onClick:()=>showNotes(ep),tabIndex:0,role:'group','aria-label':ep.title,onKeyDown:e=>{if(e.target===e.currentTarget&&(e.key==='Enter'||e.key===' ')){e.preventDefault();showNotes(ep)}} },
        rank !== undefined ? h('span', { className: 'xyz-rank' }, String(rank)) : null,
        h('img', { src: imageOf(episodeImage(ep)), alt: '' }),
        h(
          'div',
          { className: 'xyz-row-main', style: { cursor: 'pointer' } },
          h(FullTitle, {className:'xyz-row-t',text:ep.title}),
          h('div', { className: 'xyz-row-s' },h(PodcastLink,{episode:ep}),episodePodcast(ep)?(sub.startsWith(episodePodcast(ep))?sub.slice(episodePodcast(ep).length):sub?` · ${sub}`:''):sub),
        ),
        h('button',{className:'xyz-icon-btn xyz-episode-play',title:'播放这期节目',onClick:e=>{e.stopPropagation();playIt()}},IconPlay(14)),
        h(QueueAdd,{episode:ep,player,disabled:!!addDisabled,onAdd}),
      );
    }

    function QrLogin({onSuccess,onCancel}) {
      const [refresh,setRefresh]=useState(0),[image,setImage]=useState(''),[status,setStatus]=useState('creating'),[error,setError]=useState('');
      const successRef=useRef(onSuccess);successRef.current=onSuccess;
      useEffect(()=>{
        let alive=true,timer,sessionId='';let failures=0;
        setImage('');setStatus('creating');setError('');
        const schedule=fn=>{if(alive)timer=setTimeout(fn,2000)};
        const poll=async()=>{
          if(!alive)return;
          try{
            const data=await call('qrStatus',{sessionId});if(!alive)return;
            failures=0;setError('');setStatus(data.status);
            if(data.status==='success'){await successRef.current(data);return}
            if(data.status==='expired')return;
            schedule(poll);
          }catch(e){if(!alive)return;failures++;setError(e.message);if(failures>=3){setStatus('error');return}schedule(poll)}
        };
        (async()=>{
          try{
            const data=await call('qrStart',{});sessionId=data.sessionId||'';
            if(!alive){if(sessionId)call('qrCancel',{sessionId}).catch(()=>{});return}
            if(!sessionId||!data.url)throw Error('二维码创建失败，请重试');
            const png=await QRCode.toDataURL(data.url,{width:224,margin:3,errorCorrectionLevel:'M',color:{dark:'#111111',light:'#ffffff'}});
            if(!alive)return;setImage(png);setStatus('waiting');schedule(poll);
          }catch(e){if(alive){setError(e.message);setStatus('error')}}
        })();
        return()=>{alive=false;clearTimeout(timer);if(sessionId)call('qrCancel',{sessionId}).catch(()=>{})};
      },[refresh]);
      const expired=status==='expired'||status==='error';
      const label={creating:'正在生成二维码…',waiting:'请用手机小宇宙 App 扫码登录',scanned:'扫码成功，请在手机上确认登录',success:'登录成功',expired:'二维码已失效，请刷新后重新扫码',error:'二维码暂时不可用'}[status]||'等待扫码';
      return h('div',{className:'xyz-qr-login',role:'region','aria-label':'手机扫码登录','data-qr-status':status},
        h('div',{className:'xyz-qr-code'},image&&!expired?h('img',{src:image,alt:'小宇宙登录二维码',width:224,height:224}):h('span',null,expired?'请刷新二维码':'正在生成…')),
        h('div',{className:'xyz-qr-status',role:'status'},label),
        error?h('div',{className:'xyz-err'},error):null,
        h('div',{className:'xyz-qr-actions'},h('button',{className:'xyz-btn primary',onClick:()=>setRefresh(v=>v+1),disabled:status==='creating'},expired?'刷新二维码':'换一个二维码'),h('button',{className:'xyz-btn',onClick:onCancel},'取消')));
    }

    function Panel(props) {
      const { player, onClose, initialTab, updates, navigationRevision, podcastTarget } = props;
      const showNotes=React.useContext(NotesContext);
      const [tab, setTab] = useState(initialTab || 'queue');
      useEffect(()=>setTab(initialTab||'queue'),[initialTab,navigationRevision]);
      const [state, setState] = useState(null);
      const [loggedIn, setLoggedIn] = useState(false);
      const [loading, setLoading] = useState(false);
      const [message, setMessage] = useState('');
      const [cookie, setCookie] = useState('');
      const [qrVisible,setQrVisible]=useState(false);
      const [subs, setSubs] = useState([]);
      const [showSubscriptions, setShowSubscriptions] = useState(false);
      const [detail, setDetail] = useState(null);
      const openPodcast=React.useContext(PodcastContext);
      const returnTab=useRef('queue');
      const [podcastLoading,setPodcastLoading]=useState(false);
      useEffect(()=>{
        if(!podcastTarget)return;
        let alive=true;
        if(tab!=='detail')returnTab.current=tab;
        setTab('detail');setDetail(null);setMessage('');setPodcastLoading(true);
        (async()=>{
          let pid=podcastTarget.subscriptionPid||podcastTarget.pid;
          if(!pid&&podcastTarget.eid){const data=await call(`episode?eid=${encodeURIComponent(podcastTarget.eid)}`);pid=data.episode?.pid;}
          if(!pid)throw new Error('暂时无法找到这个节目');
          const data=await call('open',{input:`https://www.xiaoyuzhoufm.com/podcast/${pid}`});
          if(alive)setDetail({podcast:data.podcast,episodes:data.episodes||[]});
        })().catch(error=>{if(alive)setMessage(error.message)}).finally(()=>{if(alive)setPodcastLoading(false)});
        return()=>{alive=false};
      },[podcastTarget]);
      const [cacheAt, setCacheAt] = useState(0);
      const [subQuery, setSubQuery] = useState('');
      const [searchResult, setSearchResult] = useState(null); // { episodes, podcasts }
      const [searching, setSearching] = useState(false);
      const [localEps, setLocalEps] = useState([]); // 本地缓存单集搜索结果
      const [dragIdx, setDragIdx] = useState(-1);   // 正在拖动的行
      const [discover, setDiscover] = useState(null); // { top, picks }
      const [discovering, setDiscovering] = useState(false);
      const [discoverQuery, setDiscoverQuery] = useState('');
      const [discoverResult, setDiscoverResult] = useState(null);
      const [discoverDetail, setDiscoverDetail] = useState(null); // { kind:'board'|'picks', board? }
      const [panelSize, setPanelSize] = useState({ w: 440, h: 600 });
      const resizingRef = useRef(null);
      const [pickDays, setPickDays] = useState([]);   // 精选明细页累积的天
      const [pickCursor, setPickCursor] = useState('');
      const [pickLoading, setPickLoading] = useState(false);
      const [overIdx, setOverIdx] = useState(-1);   // 悬停的目标行
      const localTimer = useRef(null);
      /** 订阅搜索：纯本地过滤，不发请求。匹配节目名、主播、简介。 */
      const filteredSubs = useMemo(() => {
        const q = subQuery.trim().toLowerCase();
        if (!q) return subs;
        return subs.filter((p) => {
          const hay = `${p.title || ''} ${p.author || ''} ${p.brief || ''}`.toLowerCase();
          return hay.includes(q);
        });
      }, [subs, subQuery]);

      const refreshState = useCallback(async () => {
        try {
          const data = await call('state');
          setState(data.state);
          setLoggedIn(Boolean(data.loggedIn));
          const w = Number(data.state?.panelWidth);
          const h = Number(data.state?.panelHeight);
          if (w > 300 && h > 240) setPanelSize({ w, h });
        } catch {
          /* 忽略 */
        }
      }, []);

      useEffect(() => {
        const onKey = (e) => {
          if (e.key === 'Escape') onClose();
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
      }, [onClose]);

      /** 只读本地缓存：零网络请求，打开面板时用它立刻渲染。 */
      const loadSubsFromCache = useCallback(async () => {
        try {
          const data = await call('subsCache');
          if (data.list?.length) {
            setSubs(data.list);
            if (data.at) setCacheAt(data.at);
          }
        } catch {
          /* 缓存读失败不影响手动刷新 */
        }
      }, []);

      /** 加载精选页：最热榜 + 编辑精选（公开内容，30 分钟缓存）。 */
      const loadDiscover = useCallback(async (force = false) => {
        setDiscovering(true);
        setMessage('');
        try {
          const data = await call(`discover${force ? '?refresh=1' : ''}`);
          setDiscover({ tops: data.tops || [], picks: data.picks || [] });
        } catch (err) {
          setMessage(err.message);
          if (err.needsLogin) setLoggedIn(false);
        } finally {
          setDiscovering(false);
        }
      }, []);

      /** 加载精选明细的一页；reset 时从头开始。 */
      const loadPicks = useCallback(async (cursor = '', reset = false) => {
        if (pickLoading) return;
        setPickLoading(true);
        try {
          const data = await call(`picks${cursor ? `?cursor=${encodeURIComponent(cursor)}` : ''}`);
          const days = data.days || [];
          setPickDays((prev) => {
            if (reset) return days;
            const seen = new Set(prev.map((d) => d.date));
            return prev.concat(days.filter((d) => !seen.has(d.date)));
          });
          setPickCursor(data.cursor || '');
        } catch (err) {
          setMessage(err.message);
        } finally {
          setPickLoading(false);
        }
      }, [pickLoading]);

      /** 精选页搜索：搜单集与节目。 */
      const searchDiscover = useCallback(async (q) => {
        const kw = String(q || '').trim();
        if (!kw) return;
        setSearching(true);
        setMessage('');
        try {
          const data = await call(`search?q=${encodeURIComponent(kw)}`);
          setDiscoverResult({ episodes: data.episodes || [], podcasts: data.podcasts || [] });
          if (!(data.episodes || []).length && !(data.podcasts || []).length) {
            setMessage('没有找到相关内容');
          }
        } catch (err) {
          setMessage(err.message);
          if (err.needsLogin) setLoggedIn(false);
        } finally {
          setSearching(false);
        }
      }, []);

      /** 搜索全部单集（走接口，有 1 小时缓存）。只在用户点按钮时调用。 */
      const searchEpisodes = useCallback(async (q) => {
        const keyword = String(q || '').trim();
        if (!keyword) return;
        setSearching(true);
        setMessage('');
        try {
          const data = await call(`search?q=${encodeURIComponent(keyword)}`);
          const eps = data.episodes || [];
          setSearchResult({ episodes: eps, podcasts: data.podcasts || [] });
          if (!eps.length) setMessage('没有找到相关单集');
        } catch (err) {
          setMessage(err.message);
          if (err.needsLogin) setLoggedIn(false);
        } finally {
          setSearching(false);
        }
      }, []);

      /** 真正向小宇宙请求一次。只在用户点「刷新订阅」时调用。 */
      const loadSubs = useCallback(async () => {
        setLoading(true);
        setMessage('');
        try {
          const data = await call('subscriptions?refresh=1');
          setSubs(data.list || []);
          setCacheAt(Date.now());
          if (data.user) setState((s) => ({ ...s, user: data.user }));
        } catch (err) {
          setMessage(err.message);
          if (err.needsLogin) setLoggedIn(false);
        } finally {
          setLoading(false);
        }
      }, []);

      useEffect(() => {
        if (tab === 'discover' && !discover && !discovering) loadDiscover();
      }, [tab, discover, discovering, loadDiscover]);

      // 切到历史页时刷新一次本地记录（不发网络请求）
      useEffect(() => {
        if (tab === 'history') refreshState();
      }, [tab, refreshState]);

      useEffect(() => {
        refreshState();
        // 只读本地缓存，零网络请求：打开面板就能看到上次的订阅。
        loadSubsFromCache();
      }, [refreshState, loadSubsFromCache]);


      // 不自动拉取订阅与历史：一律由用户点按钮触发，把请求频率降到最低。


      const doLogin = async () => {
        setLoading(true);
        setMessage('');
        try {
          const data = await call('login', { cookie });
          setLoggedIn(true);
          setState((s) => ({ ...s, user: data.user }));
          setCookie('');
          setSubs([]);setCacheAt(0);setShowSubscriptions(true);
          setMessage(
            data.user?.nickname
              ? `已登录：${data.user.nickname}。点「刷新订阅」拉取列表。`
              : '已导入登录态，点「刷新订阅」拉取列表。',
          );
          setTab('subs');
        } catch (err) {
          setMessage(err.message);
        } finally {
          setLoading(false);
        }
      };

      const doLogout = async () => {
        setQrVisible(false);
        try { await call('logout', {}); } catch (error) { setMessage(`退出失败：${error.message}`); return; }
        setLoggedIn(false);
        setSubs([]);
        setMessage('已退出登录');
        refreshState();
      };

      /** 续播位置保存在本机，无记录时从头开始。 */
      const resumeFor = (episode) => {
        const localRaw = state?.progress?.[episode.eid]?.position;
        const position = Number(localRaw) || 0;
        // 太快（片头）续播意义不大；太靠后视为听完，从头再来。
        if (position < 15) return 0;
        if (episode.duration > 0 && position > episode.duration - 10) return 0;
        return position;
      };



      const tabs = [
        ['queue', '播放列表'],
        ['subs', '订阅'],
        ['discover', '精选'],
        ['history', '历史'],
        ['settings', '设置'],
      ];

      return h(
        'div',
        {
          className: 'xyz-panel',
          role: 'dialog',
          'aria-label': '小宇宙播客',
          style: { width: `${panelSize.w}px`, height: `${panelSize.h}px` },
        },
        h(Styles),
        h(
          'div',
          { className: 'xyz-panel-head' },
          h(
            'div',
            { className: 'xyz-tabs' },
            ...tabs.map(([id, label]) =>
              h('button', { key: id, className: 'xyz-tab', 'data-active': String(tab === id || (id==='subs'&&tab==='detail')), onClick: () => setTab(id) }, label),
            ),
          ),
        ),
        h('div', {
          className: 'xyz-resize',
          title: '拖动调整大小',
          onMouseDown: (e) => {
            e.preventDefault();
            const start = { x: e.clientX, y: e.clientY, w: panelSize.w, h: panelSize.h };
            let latest = { w: panelSize.w, h: panelSize.h };
            const onMove = (ev) => {
              // 面板以左下角为基准，向下拖动时高度减小
              const w = Math.max(320, Math.min(820, start.w + (ev.clientX - start.x)));
              const h = Math.max(260, Math.min(window.innerHeight - 70, start.h - (ev.clientY - start.y)));
              latest = { w, h };
              setPanelSize({ w, h });
            };
            const onUp = () => {
              window.removeEventListener('mousemove', onMove);
              window.removeEventListener('mouseup', onUp);
              call('savePrefs', { panelWidth: latest.w, panelHeight: latest.h }).catch(() => {});
            };
            window.addEventListener('mousemove', onMove);
            window.addEventListener('mouseup', onUp);
          },
        }),
        h(
          'div',
          {
            className: tab === 'subs' && !showSubscriptions ? 'xyz-panel-body xyz-feed-body' : 'xyz-panel-body',
            onScroll: (e) => {
              // 精选明细：滚动到底部附近自动加载更早的一天（无感加载）
              if (discoverDetail?.kind !== 'picks' || !pickCursor || pickLoading) return;
              const el = e.currentTarget;
              if (el.scrollTop + el.clientHeight >= el.scrollHeight - 160) {
                loadPicks(pickCursor, false);
              }
            },
          },
          message ? h('div', { className: tab === 'settings' ? 'xyz-hint' : 'xyz-err', style: { marginBottom: 8 } }, message) : null,

          tab === 'detail' && !detail
            ? h('div',null,h('button',{className:'xyz-btn',onClick:()=>setTab(returnTab.current)},'‹ 返回'),podcastLoading?h('div',{className:'xyz-hint',role:'status',style:{padding:'16px 0'}},'正在加载节目…'):null)
            : null,

          tab === 'detail' && detail
            ? h(PodcastDetails, {detail, player, EpisodeRow, onBack: () => setTab(returnTab.current)})
            : null,

          tab === 'subs' && !showSubscriptions
            ? h(EpisodeUpdates, {player, call, EpisodeRow, subscriptionCount:subs.length, onSubscriptions:()=>setShowSubscriptions(true), relTime}) : null,

          tab === 'subs' && showSubscriptions
            ? loggedIn
              ? h(
                  'div',
                  { style: { display: 'flex', flexDirection: 'column', gap: 8 } },
                  h('button', {className:'xyz-btn', style:{alignSelf:'flex-start'}, onClick:()=>setShowSubscriptions(false)}, '‹ 订阅动态'),
                  h(
                    'div',
                    { className: 'xyz-toolbar' },
                    h(
                      'span',
                      { className: 'xyz-hint', style: { flex: 1 } },
                      subs.length
                        ? `${filteredSubs.length}/${subs.length} 个订阅${cacheAt ? ` · ${relTime(cacheAt)}` : ''}`
                        : '还没有缓存，点「刷新订阅」拉取一次',
                    ),
                    h(
                      'button',
                      {
                        className: 'xyz-btn primary',
                        onClick: () => loadSubs(),
                        disabled: loading,
                      },
                      loading ? '拉取中…' : subs.length ? '刷新订阅' : '拉取订阅',
                    ),
                  ),
                  subs.length
                    ? h(
                        'div',
                        { className: 'xyz-search' },
                        h(
                          'svg',
                          { width: 13, height: 13, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
                            strokeWidth: 2, strokeLinecap: 'round', 'aria-hidden': true, className: 'xyz-search-icon' },
                          h('circle', { cx: 11, cy: 11, r: 7 }),
                          h('path', { d: 'M20 20l-3.6-3.6' }),
                        ),
                        h('input', {
                          className: 'xyz-search-input',
                          type: 'search',
                          placeholder: '搜索节目名或主播',
                          value: subQuery,
                          onChange: (e) => {
                            const v = e.target.value;
                            setSubQuery(v);
                            if (searchResult) setSearchResult(null);
                            if (localTimer.current) clearTimeout(localTimer.current);
                            const kw = v.trim();
                            if (!kw) {
                              setLocalEps([]);
                              return;
                            }
                            localTimer.current = setTimeout(() => {
                              call(`localEpisodes?q=${encodeURIComponent(kw)}`)
                                .then((d) => setLocalEps(d?.episodes || []))
                                .catch(() => setLocalEps([]));
                            }, 250);
                          },
                        }),
                        subQuery
                          ? h(
                              'button',
                              {
                                className: 'xyz-search-clear',
                                title: '清除',
                                onClick: () => {
                                  setSubQuery('');
                                  setLocalEps([]);
                                  setSearchResult(null);
                                },
                              },
                              h(
                                'svg',
                                { width: 12, height: 12, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
                                  strokeWidth: 2.4, strokeLinecap: 'round', 'aria-hidden': true },
                                h('path', { d: 'M6 6l12 12M18 6L6 18' }),
                              ),
                            )
                          : null,
                      )
                    : null,
                  h(
                    'div',
                    { className: 'xyz-list' },
                    ...filteredSubs.map((p) =>
                      h(
                        'div',
                        { key: p.pid, className: 'xyz-row xyz-row-hover' },
                        h('img', { src: imageOf(p.image), alt: '' }),
                        h(
                          'div',
                          {
                            className: 'xyz-row-main',
                            style: { cursor: 'pointer' },
                            onClick: () => openPodcast(p),
                          },
                          h('div', { className: 'xyz-row-t' }, h(PodcastLink,{episode:p},p.title)),
                          h(
                            'div',
                            { className: 'xyz-row-s' },
                            `${p.author || ''}${p.episodeCount ? ` · ${p.episodeCount} 集` : ''}`,
                          ),
                        ),
                        h(
                          'span',
                          { className: 'xyz-row-chevron', 'aria-hidden': true },
                          h(
                            'svg',
                            { width: 14, height: 14, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
                              strokeWidth: 2, strokeLinecap: 'round', 'aria-hidden': true },
                            h('path', { d: 'M9 5l7 7-7 7' }),
                          ),
                        ),
                      ),
                    ),
                    filteredSubs.length === 0 && !localEps.length
                      ? h(
                          'div',
                          { className: 'xyz-empty' },
                          loading
                            ? '拉取中…'
                            : subs.length === 0
                              ? '还没拉取，点右上角按钮'
                              : h(
                                  'div',
                                  { style: { display: 'flex', flexDirection: 'column', gap: 10, alignItems: 'center' } },
                                  h('div', null, `订阅里没有匹配「${subQuery}」的节目`),
                                  h(
                                    'button',
                                    {
                                      className: 'xyz-btn primary',
                                      disabled: searching,
                                      onClick: () => searchEpisodes(subQuery),
                                    },
                                    searching ? '搜索中…' : '在小宇宙搜索单集',
                                  ),
                                ),
                        )
                      : null,
                    // 本地缓存单集（你点开过的节目里的单集）
                    localEps.length
                      ? h(
                          'div',
                          { style: { display: 'flex', flexDirection: 'column', gap: 4, marginTop: 8 } },
                          h('div', { className: 'xyz-hint' }, `本地单集 ${localEps.length} 条`),
                          ...localEps.slice(0, 30).map((ep) => {
                            return h(EpisodeRow, { key: `local-${ep.eid}`, ep: ep, player, });
                          }),
                        )
                      : null,
                    // 单集搜索结果
                    searchResult
                      ? h(
                          'div',
                          { style: { display: 'flex', flexDirection: 'column', gap: 6, marginTop: 6 } },
                          h(
                            'div',
                            { className: 'xyz-hint' },
                            `单集结果 ${searchResult.episodes.length} 条 · 点 + 加入播放列表`,
                          ),
                          ...searchResult.episodes.slice(0, 30).map((ep) => {
                            return h(EpisodeRow, { key: `row-${ep.eid}`, ep: ep, player, });
                          }),
                        )
                      : null,
                  ),
                )
              : h(
                  'div',
                  { className: 'xyz-hint' },
                  '需要登录才能读取订阅。请到「设置」扫码登录。',
                )
            : null,

          tab === 'queue'
            ? h(
                'div',
                { style: { display: 'flex', flexDirection: 'column', gap: 8 } },
                h(
                  'div',
                  { className: 'xyz-hint' },
                  player.queue.length
                    ? `${player.queue.length} 集 · 当前第 ${player.index + 1} 集`
                    : '播放列表还是空的。到「订阅」里点开一个节目，在单集右侧点 + 单独加入。',
                ),
                player.queue.length
                  ? h(
                      'div',
                      { style: { display: 'flex', gap: 8 } },
                      h(
                        'button',
                        { className: 'xyz-btn primary', onClick: () => player.playAt(player.queue, 0, { resume: false }) },
                        '从头播放',
                      ),
                      h(
                        'button',
                        {
                          className: 'xyz-btn',
                          onClick: () => {
                            player.setQueue([]);
                            player.setIndex(-1);
                            player.setCurrent(null);
                          },
                        },
                        '清空',
                      ),
                    )
                  : null,
                h(
                  'div',
                  { className: 'xyz-list' },
                  ...player.queue.map((ep, k) =>
                    h(
                      'div',
                      {
                        key: `${ep.eid}-${k}`,
                        'data-queue-eid':ep.eid,
                        'data-current':String(ep.eid===player.current?.eid),
                        tabIndex:0,
                        onKeyDown:e=>{if(e.target===e.currentTarget&&(e.key==='Enter'||e.key===' ')){e.preventDefault();showNotes(ep)}},
                        draggable: true,
                        className: [
                          'xyz-row',
                          ep.eid===player.current?.eid ? 'xyz-row-active' : '',
                          dragIdx === k ? 'xyz-dragging' : '',
                          overIdx === k && dragIdx !== k ? 'xyz-drop-target' : '',
                        ].filter(Boolean).join(' '),
                        onClick: () => {
                          if (dragIdx >= 0) return; // 拖动中不打开介绍
                          showNotes(ep);
                        },
                        onDragStart: (e) => {
                          setDragIdx(k);
                          e.dataTransfer.effectAllowed = 'move';
                          // Firefox 需要设置数据才会触发 drag
                          try { e.dataTransfer.setData('text/plain', String(k)); } catch { /* 忽略 */ }
                        },
                        onDragOver: (e) => {
                          e.preventDefault();
                          e.dataTransfer.dropEffect = 'move';
                          if (overIdx !== k) setOverIdx(k);
                        },
                        onDragEnd: () => {
                          setDragIdx(-1);
                          setOverIdx(-1);
                        },
                        onDrop: (e) => {
                          e.preventDefault();
                          e.stopPropagation();
                          const from = dragIdx;
                          setDragIdx(-1);
                          setOverIdx(-1);
                          if (from < 0 || from === k) return;
                          const next = [...player.queue];
                          const [moved] = next.splice(from, 1);
                          next.splice(k, 0, moved);
                          const cur = player.queue[player.index];
                          player.setQueue(next);
                          // 当前播放项跟随移动
                          const idx = cur ? next.findIndex((x) => x.eid === cur.eid) : -1;
                          if (idx >= 0) player.setIndex(idx);
                        },
                      },
                      h(
                        'span',
                        { className: 'xyz-grip', title: '拖动排序', 'aria-hidden': true },
                        h('svg', { width: 12, height: 12, viewBox: '0 0 24 24', fill: 'currentColor' },
                          h('circle', { cx: 9, cy: 6, r: 1.6 }), h('circle', { cx: 15, cy: 6, r: 1.6 }),
                          h('circle', { cx: 9, cy: 12, r: 1.6 }), h('circle', { cx: 15, cy: 12, r: 1.6 }),
                          h('circle', { cx: 9, cy: 18, r: 1.6 }), h('circle', { cx: 15, cy: 18, r: 1.6 })),
                      ),
                      h('img', { src: imageOf(episodeImage(ep)), alt: '' }),
                      h(
                        'div',
                        { className: 'xyz-row-main' },
                        h(FullTitle, {className:'xyz-row-t',text:ep.title}),
                        h(
                          'div',
                          { className: 'xyz-row-s' },
                          (() => {
                            const pos = Number(state?.progress?.[ep.eid]?.position) || 0;
                            const total = Number(ep.duration) || 0;
                            const parts = [];
                            if (pos >= 5) parts.push(`上次听到 ${mmss(pos)}`);
                            else if (total) parts.push(mmss(total));
                            return h(React.Fragment,null,h(PodcastLink,{episode:ep}),parts.length?` · ${parts.join(' · ')}`:'');
                          })(),
                        ),
                      ),
                      ep.eid!==player.current?.eid ? h('button',{className:'xyz-icon-btn xyz-queue-play',title:'播放这期节目',onClick:e=>{e.stopPropagation();player.playAt(player.queue,k)}},IconPlay(14)) : null,
                      h(
                        'button',
                        {
                          className: 'xyz-row-action',
                          title: '移出播放列表',
                          onClick: (e) => {
                            e.stopPropagation();
                            const next = player.queue.filter((_, idx) => idx !== k);
                            const removed = player.queue[k];
                            player.setQueue(next);
                            if (removed) {
                              call(`dropCache?eid=${encodeURIComponent(removed.eid)}&pid=${encodeURIComponent(removed.pid || '')}`).catch(() => {});
                            }
                            if (k < player.index) player.setIndex(player.index - 1);
                            else if (k === player.index) player.setIndex(Math.min(player.index, next.length - 1));
                          },
                        },
                        h(
                          'svg',
                          { width: 13, height: 13, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
                            strokeWidth: 2.2, strokeLinecap: 'round', 'aria-hidden': true },
                          h('path', { d: 'M6 6l12 12M18 6L6 18' }),
                        ),
                      ),
                    ),
                  ),
                ),
              )
            : null,

          tab === 'history'
            ? h(
                'div',
                { style: { display: 'flex', flexDirection: 'column', gap: 8 } },
                h(
                  'div',
                  { className: 'xyz-toolbar' },
                  h(
                    'span',
                    { className: 'xyz-hint', style: { flex: 1 } },
                    (state?.history || []).length
                      ? `最近听过 ${(state?.history || []).length} 集`
                      : '还没有收听记录',
                  ),
                  h(
                    'button',
                    {
                      className: 'xyz-btn',
                      onClick: () => refreshState(),
                    },
                    '刷新',
                  ),
                ),
                h(
                  'div',
                  { className: 'xyz-list' },
                  ...(state?.history || []).slice(0, 100).map((h2, i) => {
                    return h(EpisodeRow, { key: `hs-${h2.eid}-${i}`, ep: h2, player, });
                  }),
                  (state?.history || []).length === 0
                    ? h('div', { className: 'xyz-empty' }, '播放过的节目会出现在这里，可以随时重新加入播放列表')
                    : null,
                ),
              )
            : null,

          tab === 'discover'
            ? h(
                'div',
                { style: { display: 'flex', flexDirection: 'column', gap: 10 } },

                // —— 明细页 ——
                discoverDetail
                  ? h(
                      'div',
                      { style: { display: 'flex', flexDirection: 'column', gap: 8 } },
                      h(
                        'div',
                        { className: 'xyz-toolbar' },
                        h(
                          'button',
                          {
                            className: 'xyz-btn',
                            onClick: () => {
                              setDiscoverDetail(null);
                              setPickDays([]);
                              setPickCursor('');
                            },
                          },
                          '‹ 返回',
                        ),
                        h(
                          'span',
                          { className: 'xyz-section-title', style: { flex: 1, textAlign: 'right' } },
                          discoverDetail.kind === 'board'
                            ? discoverDetail.board?.title || '榜单'
                            : `编辑精选 · ${discoverDetail.day?.date || ''}`,
                        ),
                      ),
                      discoverDetail.kind === 'board'
                        ? h(
                            'div',
                            { className: 'xyz-list' },
                            ...(discoverDetail.board?.episodes || []).map((ep, i2) =>
                                                            h(EpisodeRow, { key: `dd-${ep.eid}-${i2}`, ep: ep, player, rank: i2 + 1, }),
                            ),
                          )
                        : h(
                            'div',
                            { style: { display: 'flex', flexDirection: 'column', gap: 12 } },
                            ...pickDays.map((day) =>
                              h(
                                'div',
                                { key: `pd-${day.date}`, className: 'xyz-section' },
                                h('div', { className: 'xyz-section-title' }, day.date),
                                h(
                                  'div',
                                  { className: 'xyz-list' },
                                  ...day.episodes.map((ep, i2) =>
                                                                        h(EpisodeRow, { key: `pdd-${day.date}-${ep.eid}-${i2}`, ep: ep, player, }),
                                  ),
                                ),
                              ),
                            ),
                            pickDays.length === 0
                              ? h('div', { className: 'xyz-empty' }, pickLoading ? '加载中…' : '暂无内容')
                              : null,
                            pickCursor
                              ? h(
                                  'button',
                                  {
                                    className: 'xyz-btn',
                                    style: { alignSelf: 'center' },
                                    disabled: pickLoading,
                                    onClick: () => loadPicks(pickCursor, false),
                                  },
                                  pickLoading ? '加载中…' : '加载更早的',
                                )
                              : h('div', { className: 'xyz-hint', style: { textAlign: 'center' } }, '已经到底了'),
                          ),
                    )
                  : h(
                      'div',
                      { style: { display: 'flex', flexDirection: 'column', gap: 12 } },

                      h('button', { className: 'xyz-btn', disabled: discovering, onClick: () => loadDiscover(true) }, discovering ? '刷新中…' : '刷新精选'),
                      // 搜索框
                      h(
                        'div',
                        { className: 'xyz-search' },
                        h(
                          'svg',
                          { width: 13, height: 13, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
                            strokeWidth: 2, strokeLinecap: 'round', className: 'xyz-search-icon' },
                          h('circle', { cx: 11, cy: 11, r: 7 }),
                          h('path', { d: 'M20 20l-3.6-3.6' }),
                        ),
                        h('input', {
                          className: 'xyz-search-input',
                          type: 'search',
                          placeholder: '搜索单集或节目，回车搜索',
                          value: discoverQuery,
                          onChange: (e) => {
                            setDiscoverQuery(e.target.value);
                            if (discoverResult) setDiscoverResult(null);
                          },
                          onKeyDown: (e) => {
                            if (e.key === 'Enter') searchDiscover(discoverQuery);
                          },
                        }),
                        discoverQuery
                          ? h(
                              'button',
                              {
                                className: 'xyz-search-clear',
                                title: '清除',
                                onClick: () => {
                                  setDiscoverQuery('');
                                  setDiscoverResult(null);
                                },
                              },
                              h('svg', { width: 12, height: 12, viewBox: '0 0 24 24', fill: 'none',
                                stroke: 'currentColor', strokeWidth: 2.4, strokeLinecap: 'round' },
                                h('path', { d: 'M6 6l12 12M18 6L6 18' })),
                            )
                          : null,
                      ),

                      // 搜索结果
                      discoverResult
                        ? h(
                            'div',
                            { style: { display: 'flex', flexDirection: 'column', gap: 4 } },
                            h('div', { className: 'xyz-hint' }, `找到 ${discoverResult.episodes.length} 条单集`),
                            ...discoverResult.episodes.slice(0, 40).map((ep, i2) =>
                                                            h(EpisodeRow, { key: `ds-${ep.eid}-${i2}`, ep: ep, player, }),
                            ),
                          )
                        : null,

                      // —— 卡片区 ——
                      discover
                        ? h(
                            'div',
                            { className: 'xyz-card-grid' },
                            ...(discover.tops || []).map((board) =>
                              h(
                                'button',
                                {
                                  key: `c-${board.title}`,
                                  className: 'xyz-tile',
                                  onClick: () => setDiscoverDetail({ kind: 'board', board }),
                                },
                                h('div', { className: 'xyz-tile-title' }, board.title),
                                h('div', { className: 'xyz-tile-sub' }, `${board.episodes.length} 集`),
                                h(
                                  'div',
                                  { className: 'xyz-tile-thumbs' },
                                  ...board.episodes.slice(0, 3).map((ep) =>
                                    h('img', { key: ep.eid, src: imageOf(episodeImage(ep)), alt: '' }),
                                  ),
                                ),
                              ),
                            ),
                            h(
                              'button',
                              {
                                className: 'xyz-tile',
                                onClick: () => {
                                  setDiscoverDetail({ kind: 'picks' });
                                  setPickDays([]);
                                  setPickCursor('');
                                  loadPicks('', true);
                                },
                              },
                              h('div', { className: 'xyz-tile-title' }, '编辑精选'),
                              h(
                                'div',
                                { className: 'xyz-tile-sub' },
                                `${(discover.picks || []).length} 天 · ${(discover.picks || []).reduce(
                                  (n, d) => n + d.episodes.length,
                                  0,
                                )} 集`,
                              ),
                              h(
                                'div',
                                { className: 'xyz-tile-thumbs' },
                                ...(((discover.picks || [])[0]?.episodes) || [])
                                  .slice(0, 3)
                                  .map((ep) => h('img', { key: ep.eid, src: imageOf(episodeImage(ep)), alt: '' })),
                              ),
                            ),
                          )
                        : h('div', { className: 'xyz-empty' }, discovering ? '加载中…' : '暂无内容'),

                      // —— 今天的编辑精选明细 ——
                      discover?.picks?.length
                        ? h(
                            'div',
                            { className: 'xyz-section' },
                            h(
                              'div',
                              { className: 'xyz-toolbar' },
                              h(
                                'span',
                                { className: 'xyz-section-title', style: { flex: 1 } },
                                `今天的编辑精选 · ${discover.picks[0].date}`,
                              ),
                              h(
                                'button',
                                { className: 'xyz-btn', disabled: discovering, onClick: () => loadDiscover(true) },
                                discovering ? '加载中…' : '刷新',
                              ),
                            ),
                            h(
                              'div',
                              { className: 'xyz-list' },
                              ...discover.picks[0].episodes.map((ep, i2) =>
                                                                h(EpisodeRow, { key: `tp-${ep.eid}-${i2}`, ep: ep, player, }),
                              ),
                            ),
                          )
                        : null,
                    ),
              )
            : null,

          tab === 'settings'
            ? h(
                'div',
                { className:'xyz-settings-page' },

                // —— 账户 ——
                h('div',{className:'xyz-section'},
                  h('div',{className:'xyz-section-title'},'账户'),
                  loggedIn ? h('div',{className:'xyz-account'},
                    state?.user?.avatar?h('img',{className:'xyz-avatar',src:imageOf(state.user.avatar),alt:''}):h('div',{className:'xyz-avatar empty'},IconMic(16)),
                    h('div',{className:'xyz-account-main'},h('div',{className:'xyz-account-name'},state?.user?.nickname||'已登录'),h('div',{className:'xyz-account-sub'},'播放进度和播放列表保存在本机')),
                    h('button',{className:'xyz-btn',onClick:doLogout},'退出')):
                    h('div',{className:'xyz-hint'},'使用手机小宇宙 App 扫码登录，订阅列表需手动拉取。'),
                  qrVisible ? h(QrLogin,{
                    onCancel:()=>setQrVisible(false),
                    onSuccess:async()=>{await refreshState();setQrVisible(false);setSubs([]);setCacheAt(0);setShowSubscriptions(true);setMessage('登录成功，点「拉取订阅」同步列表');setTab('subs')}
                  }):h('button',{className:loggedIn?'xyz-btn':'xyz-btn primary',style:{alignSelf:'flex-start'},onClick:()=>{setMessage('');setQrVisible(true)}},loggedIn?'扫码切换账号':'扫码登录'),
                  !loggedIn?h('details',{className:'xyz-login-advanced'},h('summary',null,'高级设置：手动导入登录态'),
                    h('textarea',{className:'xyz-input',rows:3,placeholder:'粘贴 cookie 或 x-jike-access-token',value:cookie,onChange:e=>setCookie(e.target.value)}),
                    h('button',{className:'xyz-btn',onClick:doLogin,disabled:loading||!cookie.trim()},loading?'导入中…':'导入登录态')):null),

                // —— 播放 ——
                h(
                  'div',
                  { className: 'xyz-section' },
                  h('div', { className: 'xyz-section-title' }, '播放'),
                  h(MenubarTranscriptSettings),
                  h(
                    'div',
                    { className: 'xyz-setting-row' },
                    h(
                      'div',
                      { className: 'xyz-setting-label' },
                      h('div', { className: 'xyz-setting-name' }, '默认播放倍速'),
                      h('div', { className: 'xyz-setting-desc' }, '调整后会按节目记忆播放速度'),
                    ),
                    h(
                      'div',
                      { className: 'xyz-seg' },
                      ...[0.75, 1, 1.25, 1.5, 2].map((r) =>
                        h(
                          'button',
                          {
                            key: r,
                            className: 'xyz-seg-item',
                            'data-active': String(Math.abs(player.rate - r) < 0.001),
                            onClick: () => {
                              player.changeRate(r);
                              call('savePrefs', { rate: r, volume: player.volume }).catch(() => {});
                            },
                          },
                          `${r}×`,
                        ),
                      ),
                    ),
                  ),
                ),

                h(EpisodeUpdateSettings,{call}),

                // —— 数据 ——
                h(
                  'div',
                  { className: 'xyz-section' },
                  h('div', { className: 'xyz-section-title' }, '本地数据'),
                  h(
                    'div',
                    { className: 'xyz-setting-row' },
                    h(
                      'div',
                      { className: 'xyz-setting-label' },
                      h('div', { className: 'xyz-setting-name' }, '订阅缓存'),
                      h(
                        'div',
                        { className: 'xyz-setting-desc' },
                        subs.length ? `${subs.length} 个节目 · ${cacheAt ? relTime(cacheAt) : '未知'}` : '还没有缓存',
                      ),
                    ),
                    h(
                      'button',
                      {
                        className: 'xyz-btn',
                        onClick: async () => {
                          await call('clearCache', {}).catch(() => {});
                          setSubs([]);
                          setCacheAt(0);
                          setMessage('缓存已清除，下次点「刷新」会重新拉取');
                        },
                      },
                      '清除',
                    ),
                  ),
                  h(
                    'div',
                    { className: 'xyz-hint' },
                    '清除订阅缓存不会删除播放列表和进度。',
                  ),
                  h(BackupSettings,{player}),
                ),
                h(UpdateSettings,{updates,player}),
              )
            : null,
        ),
      );
    }

    function EpisodeNotes({episode,onClose,onShare,player}) {
      const [view,setView]=useState('notes');
      const [commentsOpened,setCommentsOpened]=useState(false);
      const [data,setData]=useState(null),[error,setError]=useState('');
      const eid=episode?.eid;
      useEffect(()=>{
        let alive=true;setData(null);setError('');
        if(eid)call(`shownotes?eid=${encodeURIComponent(eid)}`).then(d=>{if(alive)setData(d)}).catch(e=>{if(alive)setError(e.message)});
        return()=>{alive=false};
      },[eid]);
      const timeHTML=useMemo(()=>{
        const doc=new DOMParser().parseFromString(data?.html||'','text/html');
        const walker=doc.createTreeWalker(doc.body,NodeFilter.SHOW_TEXT);const nodes=[];
        while(walker.nextNode())if(!walker.currentNode.parentElement.closest('a,script,style,code,pre'))nodes.push(walker.currentNode);
        for(const node of nodes){const text=node.textContent;const re=/(?<![\d:])(?:(\d{1,2}):)?(\d{1,3}):([0-5]\d)(?![\d:])/g;let match,last=0;const fragment=doc.createDocumentFragment();
          while((match=re.exec(text))){const seconds=match[1]?Number(match[1])*3600+Number(match[2])*60+Number(match[3]):Number(match[2])*60+Number(match[3]);if(match[1]&&Number(match[2])>59)continue;
            fragment.append(text.slice(last,match.index));const a=doc.createElement('a');a.href=`#ts=${seconds}`;a.textContent=match[0];a.setAttribute('aria-label',`跳转到 ${match[0]}`);fragment.append(a);last=re.lastIndex;
          }if(last){fragment.append(text.slice(last));node.replaceWith(fragment)}
        }
        for(const a of doc.body.querySelectorAll('a')){
          const stamp=a.textContent.trim();
          if(/^\d{1,3}:[0-5]\d(?::[0-5]\d)?$/.test(stamp)){
            const seconds=stamp.split(':').reduce((v,n)=>v*60+Number(n),0);
            a.setAttribute('href',`#ts=${seconds}`);a.setAttribute('aria-label',`跳转到 ${stamp}`);a.setAttribute('title',`跳转到 ${stamp}`);
          }
        }
        return doc.body.innerHTML;
      },[data?.html]);
      const html=useMemo(()=>DOMPurify.sanitize(timeHTML,{USE_PROFILES:{html:true},FORBID_TAGS:['audio','video','iframe','form','input','button','style','link','meta'],FORBID_ATTR:['style','id','name'],SANITIZE_NAMED_PROPS:true}),[timeHTML]);
      const follow=e=>{
        const a=e.target.closest('a');if(!a)return;e.preventDefault();
        const raw=a.getAttribute('href')||'';
        const ts=raw.match(/^#(?:ts=)?(\d+)$/);
        let seconds=ts?Number(ts[1]):null;
        if(seconds===null&&/^\d{1,3}:[0-5]\d(?::[0-5]\d)?$/.test(a.textContent.trim()))seconds=a.textContent.trim().split(':').reduce((v,n)=>v*60+Number(n),0);
        if(seconds!==null){const total=episode?.eid===player.current?.eid?(player.duration||episode.duration):episode?.duration;if(total&&seconds>total){setError('时间点超出本期节目时长');return}
          if(player.current?.eid===eid)player.seek(seconds);else{player.setIndex(player.queue.findIndex(ep=>ep.eid===eid));player.play(episode,seconds)}return;}
        try{const href=new URL(raw,`https://www.xiaoyuzhoufm.com/episode/${eid}`).href;window.mygo.call('Backend.OpenLink',href).catch(err=>setError(err.message))}catch{setError('链接无法打开')}
      };
      return h('section',{className:'xyz-notes',style:view!=='notes'?{display:'flex',flexDirection:'column',overflow:'hidden'}:undefined,role:'region','aria-label':'本期节目介绍、字幕与评论'},
        h('div',{className:'xyz-notes-head'},h('button',{className:'xyz-btn',onClick:onClose,title:'返回之前的页面'},'‹ 返回'),h(EpisodeActions,{episode,player}),h(FullTitle,{as:'span',className:'xyz-notes-title',text:episode?.title||'节目介绍'}),episode?.eid?h('button',{type:'button',className:'xyz-btn xyz-notes-share',style:{marginLeft:'auto',flexShrink:0},onClick:onShare},h('svg',{width:14,height:14,viewBox:'0 0 24 24',fill:'none',stroke:'currentColor',strokeWidth:1.8,strokeLinecap:'round',strokeLinejoin:'round','aria-hidden':true,focusable:'false'},h('path',{d:'M12 15V3m-4 4 4-4 4 4M7 10H5a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-9a1 1 0 0 0-1-1h-2'})),'分享'):null),
        h('div',{className:'xyz-notes-tabs','aria-label':'内容切换'},
          h('button',{type:'button','aria-pressed':view==='notes',onClick:()=>setView('notes')},'节目介绍'),
          h('button',{type:'button','aria-pressed':view==='transcript',onClick:()=>setView('transcript')},'字幕'),
          h('button',{type:'button','aria-pressed':view==='comments',onClick:()=>{setCommentsOpened(true);setView('comments')}},'评论')),
        h('style',null,'.xyz-notes-tabs{display:flex;gap:8px;padding:8px 14px;border-bottom:1px solid var(--app-border)}.xyz-notes-tabs button{font:inherit;font-size:12px;padding:6px 12px;border:1px solid var(--app-border);border-radius:6px;background:var(--app-hover);color:var(--app-fg);cursor:pointer}.xyz-notes-tabs button[aria-pressed=true]{background:var(--app-selected);border-color:var(--app-focus)}.xyz-notes-tabs button:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px}'),
        commentsOpened?h(Comments,{key:eid,episode,player,call,active:view==='comments'}):null,
        view==='comments'?null:
        view==='transcript'?h(Transcript,{key:eid,episode,player,call}):
        !eid?h('div',{className:'xyz-empty'},'先选择一集节目'):
        error?h('div',{className:'xyz-notes-message'},error):
        !data?h('div',{className:'xyz-notes-message'},'正在加载 shownotes…'):
        !html?h('div',{className:'xyz-notes-message'},'这期节目没有提供 shownotes'):
        h('article',{className:'xyz-notes-original',onClick:follow,dangerouslySetInnerHTML:{__html:html}}));
    }

        function PlayerDock(props) {
          // sidebar.footer.action 会传入 wide：true=展开栏，false=56px 窄栏
          const wide = props?.wide !== false;
          const player = usePlayer();
          const updates = useUpdates();
          const [navigationRevision,setNavigationRevision]=useState(0);
          const [open, setOpen] = useState(true);
          const [tab, setTab] = useState('queue');
          const [notesOpen,setNotesOpen]=useState(false);
          const [notesEpisode,setNotesEpisode]=useState(null);
          const [shareSnapshot,setShareSnapshot]=useState(null);
          const [podcastTarget,setPodcastTarget]=useState(null);
          const showPodcast=episode=>{setNotesOpen(false);setOpen(true);setPodcastTarget({...episode})};
          const showEpisodeNotes=ep=>{setNotesEpisode(ep);setNotesOpen(true)};
          playerRef.current = player;

          useEffect(() => {
            playerRef.current = player;
            mountedRef.current = true;
            mountedListeners.forEach((fn) => fn(true));
            const listener = () => {
              setOpen((value) => {
                if (!value) {setTab('queue');setPodcastTarget(null);}
                return !value;
              });
            };
            listeners.add(listener);
            return () => {
              listeners.delete(listener);
              mountedRef.current = false;
              mountedListeners.forEach((fn) => fn(false));
            };
          }, []);

          useEffect(() => {
            window.menuPlayer = {
              play: () => playerRef.current.toggle(),
              volume: v => playerRef.current.changeVolume(v),
              next: () => playerRef.current.step(1),
              previous: () => playerRef.current.step(-1),
              seek: pos => playerRef.current.seek(pos),
              flush: () => {const p=playerRef.current;p.sendProgress(p.current,p.audioRef.current?.currentTime||0,true)},
              status: () => { const p=playerRef.current,a=p.audioRef.current; return {title:p.current?.title||'',artist:episodePodcast(p.current),playing:!!a&&!a.paused,position:a?.currentTime||0,duration:Number.isFinite(a?.duration)?a.duration:0,rate:a?.playbackRate||p.rate,error:p.error,queue:p.queue.length,muted:a?.muted||false}; }
            };
            const tick=setInterval(()=>{const s=window.menuPlayer.status();window.mygo.call('Backend.MediaState',s.title,s.artist,s.playing,s.position,s.duration).catch(()=>{})},2000);
            return()=>clearInterval(tick);
          }, []);
          // 恢复 → 保存 的严格顺序：
          // 恢复完成前一律不保存，否则空列表会把已有数据覆盖掉（曾因此丢过列表）。
          const hydratedRef = useRef(false);
          const restoredRef = useRef(false);

          useEffect(() => {
            if (restoredRef.current) return;
            restoredRef.current = true;
            call('state')
              .then((d) => {
                // 注入本地进度表，供 play 时查续播位置
                player.hydrateLibrary(d?.state?.library);
                player.progressRef.current = d?.state?.progress || {};
                const saved = d?.state?.queue || [];
                if (saved.length) {
                  player.setQueue(saved);
                  // 默认停在列表第一项：卡片直接显示它，点播放键即可开始。
                  const savedIndex=Math.max(0,Math.min(Number(d?.state?.queueIndex)||0,saved.length-1));
                  player.setIndex(savedIndex);
                  player.setCurrent(saved[savedIndex]);
                }
                // 恢复上次的倍速与音量
                const savedRate = Number(d?.state?.rate);
                if (savedRate > 0) {
                  player.setRate(savedRate);
                  if (player.audioRef.current) player.audioRef.current.playbackRate = savedRate;
                }
                const savedVol = Number(d?.state?.volume);
                if (savedVol >= 0 && savedVol <= 1) {
                  player.setVolume(savedVol);
                  if (player.audioRef.current) player.audioRef.current.volume = savedVol;
                }
              })
              .catch(() => {})
              .finally(() => {
                hydratedRef.current = true;
              });
          }, []);

          // 队列有内容但还没有"当前项"时（例如刚加入播放列表），
          // 自动把第一项设为当前，让卡片立刻可用。
          useEffect(() => {
            if (!player.current && player.queue.length) {
              player.setCurrent(player.queue[0]);
              player.setIndex(0);
            }
          }, [player.queue, player.current]);

          useEffect(() => {
            if (!hydratedRef.current) return undefined;
            const timer = setTimeout(() => {
              call('saveQueue', {
                queue: player.queue.map(queueEpisode),
                index: player.index,
              }).catch(() => {});
            }, 600);
            return () => clearTimeout(timer);
          }, [player.queue, player.index]);

          return h(PodcastContext.Provider,{value:showPodcast},h(
            NotesContext.Provider,
            {value:showEpisodeNotes},
            // audio 元素放在最外层：它不随卡片/窄栏切换而重建，
            // 否则 play() 拿到的会是已被卸载的旧元素，seek 全部无效。
            h('audio', { ref: player.audioRef, preload: 'auto' }),
            h(MenubarTranscript,{player,call}),
            h(PlayerBar, {
              player,
              wide,
              notesOpen,
              onTitleToggle:()=>{if(notesOpen&&notesEpisode?.eid===player.current?.eid)setNotesOpen(false);else showEpisodeNotes(player.current)},
              onOpen: () => {
                setPodcastTarget(null);
                setNotesOpen(false);
                setOpen(true);
                setTab('queue');
                setNavigationRevision(v=>v+1);
              },
            }),
            h(UpdateBanner,{updates,onOpen:()=>{setPodcastTarget(null);setNotesOpen(false);setOpen(true);setTab('settings');setNavigationRevision(v=>v+1)}}),
            h('div',{className:'xyz-content',hidden:notesOpen},open ? h(Panel, { player, updates,navigationRevision, podcastTarget, initialTab: tab, onClose: () => window.mygo.call('Backend.Hide') }) : null),
            notesOpen ? h(EpisodeNotes,{key:notesEpisode?.eid,player,episode:notesEpisode,onClose:()=>setNotesOpen(false),onShare:()=>setShareSnapshot({episode:notesEpisode,position:notesEpisode?.eid===player.current?.eid?player.position:null})}) : null,
            shareSnapshot?h(ShareDialog,{...shareSnapshot,onClose:()=>setShareSnapshot(null)}):null,
          ));
        }



// Original DSH component stylesheet above is preserved verbatim. The host supplies
// All surfaces follow macOS appearance through WKWebView prefers-color-scheme.
const hostStyle=document.createElement('style');hostStyle.textContent=`
:root{color-scheme:light dark;--app-share:#087f73;--app-share-ink:#fff;--app-share-hover:#066b61;--app-bg:#f8f8fa;--app-fg:#24252b;--app-selected:#e2e4e9;--app-hover:#eceef1;--app-border:#d7d9df;--app-track:#d4d7dd;--app-link:#9a5300;--app-focus:#777b85;--app-scroll:#b7bac2;--app-play:#292b32;--app-play-ink:#fff;--dsw-alias-bg-overlay:var(--app-bg);--dsw-alias-label-primary:var(--app-fg);--dsw-alias-brand-primary:var(--app-fg);--dsw-alias-bg-base:var(--app-bg);--dsw-alias-border-l1:var(--app-border);--dsw-alias-border-l2:var(--app-border)}
@media(prefers-color-scheme:dark){:root{--app-share:#63d8c5;--app-share-ink:#102e29;--app-share-hover:#80e4d4;--app-bg:#25262b;--app-fg:#eeeff2;--app-selected:#3b3d45;--app-hover:#303239;--app-border:#3d3f47;--app-track:#50535d;--app-link:#ffbd77;--app-focus:#a9afbd;--app-scroll:#646976;--app-play:#f1f2f5;--app-play-ink:#24262c}}

.xyz-panel-body.xyz-feed-body{display:flex;flex-direction:column;overflow:hidden}
.xyz-share-backdrop{position:fixed;inset:0;z-index:2147483500;background:#0007;display:flex;align-items:center;justify-content:center;padding:16px}
.xyz-share-dialog{width:100%;max-width:440px;max-height:100%;overflow:auto;background:var(--app-bg);color:var(--app-fg);border:1px solid var(--app-border);border-radius:14px;padding:18px;display:flex;flex-direction:column;gap:16px;box-shadow:0 12px 40px #0004;font-size:13px}
.xyz-share-head{display:flex;align-items:center;justify-content:space-between;gap:10px}.xyz-share-head button{font-size:22px}
.xyz-share-title{font-size:12px;line-height:1.6;opacity:.65;overflow-wrap:anywhere}
.xyz-share-options{display:grid;grid-template-columns:1fr 1fr;gap:10px}.xyz-share-options button{background:var(--app-hover);border:1px solid var(--app-border);padding:18px 10px}
.xyz-share-timing{display:flex;gap:8px;flex-wrap:wrap}.xyz-share-timing button{padding:8px 10px;background:var(--app-hover)}.xyz-share-timing button[aria-pressed="true"]{background:var(--app-play);color:var(--app-play-ink)}
.xyz-share-card{display:block;width:100%;height:min(43vh,320px);object-fit:contain;border-radius:8px}.xyz-share-actions{display:flex;gap:10px;justify-content:flex-end}.xyz-share-actions button{padding:8px 12px}.xyz-share-back{align-self:flex-start}.xyz-share-dialog>.xyz-btn.primary{padding:10px}.xyz-share-dialog button:focus-visible{outline:2px solid var(--app-focus);outline-offset:3px}
.xyz-episode-updates{display:flex;flex-direction:column;gap:12px;flex:1;min-height:0}
.xyz-episode-updates>*{flex-shrink:0}
.xyz-episode-updates>.xyz-list{flex:1;min-height:0;overflow-y:auto}
.xyz-settings-page{display:flex;flex-direction:column;gap:12px;padding:2px 0 12px}.xyz-settings-page>.xyz-section{padding:13px 14px;border:1px solid var(--app-border);border-radius:10px;gap:10px;background:color-mix(in srgb,var(--app-hover) 35%,var(--app-bg))}.xyz-settings-page>.xyz-section>.xyz-section-title{opacity:.75;font-size:11px;letter-spacing:0;margin-bottom:2px}.xyz-settings-page .xyz-setting-row{min-height:44px;gap:12px}.xyz-settings-page .xyz-setting-name{font-size:12px;line-height:1.5}.xyz-settings-page .xyz-setting-desc{font-size:11px;line-height:1.5;opacity:.6}.xyz-settings-page .xyz-hint{font-size:11px;line-height:1.6}.xyz-settings-page .xyz-btn{font-size:11px;line-height:18px;padding:4px 9px;white-space:nowrap;flex-shrink:0;border:1px solid var(--app-border);background:var(--app-hover);font-weight:500}.xyz-settings-page .xyz-btn:hover{background:var(--app-selected)}.xyz-settings-page .xyz-btn.primary{background:var(--app-play);color:var(--app-play-ink)}.xyz-settings-page .xyz-seg{flex-wrap:wrap;justify-content:flex-end}.xyz-settings-page .xyz-seg-item{padding:3px 5px}.xyz-backup-settings{display:flex;flex-direction:column}.xyz-backup-settings>.xyz-setting-row{border-top:1px solid var(--app-border);padding-top:10px;margin-top:10px}.xyz-backup-confirm{margin-top:10px;padding:10px;background:var(--app-hover);border-radius:7px}.xyz-settings-page .xyz-account{padding-bottom:2px}.xyz-sleep-trigger.active{background:var(--app-selected);color:var(--app-share)}.xyz-sleep-panel{position:fixed;z-index:2147483400;width:270px;max-height:calc(100vh - 24px);overflow:auto;padding:14px;background:var(--app-bg);color:var(--app-fg);border:1px solid var(--app-border);border-radius:12px;box-shadow:0 8px 28px #0004;font-size:12px;display:flex;flex-direction:column;gap:12px}.xyz-sleep-heading{display:flex;align-items:center;justify-content:space-between;gap:8px}.xyz-sleep-panel button,.xyz-sleep-custom input{font:inherit;border:1px solid var(--app-border);background:var(--app-hover);color:var(--app-fg);border-radius:6px;padding:6px;cursor:pointer}.xyz-sleep-panel button:hover{background:var(--app-selected)}.xyz-sleep-panel :focus-visible{outline:2px solid var(--app-focus);outline-offset:2px}.xyz-sleep-presets{display:flex;gap:5px}.xyz-sleep-presets button{flex:1;white-space:nowrap;font-size:11px}.xyz-sleep-custom{display:flex;align-items:center;gap:7px}.xyz-sleep-custom input{width:0;min-width:0;flex:1;cursor:text}.xyz-sleep-custom button{padding:6px 12px}.xyz-sleep-end{display:flex;align-items:center;justify-content:space-between;padding:10px 0;border-top:1px solid var(--app-border);cursor:pointer}.xyz-sleep-end input{accent-color:var(--app-share);cursor:pointer}.xyz-full-title{position:fixed;z-index:2147483450;pointer-events:none;padding:8px 10px;border:1px solid var(--app-border);border-radius:7px;background:var(--app-bg);color:var(--app-fg);box-shadow:0 4px 16px #0003;font-size:12px;font-weight:500;line-height:1.6;white-space:normal;overflow-wrap:anywhere}.xyz-queue-menu{position:fixed;z-index:2147483400;width:150px;padding:5px;background:var(--app-bg);color:var(--app-fg);border:1px solid var(--app-border);border-radius:9px;box-shadow:0 6px 24px #0003}.xyz-queue-menu>button{display:flex;align-items:center;gap:9px;width:100%;padding:9px 8px;border:0;border-radius:5px;background:transparent;color:inherit;font:inherit;font-size:12px;cursor:pointer;text-align:left}.xyz-queue-menu>button:hover,.xyz-queue-menu>button:focus-visible{background:var(--app-hover);outline:none}.xyz-notes-actions{display:flex;align-items:center;gap:3px;flex-shrink:0}.xyz-notes-actions>button{display:flex;align-items:center;justify-content:center;width:27px;height:30px;padding:0;border:0;border-radius:6px;background:transparent;color:var(--app-fg);cursor:pointer}.xyz-notes-actions>button:hover{background:var(--app-hover)}.xyz-notes-actions>button:focus-visible{outline:2px solid var(--app-focus)}.xyz-feed-heading{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:4px 0 2px}
.xyz-feed-title{display:flex;align-items:center;gap:7px;font-size:14px;font-weight:600}.xyz-feed-count{font-size:10px;font-weight:500;line-height:18px;min-width:18px;padding:0 5px;border-radius:9px;text-align:center;background:var(--app-selected);color:var(--app-fg)}.xyz-feed-toolbar{display:flex;align-items:center;justify-content:space-between;gap:10px;padding-bottom:12px;border-bottom:1px solid var(--app-border)}.xyz-feed-status{font-size:11px;min-width:0}.xyz-feed-actions{display:flex;align-items:center;gap:6px;flex-shrink:0}.xyz-feed-actions>.xyz-btn{font-size:11px;line-height:18px;padding:4px 9px;white-space:nowrap}.xyz-feed-actions>.xyz-feed-check{background:var(--app-hover);border:1px solid var(--app-border)}.xyz-feed-actions>.xyz-feed-check:hover{background:var(--app-selected)}.xyz-feed-actions>.xyz-feed-add{background:var(--app-play);color:var(--app-play-ink)}
.xyz-subscriptions-entry{font-size:11px;white-space:nowrap;opacity:.7;padding:4px 0 4px 8px}.xyz-subscriptions-entry:hover{opacity:1}
.xyz-episode-updates select{font:inherit;font-size:11px;color:var(--app-fg);background:var(--app-hover);border:1px solid var(--app-border);border-radius:6px;padding:5px;min-width:0;max-width:100%}.xyz-episode-updates input[type="checkbox"]{accent-color:var(--app-share)}.xyz-feed-item{position:relative;padding-bottom:16px;border-bottom:1px solid var(--app-border)}
.xyz-feed-ignore{position:absolute;right:6px;bottom:2px;font-size:10px;opacity:.45}
.xyz-feed-footnote{margin-top:auto;padding-top:8px;font-size:10px;line-height:1.6;text-align:right;opacity:.4}
.xyz-tabs{min-width:0;overflow-x:auto}.xyz-tab{flex-shrink:0}
.xyz-update-banner{display:flex;align-items:center;gap:8px;margin:10px 12px 2px;padding:8px 9px 8px 12px;border:1px solid var(--app-border);border-radius:11px;background:var(--app-hover);flex-shrink:0;min-height:42px}
.xyz-update-spark{color:var(--app-link);font-size:18px;line-height:1;flex-shrink:0}
.xyz-update-label{flex:1;min-width:0;font-size:12px;font-weight:500;line-height:1.5;overflow-wrap:anywhere}
.xyz-update-banner .xyz-update-action{flex-shrink:0;padding:4px 11px;background:var(--app-play);color:var(--app-play-ink);border-radius:7px;font-size:11px;font-weight:600;line-height:18px}
.xyz-update-banner .xyz-update-action:hover{background:var(--app-play);filter:brightness(.9)}
.xyz-update-banner .xyz-update-dismiss{flex-shrink:0;width:24px;height:26px;padding:0;font-size:18px;line-height:26px;opacity:.5}
.xyz-update-banner .xyz-update-dismiss:hover{opacity:1}
.xyz-update-banner button:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px}
*{box-sizing:border-box}body{margin:0;padding:0;background:var(--app-bg);color:var(--app-fg);font-family:system-ui,-apple-system,sans-serif}#root{height:100vh;display:flex;flex-direction:column;gap:0}
.xyz-card-wrap{padding:0;flex-shrink:0;order:1}.xyz-panel{position:relative!important;inset:auto!important;width:100%!important;max-width:none!important;height:auto!important;max-height:none!important;min-height:0!important;flex:1;order:2;box-shadow:none}.xyz-resize{display:none!important}
.xyz-panel{border:none;border-radius:0;background:var(--app-bg)}.xyz-panel-head{padding:10px 16px}.xyz-panel-body{padding-left:16px;padding-right:16px}.xyz-row-active,.xyz-tab[data-active="true"]{background:var(--app-selected)}.xyz-row-active .xyz-row-t{color:var(--app-fg)}.xyz-row:not(.xyz-row-active):hover{background:var(--app-hover)}
.xyz-card{background:var(--app-bg);border:none;border-radius:0;padding:14px 16px;gap:9px;border-bottom:1px solid var(--app-border)}.xyz-cover{border-radius:7px}.xyz-seek{background:transparent}.xyz-seek-track{background:var(--app-track)}.xyz-seek-track>i{background:var(--app-fg)}.xyz-icon-btn.accent{background:var(--app-play);color:var(--app-play-ink)}
.xyz-seek:focus{outline:none!important}.xyz-seek:focus-visible{outline:1px solid var(--app-focus)!important;outline-offset:3px}.xyz-panel button:focus-visible{outline:1px solid var(--app-focus);outline-offset:2px}
.xyz-card-foot{position:relative;display:flex;justify-content:space-between;min-height:38px;gap:4px}.xyz-card-foot>.xyz-tnum{justify-self:start}.xyz-controls{position:absolute;left:50%;top:50%;transform:translate(-50%,-50%);gap:14px}.xyz-controls-aux{justify-self:end;display:flex;align-items:center;gap:2px}.xyz-jump{width:32px;height:32px}.xyz-controls .xyz-icon-btn.accent{width:36px;height:36px;border-radius:50%;margin:0 3px}
.xyz-content{display:flex;flex-direction:column;flex:1;min-height:0;order:2}.xyz-content[hidden]{display:none}.xyz-notes{order:2;flex:1;min-height:0;overflow:auto;background:var(--app-bg);color:var(--app-fg);border-top:1px solid var(--app-border)}.xyz-notes-original{padding:20px 22px;font-size:14px;line-height:1.85;overflow-wrap:anywhere}.xyz-notes-original p{margin:0 0 18px}.xyz-notes-original h1,.xyz-notes-original h2,.xyz-notes-original h3{font-size:16px;line-height:1.6;margin:22px 0 12px}.xyz-notes-original img{max-width:100%;height:auto}.xyz-notes-original a{color:var(--app-link);text-decoration:underline;text-underline-offset:3px;cursor:pointer;border-radius:3px}.xyz-notes-original a:hover{background:var(--app-hover);text-decoration-thickness:2px}.xyz-notes-original a:focus-visible{outline:2px solid var(--app-focus);outline-offset:3px}.xyz-notes-original ul,.xyz-notes-original ol{padding-left:22px}.xyz-notes-original li{margin-bottom:12px}.xyz-notes-original blockquote{margin:12px 0;padding-left:12px;border-left:3px solid var(--app-border)}.xyz-notes-message{padding:24px;font-size:13px}.xyz-card-top:focus-visible{outline:1px solid var(--app-focus);outline-offset:2px}
.xyz-episode-play,.xyz-queue-play{flex:0 0 auto;width:28px;height:28px}.xyz-notes-head{position:sticky;top:0;display:flex;align-items:center;gap:10px;padding:10px 14px;background:var(--app-bg);border-bottom:1px solid var(--app-border);z-index:1}.xyz-notes-head>.xyz-btn{flex:0 0 auto;white-space:nowrap;height:30px;padding:0 10px;border:1px solid var(--app-border);background:var(--app-hover);color:var(--app-fg);font-weight:600;display:inline-flex;align-items:center;justify-content:center}.xyz-notes-head>.xyz-btn:hover{background:var(--app-selected)}.xyz-notes-head>.xyz-notes-share{background:var(--app-share);color:var(--app-share-ink);border-color:transparent;padding:0 12px;gap:5px}.xyz-notes-head>.xyz-notes-share:hover{background:var(--app-share-hover)}.xyz-notes-head>.xyz-btn:active{transform:translateY(1px)}.xyz-notes-head>.xyz-btn:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px}.xyz-notes-title{flex:1;min-width:0;font-size:12px;font-weight:600;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.xyz-row-action:hover{background:var(--app-play);color:var(--app-play-ink)}
.xyz-qr-login{display:flex;flex-direction:column;align-items:center;gap:12px;padding:14px 0}.xyz-qr-code{width:248px;height:248px;padding:12px;display:grid;place-items:center;background:white;color:#24252b;border-radius:12px}.xyz-qr-code img{display:block;width:224px;height:224px}.xyz-qr-status{text-align:center;font-size:12px;line-height:1.6}.xyz-qr-actions{display:flex;gap:14px;align-items:center}.xyz-login-advanced{font-size:11px;opacity:.7;margin-top:8px}.xyz-login-advanced summary{cursor:pointer;margin-bottom:8px}.xyz-login-advanced textarea{margin-bottom:8px}
::-webkit-scrollbar{width:5px}::-webkit-scrollbar-thumb{background:var(--app-scroll);border-radius:4px}
`;document.head.append(hostStyle);
createRoot(document.getElementById('root')).render(React.createElement(React.Fragment,null,React.createElement(PlayerDock,{wide:true}),React.createElement(CoverPreview)));

const podcastStyle = document.createElement('style');
podcastStyle.textContent = `.xyz-podcast-details{display:flex;flex-direction:column;gap:16px;min-height:0}.xyz-podcast-heading{display:flex;gap:16px;align-items:center}.xyz-podcast-heading h2{font-size:18px;line-height:1.4;margin:0 0 6px;overflow-wrap:anywhere}.xyz-podcast-cover{padding:0;border:0;background:transparent;flex-shrink:0;cursor:zoom-in;border-radius:12px}.xyz-podcast-cover img{display:block;width:88px;height:88px;object-fit:cover;border-radius:12px}.xyz-podcast-cover:focus-visible{outline:2px solid var(--app-focus);outline-offset:4px}.xyz-podcast-brief{font-size:12px;line-height:1.6;margin-top:6px;opacity:.8}.xyz-podcast-tabs{display:flex;gap:20px;border-bottom:1px solid var(--app-border)}.xyz-podcast-tabs button{padding:8px 0 10px;background:none;border:0;border-bottom:2px solid transparent;color:var(--app-fg);opacity:.6;cursor:pointer;font:inherit;font-size:13px}.xyz-podcast-tabs button[aria-selected=true]{border-bottom-color:var(--app-share);color:var(--app-share);opacity:1;font-weight:600}.xyz-podcast-description{white-space:pre-wrap;overflow-wrap:anywhere;font-size:14px;line-height:1.85;padding:0 0 20px}.xyz-podcast-description p{margin:0 0 12px}.xyz-podcast-description a{color:var(--app-link)}`;
document.head.append(podcastStyle);

const podcastLinkStyle=document.createElement('style');
podcastLinkStyle.textContent=`.xyz-podcast-link{display:inline;max-width:100%;overflow:hidden;text-overflow:ellipsis;vertical-align:bottom;white-space:nowrap;padding:0;border:0;background:none;color:inherit;font:inherit;line-height:inherit;text-align:left;cursor:pointer}.xyz-podcast-link:hover{color:var(--app-share);text-decoration:underline;text-underline-offset:3px}.xyz-podcast-link:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px;border-radius:2px}.xyz-card-title-link{display:block;width:100%;min-width:0;padding:0;border:0;background:none;color:inherit;font:inherit;text-align:left;cursor:pointer}.xyz-card-title-link .xyz-card-title{display:block}.xyz-card-title-link:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px;border-radius:2px}`;
document.head.append(podcastLinkStyle);
