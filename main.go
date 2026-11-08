package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"xiaoyuzhou-menubar/internal/appupdate"
	"xiaoyuzhou-menubar/internal/player"
)

//go:embed assets/player.html
var page string

type Backend struct {
	service  *player.Service
	updates  *appupdate.Controller
	win      *mygo.Window
	tray     *mygo.Tray
	mediaMu  sync.Mutex
	title    string
	quitting bool
}

// Request dispatches directly to the Go backend. No subprocess or local server.
func (b *Backend) Request(action string, body any) (any, error) {
	return b.service.Request(action, body)
}

func (b *Backend) Hide() { b.win.Hide() }

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

func (b *Backend) show() {
	anchor := b.tray.Bounds()
	bounds := b.win.Bounds()
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: anchor.X + anchor.Width/2, Y: anchor.Y + anchor.Height/2}).WorkArea
	x := max(area.X, min(anchor.X+(anchor.Width-bounds.Width)/2, area.X+area.Width-bounds.Width))
	b.win.SetPosition(x, max(area.Y, anchor.Y+anchor.Height+6))
	b.win.Show()
	b.win.Focus()
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
	b.tray, err = mygo.NewTray(mygo.TrayOptions{ID: "xiaoyuzhou-player", Icon: trayIcon(), IconIsTemplate: true, ToolTip: "小宇宙 — 点击打开播放器"})
	if err != nil {
		log.Fatal(err)
	}
	b.win = mygo.NewWindow(mygo.WindowOptions{Title: "小宇宙菜单栏", Width: 460, Height: 620, Hidden: true, Frameless: true, DisableResize: true, DisableMinimize: true, DisableMaximize: true, DisableFullScreen: true, AlwaysOnTop: true, SkipTaskbar: true, Page: mygo.PageOptions{Autoplay: mygo.AutoplayAllow, BackgroundThrottling: mygo.ThrottlingOff}})
	b.win.OnBlur(func() {
		b.win.Hide()
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
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开播放器", Click: func(*mygo.MenuItem, *mygo.Window) { b.show() }},
		{Label: "播放 / 暂停", Click: func(*mygo.MenuItem, *mygo.Window) { go b.win.Page().Eval("window.menuPlayer?.play()") }},
		{Label: "下一集", Click: func(*mygo.MenuItem, *mygo.Window) { go b.win.Page().Eval("window.menuPlayer?.next()") }},
		mygo.Separator(), {Label: "退出小宇宙", Role: mygo.RoleQuit},
	})
	b.tray.OnRightClick(func() { b.win.Hide(); b.tray.SetMenu(menu); defer b.tray.SetMenu(nil); b.tray.PopUpMenu() })
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

// A monochrome planet with a tilted orbit. Transparent separation keeps the
// foreground orbit readable when macOS tints the template white or dark.
func trayIcon() []byte {
	const size = 36
	const samples = 8
	const angle = -38 * math.Pi / 180
	c, s := math.Cos(angle), math.Sin(angle)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			covered := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					dx := float64(x) + (float64(sx)+0.5)/samples - size/2
					dy := float64(y) + (float64(sy)+0.5)/samples - size/2
					u, v := c*dx+s*dy, -s*dx+c*dy
					const a, b = 17.0, 4.8
					gradient := 2 * math.Hypot(u/(a*a), v/(b*b))
					distance := math.Inf(1)
					if gradient > 0 {
						distance = math.Abs((u*u/(a*a) + v*v/(b*b) - 1) / gradient)
					}
					planet := dx*dx+dy*dy <= 10.5*10.5
					orbit := distance <= 0.85
					// The back arc passes behind the planet; the front arc gets a clear gap.
					if planet && v > 0 && distance < 1.9 {
						planet = false
					}
					if planet || orbit {
						covered++
					}
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: uint8(covered * 255 / (samples * samples))})
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, img)
	return out.Bytes()
}

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
	b := &Backend{service: service, updates: appupdate.New(currentVersion(), updateDriver{}, filepath.Join(dir, "updates.json"))}
	mygo.Bind(b)
	mygo.App.SetName("小宇宙菜单栏")
	mygo.Theme.SetSource(mygo.ThemeSystem)
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { b.quitting = true })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { b.updates.Close(); mygo.NowPlaying.Clear(); b.tray.Destroy() })
	mygo.App.OnActivate(func(visible bool) {
		if !visible && b.win != nil {
			b.show()
		}
	})
	mygo.App.WhenReady(func() { b.ready(); b.updates.Start() })
	if err = mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
