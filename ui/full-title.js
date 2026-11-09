import React from 'react';
import {createPortal} from 'react-dom';
const h=React.createElement;
const {useState,useRef,useEffect}=React;
export function FullTitle({as='div',text='',className}) {
  const anchor=useRef(null),[position,setPosition]=useState(null);
  const show=()=>{
    const el=anchor.current;if(!el||el.scrollWidth<=el.clientWidth){setPosition(null);return}
    const r=el.getBoundingClientRect(),width=Math.min(420,window.innerWidth-24);
    setPosition({left:Math.max(12,Math.min(r.left,window.innerWidth-width-12)),width,...(r.bottom+100<window.innerHeight?{top:r.bottom+7}:{bottom:window.innerHeight-r.top+7})});
  };
  useEffect(()=>{
    if(!position)return;
    const hide=()=>setPosition(null);
    window.addEventListener('scroll',hide,true);window.addEventListener('resize',hide);
    return()=>{window.removeEventListener('scroll',hide,true);window.removeEventListener('resize',hide)};
  },[position]);
  useEffect(()=>setPosition(null),[text]);
  return h(React.Fragment,null,
    h(as,{ref:anchor,className,onMouseEnter:show,onMouseLeave:()=>setPosition(null),onFocus:show,onBlur:()=>setPosition(null)},text),
    position?createPortal(h('div',{className:'xyz-full-title',role:'tooltip',style:position},text),document.body):null);
}
