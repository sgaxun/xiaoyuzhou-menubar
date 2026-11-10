import React from 'react';
const {createElement:h,useEffect,useLayoutEffect,useRef,useState}=React;
const styles=`
.xyz-comments{min-height:0;flex:1;display:flex;flex-direction:column;overflow:hidden;padding:12px 20px 0;overscroll-behavior:contain;user-select:text;-webkit-user-select:text}
.xyz-comments-page{min-height:0;flex:1;overflow:auto;overscroll-behavior:contain;padding-bottom:24px}
.xyz-comments-sort{flex-shrink:0;display:flex;gap:18px;padding:0 0 12px;border-bottom:1px solid var(--app-border)}
.xyz-comments-sort button{background:none;border:0;padding:4px 0;color:var(--app-fg);opacity:.5;font:inherit;font-size:14px;cursor:pointer}
.xyz-comments-sort button[aria-pressed=true]{color:var(--app-share);opacity:1}
.xyz-comments-summary{font-size:12px;opacity:.6;margin:0 0 8px}
.xyz-comment{padding:16px 0;border-bottom:1px solid var(--app-border)}
.xyz-comment-head{display:flex;align-items:center;gap:9px;font-size:12px;color:var(--app-fg)}
.xyz-comment-avatar{width:28px;height:28px;border-radius:50%;object-fit:cover}
.xyz-comment-meta{font-size:11px;opacity:.5;margin-top:3px}
.xyz-comment-likes{margin-left:auto;opacity:.55;white-space:nowrap}
.xyz-comment-text{font-size:14px;line-height:1.8;white-space:pre-wrap;overflow-wrap:anywhere;margin:10px 0}
.xyz-comment-time{border:0;background:none;padding:0;color:var(--app-share);font:inherit;text-decoration:underline;cursor:pointer}
.xyz-comment-replies{background:var(--app-hover);border-radius:8px;padding:0 12px;margin-top:8px}
.xyz-comment-replies .xyz-comment:last-child{border:0}
.xyz-comments button:focus-visible{outline:2px solid var(--app-focus);outline-offset:2px}
.xyz-comments-error{font-size:12px;color:var(--app-fg);padding:8px 0}
`;
const merge=(a,b)=>[...new Map([...a,...b].map(c=>[c.id,c])).values()];
function Page({eid,thread='',order='HOT',active=true,call,render}) {
 const [rows,setRows]=useState([]),[cursor,setCursor]=useState(null),[busy,setBusy]=useState(false),[error,setError]=useState(''),[loaded,setLoaded]=useState(false);
 const alive=useRef(false),pending=useRef(false),container=useRef(null),scroll=useRef(0);
 useLayoutEffect(()=>{if(active&&container.current)container.current.scrollTop=scroll.current},[active]);
 const load=async(key=null)=>{
  if(pending.current)return;pending.current=true;setBusy(true);setError('');
  try{const result=await call('comments',{eid,thread,order,loadMoreKey:key});
   if(!result?.ok||!Array.isArray(result.data))throw Error(result?.error||'评论加载失败');
   if(alive.current){setRows(prev=>key?merge(prev,result.data):result.data);setCursor(result.loadMoreKey||null);setLoaded(true)}
  }catch(e){if(alive.current)setError(e.message||'评论加载失败')}
  finally{pending.current=false;if(alive.current)setBusy(false)}
 };
 useEffect(()=>{alive.current=true;load();return()=>{alive.current=false}},[]);
 return h('div',{ref:container,className:thread?undefined:'xyz-comments-page',style:active?undefined:{display:'none'},onScroll:()=>{if(active&&container.current)scroll.current=container.current.scrollTop}},
  !thread&&loaded?h('p',{className:'xyz-comments-summary'},`已加载 ${rows.length} 条主评论${cursor?' · '+({HOT:'按热门排序',TIME:'最新优先',TIMESTAMP:'按播放时点排序'}[order]):' · 已全部加载'}`):null,
  rows.map(render),
  error?h('div',{className:'xyz-comments-error',role:'alert'},error):null,
  busy?h('p',{className:'xyz-comments-summary',role:'status'},'正在加载评论…'):null,
  !busy&&(error||cursor)?h('button',{type:'button',className:'xyz-btn',onClick:()=>load(cursor)},error?'重试':'加载更多'):null,
  loaded&&!rows.length?h('p',{className:'xyz-comments-summary'},thread?'还没有回复':'这期节目还没有评论'):null);
}
function Comment({comment:c,episode,player,call,reply=false}) {
 const [expanded,setExpanded]=useState(false),[opened,setOpened]=useState(false),[seekError,setSeekError]=useState('');
 const text=String(c.text||'');
 const parts=text.split(/((?<![\d:])(?:\d{1,2}:)?\d{1,3}:[0-5]\d(?![\d:]))/g);
 const seek=stamp=>{
  const seconds=stamp.split(':').reduce((v,n)=>v*60+Number(n),0);
  const total=episode.eid===player.current?.eid?(player.duration||episode.duration):episode.duration;
  if(total&&seconds>total){setSeekError('时间点超出本期节目时长');return}
  setSeekError('');
  if(player.current?.eid===episode.eid)player.seek(seconds);
  else{player.setIndex(player.queue.findIndex(e=>e.eid===episode.eid));player.play(episode,seconds)}
 };
 const date=new Date(c.createdAt);const avatar=c.author?.avatar?.picture?.thumbnailUrl||c.author?.avatar?.picture?.picUrl;
 const count=Number(c.threadReplyCount||c.replyCount)||0;
 return h('article',{className:'xyz-comment'},
  h('div',{className:'xyz-comment-head'},avatar?h('img',{className:'xyz-comment-avatar',src:avatar,alt:'',loading:'lazy',referrerPolicy:'no-referrer'}):null,
   h('div',null,c.author?.nickname||'已注销用户',h('div',{className:'xyz-comment-meta'},[Number.isNaN(date.getTime())?'':date.toLocaleDateString('zh-CN'),c.ipLoc].filter(Boolean).join(' · '))),
   h('span',{className:'xyz-comment-likes',title:'点赞数'},`${c.likeCount||0} 赞`)),
  h('p',{className:'xyz-comment-text'},reply&&c.replyToComment?.author?.nickname?`回复 ${c.replyToComment.author.nickname}：`:null,
   ...parts.map((part,i)=>i%2?h('button',{key:i,type:'button',className:'xyz-comment-time',onClick:()=>seek(part),'aria-label':`跳转到 ${part}`},part):part)),
  seekError?h('div',{role:'alert',className:'xyz-comments-error'},seekError):null,
  !reply&&count>0?h('button',{type:'button',className:'xyz-btn','aria-expanded':expanded,onClick:()=>{setOpened(true);setExpanded(v=>!v)}},expanded?'收起回复':`查看 ${count} 条回复`):null,
  opened?h('div',{className:'xyz-comment-replies',hidden:!expanded},h(Page,{eid:episode.eid,thread:c.id,active:expanded,call,render:r=>h(Comment,{key:r.id,comment:r,episode,player,call,reply:true})})):null);
}
// Lazy-mounted on first entry; visited sorts and replies remain in memory only
// for this episode view. Hidden pages never initiate additional requests.
export function Comments({episode,player,call,active=true}) {
 const [order,setOrder]=useState('HOT'),[visited,setVisited]=useState(['HOT']);
 const changeOrder=value=>{setVisited(values=>values.includes(value)?values:[...values,value]);setOrder(value)};
 return h('section',{className:'xyz-comments',style:active?undefined:{display:'none'},'aria-label':'单集评论'},h('style',null,styles),
  h('div',{className:'xyz-comments-sort','aria-label':'评论排序'},...[['HOT','全部'],['TIME','最新'],['TIMESTAMP','时点']].map(([value,label])=>h('button',{key:value,type:'button','aria-pressed':order===value,onClick:()=>changeOrder(value)},label))),
  episode?.eid?visited.map(value=>h(Page,{key:episode.eid+value,eid:episode.eid,order:value,active:active&&order===value,call,render:c=>h(Comment,{key:c.id,comment:c,episode,player,call})})):h('p',null,'先选择一集节目'));
}
