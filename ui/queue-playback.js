export function episodeFirst(queue, episode) {
  if (queue[0] === episode) return queue;
  return [episode, ...queue.filter(item => item.eid !== episode.eid)];
}

// Remove by identity, because dragging can move the current episode away from
// its saved index. Stop the old media before starting the successor.
export function removeQueueEpisode({queue, currentEid, stop, playAt, setQueue, setIndex}, eid) {
  const removedIndex = queue.findIndex(item => item.eid === eid);
  if (removedIndex < 0) return;
  const remaining = queue.filter(item => item.eid !== eid);
  if (eid !== currentEid) {
    setQueue(remaining);
    setIndex(remaining.findIndex(item => item.eid === currentEid));
    return;
  }
  stop();
  if (remaining.length) {
    playAt(remaining, removedIndex < remaining.length ? removedIndex : 0);
  } else {
    setQueue([]);
    setIndex(-1);
  }
}
