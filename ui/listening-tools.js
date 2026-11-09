import React from 'react';
import {createPortal} from 'react-dom';
import {QueueIcon} from './queue-add.js';

const h = React.createElement;
const {useState, useRef, useEffect} = React;
const styles = {
  section: {fontSize:12, color:'var(--app-fg, var(--dsw-alias-label-primary, inherit))', padding:'8px 14px', borderBottom:'1px solid var(--app-border, var(--dsw-alias-border-l1, #8883))', minWidth:0},
  stack: {display:'flex', flexDirection:'column', gap:8, minWidth:0},
  row: {display:'flex', alignItems:'center', flexWrap:'wrap', gap:6},
  control: {font:'inherit', color:'inherit', background:'var(--app-bg, var(--dsw-alias-bg-layer-2, transparent))', border:'1px solid var(--app-border, var(--dsw-alias-border-l1, #8885))', borderRadius:5, padding:'4px 7px'},
  hint: {fontSize:11, color:'var(--app-muted, var(--dsw-alias-label-secondary, inherit))', lineHeight:1.5},
};
const time = value => {
  const seconds = Math.max(0, Math.floor(Number(value) || 0));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
};
const button = (label, onClick, disabled = false, extra = {}) => h('button', {type:'button', style:{...styles.control, cursor:disabled ? 'default' : 'pointer', opacity:disabled ? .5 : 1}, onClick, disabled, ...extra}, label);

function useAction() {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState(false);
  const lock = useRef(false);
  const run = async (task, success = '') => {
    if (lock.current) return false;
    lock.current = true;
    setBusy(true); setMessage(''); setError(false);
    try {
      const result = await task();
      if (result === false) return false;
      setMessage(success);
      return true;
    } catch (err) {
      setError(true); setMessage(err?.message || String(err));
      return false;
    } finally {lock.current = false; setBusy(false)}
  };
  return {busy, run, feedback:message ? h('div', {role:error ? 'alert' : 'status', style:{...styles.hint, ...(error ? {color:'var(--dsw-alias-state-error-primary, #e5484d)'} : {})}}, message) : null};
}

export function BackupSettings({player}) {
  const action=useAction();
  const [confirmImport,setConfirmImport]=useState(false);
  return h('div',{className:'xyz-backup-settings'},
      h('div',{className:'xyz-setting-row'},
        h('div',{className:'xyz-setting-label'},h('div',{className:'xyz-setting-name'},'导出备份'),h('div',{className:'xyz-setting-desc'},'保存播放列表、进度与偏好')),
        h('button',{type:'button',className:'xyz-btn',disabled:action.busy,onClick:() => action.run(async () => {
          await player.flushLocalData();
          const saved = await window.mygo.call('Backend.ExportLocalBackup');
          return saved === true;
        }, '备份已导出')},'导出')),
      h('div',{className:'xyz-setting-row'},
        h('div',{className:'xyz-setting-label'},h('div',{className:'xyz-setting-name'},'恢复备份'),h('div',{className:'xyz-setting-desc'},'从本地文件恢复，不影响登录信息')),
        h('button',{type:'button',className:'xyz-btn',disabled:action.busy||confirmImport,onClick:()=>setConfirmImport(true)},'导入')),
      confirmImport ? h('div', {className:'xyz-backup-confirm',style:styles.stack, role:'group', 'aria-label':'确认导入备份'},
        h('span', {style:styles.hint}, '导入会覆盖播放列表、进度和偏好，并暂停播放；登录信息不变。建议先导出备份。继续选择文件？'),
        h('div', {style:styles.row},
          button('确认导入', async () => {
            await action.run(async () => {
              await player.flushLocalData(true);
              const imported = await window.mygo.call('Backend.ImportLocalBackup');
              if (imported !== true) return false;
              await player.reloadLocalState();
              return true;
            }, '备份已导入');
            setConfirmImport(false);
          }, action.busy),
          button('取消', () => setConfirmImport(false), action.busy))) : null,
      action.feedback);
}

