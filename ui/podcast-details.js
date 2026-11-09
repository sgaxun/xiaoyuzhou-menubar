import React from 'react';
import DOMPurify from 'dompurify';
const h = React.createElement;

export function PodcastDetails({detail, onBack, player, EpisodeRow}) {
  const [tab, setTab] = React.useState('episodes');
  React.useEffect(() => setTab('episodes'), [detail.podcast?.pid]);
  const podcast = detail.podcast;
  const description = podcast?.description || podcast?.brief || '';
  const html = React.useMemo(() => {
    // The public description can contain paragraphs and links. Keep text line breaks too.
    return DOMPurify.sanitize(description, {ALLOWED_TAGS: ['p','br','a','strong','b','em','i','ul','ol','li','blockquote'], ALLOWED_ATTR: ['href','title']});
  }, [description]);
  return h('section', {className: 'xyz-podcast-details'},
    h('button', {className: 'xyz-btn', onClick: onBack, style: {alignSelf: 'flex-start'}}, '‹ 返回'),
    podcast && h('header', {className: 'xyz-podcast-heading'},
      h('button', {className: 'xyz-podcast-cover', onClick: event => {
        event.preventDefault(); event.stopPropagation();
        document.dispatchEvent(new CustomEvent('xyz-cover-preview', {detail: podcast.image}));
      }, 'aria-label': `放大${podcast.title}封面`}, h('img', {src: podcast.image, alt: podcast.title})),
      h('div', {style: {minWidth: 0}},
        h('h2', null, podcast.title),
        h('div', {className: 'xyz-hint'}, podcast.author || ''),
        h('div', {className: 'xyz-hint'}, `共 ${podcast.episodeCount || detail.episodes.length} 集`),
        podcast.brief && h('div', {className: 'xyz-podcast-brief'}, podcast.brief))),
    h('div', {className: 'xyz-podcast-tabs', role: 'tablist', 'aria-label': '节目内容'},
      ...[['episodes','单集'],['description','节目简介']].map(([key,label]) => h('button', {
        key, id: `podcast-tab-${key}`, role: 'tab', 'aria-selected': tab === key,
        'aria-controls': `podcast-panel-${key}`, onClick: () => setTab(key),
      }, label))),
    tab === 'description'
      ? h('div', {id: 'podcast-panel-description', role: 'tabpanel', 'aria-labelledby': 'podcast-tab-description', className: 'xyz-podcast-description',
        onClick: event => {
          const link = event.target.closest('a');
          if (link) {event.preventDefault(); if (/^https?:\/\//i.test(link.href)) window.mygo.call('Backend.OpenLink', link.href);}
        },
        dangerouslySetInnerHTML: {__html: html || '这个节目暂时没有填写简介。'}})
      : h('div', {id: 'podcast-panel-episodes', role: 'tabpanel', 'aria-labelledby': 'podcast-tab-episodes', className: 'xyz-list'},
        ...detail.episodes.map(ep => h(EpisodeRow, {key: ep.eid, ep, player})),
        !detail.episodes.length && h('div', {className: 'xyz-empty'}, '这个节目暂时没有可播放的单集')));
}
