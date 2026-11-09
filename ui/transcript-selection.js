// Read only subtitle text intersections, excluding timestamps and surrounding UI.
// Range boundaries are normalized by the browser even for backwards selections.
export function readTranscriptSelection(container, selection = window.getSelection()) {
  if (!container || !selection || selection.isCollapsed || !selection.rangeCount ||
      !container.contains(selection.anchorNode) || !container.contains(selection.focusNode)) return null;
  const range = selection.getRangeAt(0);
  const parts = [];
  for (const node of container.querySelectorAll('[data-transcript-text]')) {
    if (!range.intersectsNode(node)) continue;
    const full = document.createRange();
    full.selectNodeContents(node);
    const clipped = full.cloneRange();
    if (range.compareBoundaryPoints(Range.START_TO_START, full) > 0) clipped.setStart(range.startContainer, range.startOffset);
    if (range.compareBoundaryPoints(Range.END_TO_END, full) < 0) clipped.setEnd(range.endContainer, range.endOffset);
    const text = clipped.toString();
    if (text.trim()) parts.push({text, startMs: Number(node.dataset.startMs)});
  }
  if (!parts.length || !Number.isFinite(parts[0].startMs)) return null;
  const viewport = container.getBoundingClientRect();
  const rects = [...range.getClientRects()].filter(r => r.width > 0 && r.height > 0 && r.bottom > viewport.top && r.top < viewport.bottom);
  const rect = rects.at(-1);
  if (!rect) return null;
  const left = Math.max(12, Math.min(rect.right - 90, window.innerWidth - 112));
  const below = rect.bottom + 8;
  const top = below + 34 <= Math.min(viewport.bottom, window.innerHeight - 8) ? below : Math.max(8, Math.min(rect.top - 40, window.innerHeight - 44));
  return {text: parts.map(part => part.text).join('\n'), startMs: parts[0].startMs, left, top, range: range.cloneRange()};
}
