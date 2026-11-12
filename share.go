package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"xiaoyuzhou-menubar/internal/podcast"
)

const maxSharePNG = 8 << 20

func episodeShareURL(eid string) (string, error) {
	if !podcast.IsValidID(eid) {
		return "", fmt.Errorf("无效的单集 ID")
	}
	return "https://www.xiaoyuzhoufm.com/episode/" + eid, nil
}
func episodeShareText(title, podcastTitle string, position float64, includeTime bool) (string, error) {
	for _, s := range []string{title, podcastTitle} {
		if !utf8.ValidString(s) || utf8.RuneCountInString(s) > 500 || strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) }) >= 0 {
			return "", fmt.Errorf("分享标题无效或过长")
		}
	}
	if math.IsNaN(position) || math.IsInf(position, 0) || position < 0 || position > 7*24*3600 {
		return "", fmt.Errorf("播放时间无效")
	}
	text := strings.TrimSpace(title)
	if p := strings.TrimSpace(podcastTitle); p != "" {
		if text != "" {
			text += " — "
		}
		text += p
	}
	if includeTime {
		s := int(position)
		stamp := fmt.Sprintf("%02d:%02d", s/60, s%60)
		if s >= 3600 {
			stamp = fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
		}
		text += "\n从 " + stamp + " 开始听"
	}
	return strings.TrimSpace(text), nil
}
func telegramShareURL(link, text string, desktop bool) string {
	q := url.Values{"url": {link}, "text": {text}}
	if desktop {
		return "tg://msg_url?" + q.Encode()
	}
	return "https://t.me/share/url?" + q.Encode()
}