export function SleepTimer({player}) {
  const [open,setOpen]=useState(null);
  const anchor=useRef(null),panel=useRef(null);
  const active=player.sleepMode!=='off';
  const label=player.sleepMode==='end'?'播完本期暂停':active?`剩余 ${time(player.sleepRemaining)}`:'未开启';
  const close=()=>{setOpen(null);anchor.current?.focus({preventScroll:true})};
  useEffect(()=>{
    if(!open)return;
    panel.current?.querySelector('button')?.focus({preventScroll:true});
    const outside=e=>{if(!anchor.current?.contains(e.target)&&!panel.current?.contains(e.target))setOpen(null)};
    const hide=()=>setOpen(null);
    document.addEventListener('pointerdown',outside,true);window.addEventListener('resize',hide);
    return()=>{document.removeEventListener('pointerdown',outside,true);window.removeEventListener('resize',hide)};
  },[open]);
  const start=minutes=>{player.setSleepMode(String(minutes));close()};
  return h(React.Fragment,null,
    h('button',{ref:anchor,type:'button',className:`xyz-icon-btn xyz-sleep-trigger${active?' active':''}`,title:`睡眠定时 · ${label}`,'aria-label':`睡眠定时 · ${label}`,'aria-expanded':!!open,'aria-haspopup':'dialog',onClick:()=>{if(open){close();return}const r=anchor.current.getBoundingClientRect();setOpen({left:Math.max(8,Math.min(r.right-270,window.innerWidth-278)),top:r.bottom+8})}},
      h('svg',{width:15,height:15,viewBox:'0 0 24 24',fill:'none',stroke:'currentColor',strokeWidth:1.8,strokeLinecap:'round',strokeLinejoin:'round','aria-hidden':true},h('path',{d:'M20.8 13A9 9 0 0 1 11 3.2 9 9 0 1 0 20.8 13Z'}))),
    open?createPortal(h('section',{ref:panel,className:'xyz-sleep-panel',role:'dialog','aria-label':'睡眠定时',style:open,onKeyDown:e=>{e.stopPropagation();if(e.key==='Escape'){e.preventDefault();close()}if(e.key==='Tab'){const items=[...panel.current.querySelectorAll('button,input')].filter(el=>!el.disabled);const i=items.indexOf(document.activeElement);if(e.shiftKey&&i===0){e.preventDefault();items.at(-1)?.focus()}else if(!e.shiftKey&&i===items.length-1){e.preventDefault();items[0]?.focus()}}}},
      h('div',{className:'xyz-sleep-heading'},h('strong',null,'睡眠定时'),h('span',{className:'xyz-hint',role:'status'},label)),
      h('div',{className:'xyz-sleep-presets'},...[15,30,60,90].map(n=>h('button',{key:n,type:'button',onClick:()=>start(n)},`${n} 分钟`))),
      h('label',{className:'xyz-sleep-end'},h('span',null,'播完当前这期暂停'),h('input',{type:'checkbox',role:'switch',checked:player.sleepMode==='end',disabled:!player.current,onChange:e=>{player.setSleepMode(e.target.checked?'end':'off')}})),
      active?h('button',{type:'button',className:'xyz-sleep-cancel',onClick:()=>{player.setSleepMode('off');close()}},'关闭定时'):null,
      ),document.body):null);
}

export function EpisodeActions({episode, player}) {
  const action=useAction();
  if(!episode?.eid||player.current?.eid===episode.eid||player.queue.some(ep=>ep.eid===episode.eid))return null;
  return h('div',{className:'xyz-notes-actions','aria-label':'加入播放列表'},
    h('button',{type:'button',title:'下一集播放','aria-label':'下一集播放',disabled:action.busy,onClick:()=>action.run(()=>player.enqueue(episode,'next'))},h(QueueIcon,{next:true})),
    h('button',{type:'button',title:'加入队尾','aria-label':'加入队尾',disabled:action.busy,onClick:()=>action.run(()=>player.enqueue(episode,'end'))},h(QueueIcon,{})));
}
