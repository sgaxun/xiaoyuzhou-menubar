import React from 'react';
import {episodePodcast} from './episode.js';
export const PodcastContext = React.createContext(()=>{});

// Program names always navigate inside the player, independently of row actions.
export function PodcastLink({episode,children}) {
  const open = React.useContext(PodcastContext);
  const name = children || episodePodcast(episode) || episode?.title;
  if (!name) return null;
  return React.createElement('button', {
    type:'button',className:'xyz-podcast-link',title:'进入节目主页',
    'aria-label':`进入节目主页：${name}`,
    onClick:event=>{event.preventDefault();event.stopPropagation();open(episode)},
    onKeyDown:event=>{if(event.key==='Enter'||event.key===' ')event.stopPropagation()},
    onDragStart:event=>{event.preventDefault();event.stopPropagation()},
  },name);
}