// ShareEpisode opens a composer/selection UI; it never chooses a recipient.
func (b *Backend) ShareEpisode(destination, eid, title, podcastTitle string, position float64, includeTime bool) error {
	link, err := episodeShareURL(eid)
	if err != nil {
		return err
	}
	text, err := episodeShareText(title, podcastTitle, position, includeTime)
	if err != nil {
		return err
	}
	if includeTime {
		link += fmt.Sprintf("#ts=%d", int(position))
	}
	switch destination {
	case "wechat":
		// This extension only accepts URL items; copy the description separately.
		if err := runShareHelper(map[string]string{"destination": "wechat", "url": link}); err != nil {
			return err
		}
		mygo.Clipboard.WriteText(text)
		return nil
	case "telegram":
		desktop := runShareHelper(map[string]string{"destination": "telegram-probe"}) == nil
		return mygo.Shell.OpenExternal(telegramShareURL(link, text, desktop))
	default:
		return fmt.Errorf("不支持的分享目标")
	}
}
func (b *Backend) CopyEpisodeLink(eid string) error {
	return b.CopyEpisodeLinkAtTime(eid, 0, false)
}
func (b *Backend) CopyEpisodeLinkAtTime(eid string, position float64, includeTime bool) error {
	link, err := episodeShareURL(eid)
	if err != nil {
		return err
	}
	if includeTime {
		if _, err := episodeShareText("", "", position, true); err != nil {
			return err
		}
		link += fmt.Sprintf("#ts=%d", int(position))
	}
	mygo.Clipboard.WriteText(link)
	return nil
}
func decodeSharePNG(dataURL string) ([]byte, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) || len(dataURL) > len(prefix)+base64.StdEncoding.EncodedLen(maxSharePNG) {
		return nil, fmt.Errorf("分享卡必须为不超过 8MB 的 PNG")
	}
	data, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil || len(data) > maxSharePNG {
		return nil, fmt.Errorf("PNG 编码无效")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 16_000_000 {
		return nil, fmt.Errorf("PNG 尺寸无效或过大")
	}
	if _, err = png.Decode(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("PNG 内容无效: %w", err)
	}
	return data, nil
}
func (b *Backend) SaveShareCard(dataURL string) (bool, error) {
	data, err := decodeSharePNG(dataURL)
	if err != nil {
		return false, err
	}
	b.nativeShareDialogs.Add(1)
	defer b.nativeShareDialogs.Add(-1)
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: b.win, Title: "保存分享卡", DefaultPath: "小宇宙分享卡.png", Filters: []mygo.FileFilter{{Name: "PNG 图片", Extensions: []string{"png"}}}})
	if err != nil || path == "" {
		return false, err
	}
	// Stage with private permissions and rename, avoiding partial files and symlink writes.
	f, err := os.CreateTemp(filepath.Dir(path), ".share-card-*.png")
	if err != nil {
		return false, err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return false, err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return false, err
	}
	if err = f.Close(); err != nil {
		return false, err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return false, err
	}
	return true, nil
}
func (b *Backend) CopyShareCard(dataURL string) error {
	data, err := decodeSharePNG(dataURL)
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := mygo.Clipboard.WriteImage(data); err != nil {
			return fmt.Errorf("复制卡片失败: %w", err)
		}
		return mygo.Clipboard.Flush()
	}
	helper, err := shareHelperPath()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{"destination": "copy-card", "png": base64.StdEncoding.EncodeToString(data)})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, helper)
	cmd.Stdin = bytes.NewReader(payload)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("复制卡片失败: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}
func (b *Backend) ShareCard(dataURL string) error {
	data, err := decodeSharePNG(dataURL)
	if err != nil {
		return err
	}
	return runShareHelper(map[string]string{"destination": "system", "png": base64.StdEncoding.EncodeToString(data)})
}
func shareHelperPath() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("原生分享仅支持 macOS")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidates := []string{filepath.Join(filepath.Dir(exe), "../Resources/bin/小宇宙分享助手.app/Contents/MacOS/share-helper"), "resources/bin/小宇宙分享助手.app/Contents/MacOS/share-helper", filepath.Join(filepath.Dir(exe), "../Resources/bin/share-helper"), filepath.Join(filepath.Dir(exe), "bin/share-helper"), "resources/bin/share-helper"}
	for _, p := range candidates {
		if s, e := os.Stat(p); e == nil && s.Mode().IsRegular() && s.Mode()&0111 != 0 {
			return filepath.Abs(p)
		}
	}
	return "", fmt.Errorf("分享助手未构建，请运行 macOS 打包流程")
}
func runShareHelper(payload map[string]string) (err error) {
	cleanup := func() {}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	if raw := payload["png"]; raw != "" {
		data, err := base64.StdEncoding.Strict().DecodeString(raw)
		if err != nil {
			return err
		}
		dir, err := os.MkdirTemp("", "xiaoyuzhou-share-card-")
		if err != nil {
			return err
		}
		cleanup = func() { _ = os.RemoveAll(dir) }
		file := filepath.Join(dir, "小宇宙分享卡.png")
		if err := os.WriteFile(file, data, 0600); err != nil {
			return err
		}
		payload = map[string]string{"destination": "system", "file": file}
	}

	path, err := shareHelperPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return runShareCommand(path, data, payload["destination"] == "telegram-probe", cleanup)
}

// READY means UI opened, not that the user sent anything. A background Wait
// reaps the child; cancellation/deadlines remain active after this returns.
func runShareCommand(path string, data []byte, probe bool, onExit ...func()) error {
	cleanup := func() {
		for _, done := range onExit {
			done()
		}
	}
	started := false
	defer func() {
		if !started {
			cleanup()
		}
	}()
	timeout := 125 * time.Second
	if probe {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(data)
	cmd.WaitDelay = time.Second
	if probe {
		defer cancel()
		return cmd.Run()
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		cancel()
		return err
	}
	started = true
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		ok := scanner.Scan() && scanner.Text() == "READY"
		ready <- ok
		for scanner.Scan() {
		}
	}()
	done := make(chan error, 1)
	go func() { err := cmd.Wait(); cancel(); cleanup(); done <- err }()
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	select {
	case ok := <-ready:
		if ok {
			return nil
		}
		cancel()
		<-done
		return fmt.Errorf("原生分享未打开: %s", strings.TrimSpace(stderr.String()))
	case err := <-done:
		return fmt.Errorf("原生分享未打开: %s (%v)", strings.TrimSpace(stderr.String()), err)
	case <-timer.C:
		cancel()
		return fmt.Errorf("原生分享启动超时")
	}
}
