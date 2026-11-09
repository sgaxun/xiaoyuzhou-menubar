import QRCode from 'qrcode';
import {episodeLink, shareTime} from './share.js';
import {episodeImage, episodePodcast} from './episode.js';

const WIDTH = 1080;
const MAX_PIXELS = 16000000;
const MAX_BYTES = 8 * 1024 * 1024;
const MAX_HEIGHT = Math.floor(MAX_PIXELS / WIDTH);
const FONT = 'system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif';
const tooLong = () => new Error('这段摘录太长，图片放不下，请少选一些文字再试');

// Keep explicit blank lines, spaces and grapheme clusters; never ellipsize text.
function wrap(ctx, text, width, maxLines) {
  const result = [];
  const segmenter = typeof Intl.Segmenter === 'function'
    ? new Intl.Segmenter(undefined, {granularity:'grapheme'}) : null;
  const push = line => {
    result.push(line);
    if (result.length > maxLines) throw tooLong();
  };
  for (const paragraph of text.split(/\r\n|\r|\n/)) {
    let line = '';
    const chars = segmenter
      ? Array.from(segmenter.segment(paragraph), item => item.segment) : Array.from(paragraph);
    for (const char of chars) {
      if (ctx.measureText(line + char).width > width && line) {
        // Keep closing punctuation with the preceding grapheme in Chinese text.
        if (/^[，。！？；：、）》】」』,.!?;:%]$/.test(char)) {
          const glyphs = segmenter ? Array.from(segmenter.segment(line), item => item.segment) : Array.from(line);
          const last = glyphs.pop();
          push(glyphs.join('')); line = last || '';
        } else {push(line); line = '';}
      }
      line += char;
    }
    push(line);
  }
  return result;
}

