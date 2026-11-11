package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"xiaoyuzhou-menubar/internal/appicon"
	"xiaoyuzhou-menubar/internal/appupdate"
	"xiaoyuzhou-menubar/internal/player"
)

//go:embed assets/player.html
var page string

type Backend struct {
	service            *player.Service
	updates            *appupdate.Controller
	win                *mygo.Window
	tray               *mygo.Tray
	mediaMu            sync.Mutex
	nativeShareDialogs atomic.Int32
	subtitle           string
	subtitleOffset     int
	subtitleRenderer   subtitleRenderer
	subtitleToggle     *mygo.MenuItem
	pinned             atomic.Bool
	pinToggle          *mygo.MenuItem
	title              string
	quitting           bool
}

// Request dispatches directly to the Go backend. No subprocess or local server.
func (b *Backend) Request(action string, body any) (any, error) {
	return b.service.Request(action, body)
}

func (b *Backend) Hide() { b.win.Hide() }

func (b *Backend) Pinned() bool { return b.pinned.Load() }
func (b *Backend) SetPinned(pinned bool) bool {
	b.pinned.Store(pinned)
	b.win.SetAlwaysOnTop(pinned)
	if b.pinToggle != nil {
		b.pinToggle.SetChecked(pinned)
	}
	go b.win.Page().Eval(fmt.Sprintf("window.dispatchEvent(new CustomEvent('xyz-pin-changed',{detail:%t}))", pinned))
	return pinned
}

func (b *Backend) OpenLink(link string) error {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto") {
		return fmt.Errorf("不支持的链接")
	}
	return mygo.Shell.OpenExternal(link)
}

func (b *Backend) MediaState(title, artist string, playing bool, position, duration float64) {
	b.mediaMu.Lock()
	defer b.mediaMu.Unlock()
	if title == "" {
		return
	}
	if b.title != title {
		b.title = title
		mygo.NowPlaying.Set(mygo.NowPlayingInfo{Title: title, Artist: artist, Duration: time.Duration(duration * float64(time.Second)), Commands: mygo.MediaPlay | mygo.MediaPause | mygo.MediaNext | mygo.MediaPrevious | mygo.MediaSeek})
		b.tray.SetToolTip(title + " — 小宇宙")
	}
	state := mygo.PlaybackPaused
	if playing {
		state = mygo.PlaybackPlaying
	}
	mygo.NowPlaying.SetPlayback(state, time.Duration(position*float64(time.Second)))
}

// MenuSubtitle follows the audio clock, including seeks and pauses.
func (b *Backend) MenuSubtitle(text string, progress float64) error {
	b.mediaMu.Lock()
	defer b.mediaMu.Unlock()
	if b.tray == nil {
		return nil
	}
	return b.subtitleRenderer.update(text, progress, &b.subtitle, &b.subtitleOffset, func(raw []byte) error {
		b.tray.SetTitle("")
		return b.tray.SetIcon(raw, true)
	})
}
func (b *Backend) MenuSubtitleEnabled(enabled bool) {
	if b.subtitleToggle != nil {
		b.subtitleToggle.SetChecked(enabled)
	}
}

func (b *Backend) show() {
	anchor := b.tray.Bounds()
	bounds := b.win.Bounds()
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: anchor.X + anchor.Width/2, Y: anchor.Y + anchor.Height/2}).WorkArea
	x := max(area.X, min(anchor.X+(anchor.Width-bounds.Width)/2, area.X+area.Width-bounds.Width))
	b.win.SetPosition(x, max(area.Y, anchor.Y+anchor.Height+6))
	b.win.Show()
	b.win.Focus()
}
func pointInTray(point mygo.Point, bounds mygo.Rectangle) bool {
	return bounds.Width > 0 && bounds.Height > 0 && point.X >= bounds.X && point.X < bounds.X+bounds.Width && point.Y >= bounds.Y && point.Y < bounds.Y+bounds.Height
}

func (b *Backend) toggle() {
	if b.win.IsVisible() {
		b.win.Hide()
	} else {
		b.show()
	}
}

