/** Preserve the subscribed source without changing the canonical episode ID. */
export function queueEpisode(ep) {
  const {eid,pid,title,podcastTitle,image,duration,subscriptionPid,subscriptionTitle,subscriptionImage,isCoCreated} = ep;
  return {eid,pid,title,podcastTitle,image,duration,subscriptionPid,subscriptionTitle,subscriptionImage,isCoCreated};
}
export const episodeImage = ep => ep?.subscriptionImage || ep?.image || '';
export const episodePodcast = ep => {
  if (!ep) return '';
  return `${ep.subscriptionTitle || ep.podcastTitle || ''}${ep.subscriptionTitle && ep.isCoCreated ? ' · 共创' : ''}`;
};