function loadCover(url) {
  return new Promise(resolve => {
    if (typeof url !== 'string' || !/^https:\/\//i.test(url)) return resolve(null);
    const img = new Image();
    const finish = value => {
      clearTimeout(timer);
      img.onload = img.onerror = null;
      resolve(value);
    };
    const timer = setTimeout(() => {finish(null); img.src = '';}, 4000);
    img.crossOrigin = 'anonymous';
    img.referrerPolicy = 'no-referrer';
    img.onload = () => finish(img.naturalWidth && img.naturalHeight ? img : null);
    img.onerror = () => finish(null);
    img.src = url;
  });
}

function coverStyle(cover, dark) {
  const fallback = {cover:null, accent:dark ? '#d6ab7e' : '#936846'};
  if (!cover) return fallback;
  try {
    const probe = document.createElement('canvas');
    probe.width = probe.height = 12;
    const ctx = probe.getContext('2d');
    ctx.drawImage(cover, 0, 0, 12, 12);
    const pixels = ctx.getImageData(0, 0, 12, 12).data;
    let r = 0, g = 0, b = 0;
    for (let i = 0; i < pixels.length; i += 4) {
      r += pixels[i]; g += pixels[i + 1]; b += pixels[i + 2];
    }
    const base = dark ? 220 : 48;
    const channel = value => Math.round(value / 144 * 0.45 + base * 0.55);
    return {cover, accent:`rgb(${channel(r)}, ${channel(g)}, ${channel(b)})`};
  } catch {
    // A tainted image must never reach the export canvas.
    return fallback;
  }
}

/** Pure renderer: no transcript edits, persistence, clipboard or backend calls.
 * The caller supplies selected text with its original line breaks and start time.
 */
export function cleanTranscriptExcerpt(text) {
  return text.split(/\r\n|\r|\n/)
    .filter(line => !/^\s*(?:\[(?:music|音乐)\]|［(?:music|音乐)］|【(?:music|音乐)】)\s*$/i.test(line))
    .join('\n');
}

export async function createTranscriptCard(episode, segments, {theme='light', endMs=null}={}) {
  if (theme !== 'light' && theme !== 'dark') throw new Error('不支持的字幕卡主题');
  if (!Array.isArray(segments) || segments.length < 1) {
    throw new Error('请先选择要分享的文字');
  }
  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i];
    if (!segment || !Number.isFinite(segment.startMs) || segment.startMs < 0 ||
        typeof segment.text !== 'string' || (i && segment.startMs < segments[i - 1].startMs)) {
      throw new Error('字幕内容或时间无效，请重新选择');
    }
  }
  const excerptText = cleanTranscriptExcerpt(segments.map(segment => segment.text).join('\n'));
  if (!excerptText.trim()) throw new Error('所选内容只有音乐提示，请选择要分享的字幕文字');
  const startMs = segments[0].startMs;
  if (endMs !== null && (!Number.isFinite(endMs) || endMs < segments.at(-1).startMs)) {
    throw new Error('字幕结束时间无效');
  }
  const link = episodeLink(episode?.eid, startMs / 1000);
  if (document.fonts?.ready) await document.fonts.ready;
  const canvas = document.createElement('canvas');
  canvas.width = WIDTH;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('当前环境无法生成字幕分享卡');
  const contentWidth = 920;
  ctx.font = `500 28px ${FONT}`;
  const podcastLines = wrap(ctx, String(episodePodcast(episode) || '播客单集'), 704, 350);
  ctx.font = `600 38px ${FONT}`;
  const titleLines = wrap(ctx, String(episode.title || '未命名单集'), 704, 350);
  const headerHeight = Math.max(168, podcastLines.length * 40 + 14 + titleLines.length * 54);
  const bodyTop = 108 + headerHeight + 112;
  const footerHeight = 366;
  const maxBodyLines = Math.floor((MAX_HEIGHT - bodyTop - footerHeight) / 66);
  if (maxBodyLines < 1) throw tooLong();
  ctx.font = `400 42px ${FONT}`;
  const bodyLines = wrap(ctx, excerptText, contentWidth, maxBodyLines);
  const footerTop = bodyTop + bodyLines.length * 66 + 64;
  const height = footerTop + 302;
  if (WIDTH * height > MAX_PIXELS) throw tooLong();

  const dark = theme === 'dark';
  const [loadedCover, qr] = await Promise.all([
    loadCover(episodeImage(episode)),
    QRCode.toCanvas(link, {width:228, margin:4, errorCorrectionLevel:'M',
      color:{dark:'#151515', light:'#ffffff'}}),
  ]);
  const style = coverStyle(loadedCover, dark);
  canvas.height = height;
  const foreground = dark ? '#f2ede5' : '#282622';
  const secondary = dark ? '#bbb4aa' : '#756d63';
  ctx.fillStyle = dark ? '#1d1e20' : '#faf6ee';
  ctx.fillRect(0, 0, WIDTH, height);
  ctx.textBaseline = 'top';
  ctx.fillStyle = style.accent;
  ctx.fillRect(80, 56, 56, 6);

  ctx.save();
  ctx.beginPath(); ctx.roundRect(80, 108, 168, 168, 20); ctx.clip();
  if (style.cover) {
    const size = Math.min(style.cover.naturalWidth, style.cover.naturalHeight);
    ctx.drawImage(style.cover, (style.cover.naturalWidth - size) / 2,
      (style.cover.naturalHeight - size) / 2, size, size, 80, 108, 168, 168);
  } else {
    ctx.fillStyle = dark ? '#343130' : '#e9dfd1'; ctx.fillRect(80, 108, 168, 168);
    ctx.fillStyle = style.accent; ctx.font = `600 42px ${FONT}`;
    ctx.fillText('播客', 122, 168);
  }
  ctx.restore();
  ctx.fillStyle = secondary; ctx.font = `500 28px ${FONT}`;
  podcastLines.forEach((line, i) => ctx.fillText(line, 296, 108 + i * 40));
  ctx.fillStyle = foreground; ctx.font = `600 38px ${FONT}`;
  titleLines.forEach((line, i) => ctx.fillText(line, 296, 108 + podcastLines.length * 40 + 14 + i * 54));

  const range = endMs === null ? `从 ${shareTime(startMs / 1000)} 收听`
    : `${shareTime(startMs / 1000)} – ${shareTime(endMs / 1000)}`;
  ctx.fillStyle = style.accent; ctx.font = `500 28px ${FONT}`;
  ctx.fillText(range, 80, bodyTop - 64);
  ctx.fillStyle = foreground; ctx.font = `400 42px ${FONT}`;
  bodyLines.forEach((line, i) => ctx.fillText(line, 80, bodyTop + i * 66));

  ctx.fillStyle = dark ? '#41403d' : '#ded6c9';
  ctx.fillRect(80, footerTop, contentWidth, 1);
  ctx.imageSmoothingEnabled = false;
  ctx.drawImage(qr, 80, footerTop + 32);
  ctx.fillStyle = foreground; ctx.font = `600 32px ${FONT}`;
  ctx.fillText('扫码从这里收听', 348, footerTop + 85);
  ctx.fillStyle = secondary; ctx.font = `400 26px ${FONT}`;
  ctx.fillText('小宇宙·自动生成字幕', 348, footerTop + 140);
  let dataURL;
  try {dataURL = canvas.toDataURL('image/png');}
  catch {throw new Error('字幕分享卡导出失败，请重试');}
  if (!dataURL.startsWith('data:image/png;base64,')) throw new Error('字幕分享卡导出失败');
  const encoded = dataURL.slice(dataURL.indexOf(',') + 1);
  const bytes = encoded.length * 3 / 4 - (encoded.endsWith('==') ? 2 : encoded.endsWith('=') ? 1 : 0);
  if (bytes > MAX_BYTES) throw tooLong();
  return dataURL;
}