func (b *Backend) ready() {
	var err error
	b.tray, err = mygo.NewTray(mygo.TrayOptions{ID: "xiaoyuzhou-player", Icon: trayIcon(), IconIsTemplate: true, ToolTip: "小宇宙播放器 — 点击显示或隐藏播放器"})
	if err != nil {
		log.Fatal(err)
	}
	b.win = mygo.NewWindow(mygo.WindowOptions{Title: "小宇宙播放器", Width: 460, Height: 620, Hidden: true, Frameless: true, DisableResize: true, DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true, AlwaysOnTop: false, SkipTaskbar: true, Page: mygo.PageOptions{Autoplay: mygo.AutoplayAllow, BackgroundThrottling: mygo.ThrottlingOff}})
	b.win.OnBlur(func() {
		// AppKit may blur the window on mouse-down before the tray click fires.
		// Let that click decide whether to hide, rather than hiding then reopening.
		if pointInTray(mygo.Screen.CursorScreenPoint(), b.tray.Bounds()) {
			return
		}
		if !b.pinned.Load() && b.nativeShareDialogs.Load() == 0 {
			b.win.Hide()
		}
	})
	b.win.OnClose(func(e *mygo.CloseEvent) {
		if !b.quitting {
			e.PreventDefault()
			b.win.Hide()
		}
	})
	b.win.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		if e.URL != "about:blank" {
			e.PreventDefault()
		}
	})
	var firstLoad sync.Once
	b.win.Page().OnDidFinishLoad(func() { firstLoad.Do(b.show) })
	b.win.Page().LoadHTML(page, "")
	b.tray.OnClick(b.toggle)
	b.subtitleToggle = &mygo.MenuItem{Label: "菜单栏显示字幕", Type: mygo.MenuItemCheckbox, Checked: false, Click: func(*mygo.MenuItem, *mygo.Window) {
		go b.win.Page().Eval("window.dispatchEvent(new Event('xyz-toggle-menubar-transcript'))")
	}}
	b.pinToggle = &mygo.MenuItem{Label: "置顶窗口", Type: mygo.MenuItemCheckbox, Click: func(item *mygo.MenuItem, _ *mygo.Window) { b.SetPinned(item.IsChecked()) }}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开播放器", Click: func(*mygo.MenuItem, *mygo.Window) { b.show() }},
		{Label: "播放 / 暂停", Click: func(*mygo.MenuItem, *mygo.Window) { go b.win.Page().Eval("window.menuPlayer?.play()") }},
		{Label: "下一集", Click: func(*mygo.MenuItem, *mygo.Window) { go b.win.Page().Eval("window.menuPlayer?.next()") }},
		b.pinToggle,
		b.subtitleToggle,
		mygo.Separator(), {Label: "退出小宇宙播放器", Role: mygo.RoleQuit},
	})
	b.tray.OnRightClick(func() {
		if !b.pinned.Load() {
			b.win.Hide()
		}
		b.tray.SetMenu(menu)
		defer b.tray.SetMenu(nil)
		b.tray.PopUpMenu()
	})
	mygo.NowPlaying.OnCommand(func(cmd mygo.MediaCommand, pos time.Duration) {
		js := ""
		switch cmd {
		case mygo.MediaPlay:
			js = "if(!window.menuPlayer.status().playing)window.menuPlayer.play()"
		case mygo.MediaPause:
			js = "if(window.menuPlayer.status().playing)window.menuPlayer.play()"
		case mygo.MediaNext:
			js = "window.menuPlayer.next()"
		case mygo.MediaPrevious:
			js = "window.menuPlayer.previous()"
		case mygo.MediaSeek:
			js = fmt.Sprintf("window.menuPlayer.seek(%f)", pos.Seconds())
		}
		if js != "" {
			go b.win.Page().Eval(js)
		}
	})

}

func trayIcon() []byte { return appicon.TemplatePNG() }

func main() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Library/Application Support/XiaoyuzhouMenuBar")
	if custom := os.Getenv("XIAOYUZHOU_MENUBAR_DATA_DIR"); custom != "" {
		dir = custom
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Fatal("初始化播放器数据失败：", err)
	}
	service, err := player.New(dir)
	if err != nil {
		log.Fatal("初始化播放器服务失败：", err)
	}
	episodeCtx, cancelEpisodes := context.WithCancel(context.Background())
	defer cancelEpisodes()
	b := &Backend{service: service, updates: appupdate.New(currentVersion(), updateDriver{}, filepath.Join(dir, "updates.json"))}
	mygo.Bind(b)
	mygo.App.SetName("小宇宙播放器")
	mygo.Theme.SetSource(mygo.ThemeSystem)
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { b.quitting = true })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { cancelEpisodes(); b.updates.Close(); mygo.NowPlaying.Clear(); b.tray.Destroy() })
	mygo.App.OnActivate(func(visible bool) {
		if !visible && b.win != nil {
			b.show()
		}
	})
	mygo.App.WhenReady(func() {
		b.ready()
		b.updates.Start()
		go service.RunEpisodeUpdateScheduler(episodeCtx)
		go service.StartCloudSync(episodeCtx)
	})
	if err = mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
