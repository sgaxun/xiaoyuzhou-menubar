import React from 'react';

export function openCover(event) {
  event.preventDefault();
  event.stopPropagation();
  document.dispatchEvent(new CustomEvent('xyz-cover-preview', {detail: event.currentTarget.src}));
}

export function CoverPreview() {
  const [src, setSrc] = React.useState('');
  const closeRef = React.useRef(null);
  React.useEffect(() => {
    const open = event => setSrc(event.detail);
    document.addEventListener('xyz-cover-preview', open);
    return () => document.removeEventListener('xyz-cover-preview', open);
  }, []);
  React.useEffect(() => {
    if (!src) return;
    const previous = document.activeElement;
    closeRef.current?.focus();
    const keydown = event => {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopImmediatePropagation();
        setSrc('');
      } else if (event.key === 'Tab') {
        event.preventDefault();
        closeRef.current?.focus();
      }
    };
    document.addEventListener('keydown', keydown, true);
    return () => {
      document.removeEventListener('keydown', keydown, true);
      previous?.focus();
    };
  }, [src]);
  if (!src) return null;
  return React.createElement('div', {
    role: 'dialog', 'aria-modal': true, 'aria-label': '封面大图',
    onClick: () => setSrc(''),
    style: {position: 'fixed', inset: 0, zIndex: 2147483600, background: 'rgba(0,0,0,.86)', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 32, boxSizing: 'border-box'},
  }, React.createElement('img', {
    src, alt: '封面大图', onClick: event => event.stopPropagation(),
    style: {width: 'min(100%, 560px)', height: 'auto', maxWidth: '100%', maxHeight: 'calc(100vh - 64px)', objectFit: 'contain', borderRadius: 8},
  }), React.createElement('button', {
    ref: closeRef, type: 'button', 'aria-label': '关闭封面大图', onClick: () => setSrc(''),
    style: {position: 'absolute', top: 12, right: 12, border: 0, borderRadius: 20, width: 36, height: 36, background: '#333', color: '#fff', fontSize: 24, cursor: 'pointer'},
  }, '×'));
}
