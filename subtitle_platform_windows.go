//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
)

// One subtitle strip belongs to the application's single Backend. Its lock also
// serializes the initial asynchronous page load with incoming audio-clock ticks.
var desktopSubtitle struct {
	sync.Mutex
	win      *mygo.Window
	renderer subtitleRenderer
	ready    bool
	text     string
	offset   int
	frame    string
	sent     string
}

// platformSubtitle never changes the tray icon. The same bounded raster and
// progress calculations used by macOS feed an independent Windows window.
func (b *Backend) platformSubtitle(text string, progress float64) error {
	s := &desktopSubtitle
	s.Lock()
	defer s.Unlock()
	text = normalizeSubtitle(text)
	if text == "" {
		s.text, s.frame, s.sent = "", "", ""
		if s.win != nil && !s.win.IsDestroyed() {
			s.win.Hide()
		}
		return nil
	}
	offset, err := s.renderer.prepare(text, progress)
	if err != nil {
		return err
	}
	if text != s.text || offset != s.offset || s.frame == "" {
		raw, err := s.renderer.render(offset)
		if err != nil {
			return err
		}
		s.text, s.offset = text, offset
		s.frame = "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	}
	if s.win == nil || s.win.IsDestroyed() {
		s.ready, s.sent = false, ""
		s.win = mygo.NewWindow(mygo.WindowOptions{
			Title: "小宇宙字幕", Width: subtitleWidth, Height: subtitleHeight,
			Hidden: true, Frameless: true, AlwaysOnTop: true, SkipTaskbar: true,
			DisableResize: true, DisableMove: true, DisableMinimize: true,
			DisableMaximize: true, DisableFullScreen: true, DisableClose: true,
			DisableShadow: true, BackgroundColor: "#202020",
		})
		if err := subtitleNoActivate(s.win.NativeHandle()); err != nil {
			s.win.Destroy()
			s.win = nil
			return err
		}
		s.win.SetIgnoreMouseEvents(true)
		win := s.win
		win.Page().OnDidFinishLoad(func() {
			// Page.Eval waits for the UI thread, so never call it inside its event.
			go func() {
				s.Lock()
				defer s.Unlock()
				if s.win != win || win.IsDestroyed() {
					return
				}
				s.ready = true
				// Use the latest frame, including updates/clears during LoadHTML. Do
				// not mark it delivered before the document actually has its image.
				_ = flushDesktopSubtitle()
			}()
		})
		win.Page().LoadHTML(`<!doctype html><meta charset="utf-8"><style>html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#202020;pointer-events:none}img{display:block;width:100%;height:100%;object-fit:contain}</style><img id="subtitle" alt="">`, "")
	}
	return flushDesktopSubtitle()
}

// Caller holds desktopSubtitle's lock. Failed evaluation remains retryable on
// the next clock tick, and no stale frame can show after a clear or a seek.
func flushDesktopSubtitle() error {
	s := &desktopSubtitle
	if !s.ready || s.frame == "" {
		return nil
	}
	if s.sent != s.frame {
		if _, err := s.win.Page().Eval("document.getElementById('subtitle').src=" + strconv.Quote(s.frame)); err != nil {
			return err
		}
		s.sent = s.frame
	}
	area := mygo.Screen.PrimaryDisplay().WorkArea
	bounds := subtitleStripBounds(area)
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return fmt.Errorf("字幕显示区域不可用")
	}
	s.win.SetSize(bounds.Width, bounds.Height)
	s.win.SetPosition(bounds.X, bounds.Y)
	s.win.ShowInactive()
	return nil
}

// WS_EX_NOACTIVATE prevents clicks or window activation from taking the
// keyboard; TOOLWINDOW keeps the noninteractive strip out of Alt+Tab too.
func subtitleNoActivate(hwnd uintptr) error {
	dll := windows.NewLazySystemDLL("user32.dll")
	getName, setName := "GetWindowLongPtrW", "SetWindowLongPtrW"
	if strconv.IntSize == 32 {
		getName, setName = "GetWindowLongW", "SetWindowLongW"
	}
	index := ^uintptr(19) // GWL_EXSTYLE = -20
	style, _, err := dll.NewProc(getName).Call(hwnd, index)
	if style == 0 {
		return fmt.Errorf("读取字幕窗口样式: %w", err)
	}
	result, _, err := dll.NewProc(setName).Call(hwnd, index, style|0x08000000|0x00000080)
	if result == 0 {
		return fmt.Errorf("设置字幕窗口不激活样式: %w", err)
	}
	return nil
}

// verifyPlatformSubtitle is a native-smoke hook. Call off the UI thread after
// submitting a nonempty subtitle and before clearing it. TryLock lets the
// deadline expire even if an asynchronous page evaluation is still pending.
func (b *Backend) verifyPlatformSubtitle() error {
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		s := &desktopSubtitle
		if s.TryLock() {
			ready := s.ready && s.frame != "" && s.sent == s.frame && s.win != nil && !s.win.IsDestroyed()
			if ready && s.win.IsVisible() {
				hwnd := s.win.NativeHandle()
				s.Unlock()
				name := "GetWindowLongPtrW"
				if strconv.IntSize == 32 {
					name = "GetWindowLongW"
				}
				style, _, err := windows.NewLazySystemDLL("user32.dll").NewProc(name).Call(hwnd, ^uintptr(19))
				if style == 0 {
					return fmt.Errorf("验证字幕窗口样式: %w", err)
				}
				if style&0x08000000 == 0 {
					return fmt.Errorf("字幕窗口缺少 WS_EX_NOACTIVATE")
				}
				return nil
			}
			s.Unlock()
		}
		select {
		case <-deadline.C:
			return fmt.Errorf("等待字幕首帧及可见窗口超时（10 秒）")
		case <-tick.C:
		}
	}
}
