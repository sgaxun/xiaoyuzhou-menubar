export function scrollingSubtitle(segments,seconds,duration){
 if(!Number.isFinite(seconds)||seconds<0)return {text:'',progress:0};
 let lo=0,hi=segments.length-1,index=-1;
 while(lo<=hi){const mid=(lo+hi)>>1;if(segments[mid].startMs<=seconds*1000){index=mid;lo=mid+1}else hi=mid-1}
 if(index<0)return {text:'',progress:0};
 const segment=segments[index],start=segment.startMs/1000;
 const end=segments[index+1]?.startMs/1000||duration;
 const fraction=end>start?(seconds-start)/(end-start):0;
 // Briefly hold the beginning and end; interpolate using audio time, so
 // pausing freezes the text and seeking immediately repositions it.
 return {text:segment.text,progress:Math.max(0,Math.min(1,(fraction-.12)/.76))};
}
// Owns only subtitle timing. Media time, never wall time, drives scrolling.
export function createSubtitleScheduler({audio,enabled,eid,hasSegments=false,onStarted,onUpdate,
 setInterval:repeat=globalThis.setInterval,clearInterval:cancel=globalThis.clearInterval,
 observeSource=observeAudioSource}){
 let timer=null,disposed=false,started=false;
 const events=['play','playing','pause','ended','seeking','seeked','timeupdate','loadedmetadata','loadstart','emptied','durationchange','ratechange'];
 const stop=()=>{if(timer!==null){cancel(timer);timer=null}};
 const update=()=>{
  if(disposed)return;
  const matches=enabled&&eid&&audio?.dataset.xyzEid===eid;
  const playing=matches&&!audio.paused&&!audio.ended;
  if(playing&&!started){started=true;onStarted(eid)}
  onUpdate(matches?audio:null);
  if(playing&&hasSegments&&audio.readyState>=1){if(timer===null)timer=repeat(update,100)}else stop();
 };
 if(enabled&&eid&&audio){
  for(const event of events)audio.addEventListener(event,update);
 }
 const disconnect=enabled&&eid&&audio?observeSource(audio,update):()=>{};
 update();
 return()=>{
  if(disposed)return;
  disposed=true;stop();disconnect();
  if(enabled&&eid&&audio)for(const event of events)audio.removeEventListener(event,update);
 };
}
function observeAudioSource(audio,update){
 const observer=new MutationObserver(update);
 observer.observe(audio,{attributes:true,attributeFilter:['src','data-xyz-eid'],childList:true});
 return()=>observer.disconnect();
}
