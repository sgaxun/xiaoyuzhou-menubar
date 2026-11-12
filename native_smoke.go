package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo"
	"image/png"
	"os"
	"time"
)

// Opt-in diagnostics for native CI. Never uses an account or contacts a service.
func (b *Backend) nativeSmoke() {
	path := os.Getenv("XIAOYUZHOU_NATIVE_SMOKE_RESULT")
	if path == "" {
		return
	}
	result := map[string]any{"ok": false}
	defer func() {
		raw, _ := json.Marshal(result)
		if os.WriteFile(path+".tmp", raw, 0600) == nil {
			_ = os.Rename(path+".tmp", path)
		}
	}()
	fail := func(err error) { result["error"] = err.Error() }
	deadline := time.Now().Add(25 * time.Second)
	for {
		ready, err := b.win.Page().Eval("Boolean(window.menuPlayer && document.querySelector('#root')?.children.length)")
		if err != nil {
			fail(err)
			return
		}
		if ready == true {
			break
		}
		if time.Now().After(deadline) {
			fail(fmt.Errorf("player UI did not initialize"))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	// Exercise the application's actual image-copy handler and native clipboard.
	raw := trayIcon()
	if err := b.CopyShareCard("data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)); err != nil {
		fail(err)
		return
	}
	copied := mygo.Clipboard.ReadImage()
	img, err := png.Decode(bytes.NewReader(copied))
	if err != nil || img.Bounds().Dx() != 36 {
		fail(fmt.Errorf("image clipboard roundtrip failed: %v", err))
		return
	}
	// WebView audio must actually advance its playback clock, not merely expose Audio.
	value, err := b.win.Page().Eval(`(async()=>{
 const data=new ArrayBuffer(44+16000),v=new DataView(data);
 const text=(p,s)=>{for(let i=0;i<s.length;i++)v.setUint8(p+i,s.charCodeAt(i))};
 text(0,'RIFF');v.setUint32(4,36+16000,true);text(8,'WAVE');text(12,'fmt ');v.setUint32(16,16,true);v.setUint16(20,1,true);v.setUint16(22,1,true);v.setUint32(24,8000,true);v.setUint32(28,8000,true);v.setUint16(32,1,true);v.setUint16(34,8,true);text(36,'data');v.setUint32(40,16000,true);new Uint8Array(data,44).fill(128);
 const url=URL.createObjectURL(new Blob([data],{type:'audio/wav'}));const a=new Audio(url);
 try {await Promise.race([a.play(),new Promise((_,reject)=>setTimeout(()=>reject(Error('audio play timed out')),8000))]);await new Promise(r=>setTimeout(r,350));return a.currentTime>0&&!a.paused} finally {a.pause();URL.revokeObjectURL(url)}
 })()`)
	if err != nil || value != true {
		fail(fmt.Errorf("native audio playback failed (%v): %v", value, err))
		return
	}
	if err := b.platformSubtitle("Windows 字幕运行检查", 0.5); err != nil {
		fail(err)
		return
	}
	if err := b.verifyPlatformSubtitle(); err != nil {
		fail(err)
		return
	}
	if err := b.platformSubtitle("", 0); err != nil {
		fail(err)
		return
	}
	result["ok"] = true
	result["checks"] = []string{"player-ui", "image-clipboard", "webview-audio", "subtitle-window"}
}
