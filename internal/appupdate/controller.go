// Package appupdate owns user-controlled update state. It never installs or
// restarts as a result of an automatic check.
package appupdate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Candidate interface {
	Version() string
	Notes() string
	Install(context.Context, func(int64, int64)) error
}
type Driver interface {
	Enabled() bool
	Check(context.Context) (Candidate, error)
	Restart()
}
type Status struct {
	Version          string `json:"version"`
	Enabled          bool   `json:"enabled"`
	Automatic        bool   `json:"automatic"`
	Phase            string `json:"phase"`
	AvailableVersion string `json:"availableVersion"`
	Notes            string `json:"notes"`
	Message          string `json:"message"`
	Downloaded       int64  `json:"downloaded"`
	Total            int64  `json:"total"`
	LastChecked      string `json:"lastChecked"`
}
type Controller struct {
	mu               sync.Mutex
	driver           Driver
	status           Status
	candidate        Candidate
	preferences      string
	ctx              context.Context
	cancel           context.CancelFunc
	restartRequested bool
	once             sync.Once
}

func New(version string, driver Driver, preferences string) *Controller {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Controller{driver: driver, preferences: preferences, ctx: ctx, cancel: cancel, status: Status{Version: version, Automatic: true, Phase: "idle"}}
	if raw, err := os.ReadFile(preferences); err == nil {
		var p struct {
			Automatic *bool `json:"automatic"`
		}
		if json.Unmarshal(raw, &p) == nil && p.Automatic != nil {
			c.status.Automatic = *p.Automatic
		}
	}
	c.status.Enabled = driver.Enabled()
	if !c.status.Enabled {
		c.status.Phase = "disabled"
		c.status.Message = "此构建未启用在线更新，或应用安装位置不可写。"
	}
	return c
}
func (c *Controller) Status() Status { c.mu.Lock(); defer c.mu.Unlock(); return c.status }
func (c *Controller) SetAutomatic(enabled bool) (Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.preferences != "" {
		raw, _ := json.Marshal(map[string]bool{"automatic": enabled})
		if err := os.MkdirAll(filepath.Dir(c.preferences), 0700); err != nil {
			return c.status, err
		}
		f, err := os.CreateTemp(filepath.Dir(c.preferences), ".updates-*.tmp")
		if err != nil {
			return c.status, err
		}
		name := f.Name()
		defer os.Remove(name)
		if _, err = f.Write(raw); err != nil {
			f.Close()
			return c.status, err
		}
		if err = f.Close(); err != nil {
			return c.status, err
		}
		if err = os.Rename(name, c.preferences); err != nil {
			return c.status, err
		}
	}
	c.status.Automatic = enabled
	return c.status, nil
}
func (c *Controller) Start() {
	c.once.Do(func() {
		go func() {
			timer := time.NewTimer(10 * time.Second)
			defer timer.Stop()
			for {
				select {
				case <-c.ctx.Done():
					return
				case <-timer.C:
					c.mu.Lock()
					automatic := c.status.Automatic
					c.mu.Unlock()
					if automatic {
						c.Check()
					}
					timer.Reset(6 * time.Hour)
				}
			}
		}()
	})
}
func (c *Controller) Close() { c.cancel() }
func (c *Controller) Check() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx.Err() != nil || c.status.Phase == "checking" || c.status.Phase == "downloading" || c.status.Phase == "ready" {
		return c.status
	}
	c.status.Enabled = c.driver.Enabled()
	if !c.status.Enabled {
		c.status.Phase = "disabled"
		c.status.Message = "此构建未启用在线更新，或应用安装位置不可写。"
		return c.status
	}
	c.status.Phase = "checking"
	c.status.Message = "正在检查更新…"
	go c.check()
	return c.status
}
func (c *Controller) check() {
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()
	up, err := c.driver.Check(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx.Err() != nil {
		return
	}
	c.status.LastChecked = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		c.status.Phase = "error"
		c.status.Message = "检查失败，请检查网络或稍后重试。"
		return
	}
	c.candidate = up
	c.status.Downloaded = 0
	c.status.Total = 0
	if up == nil {
		c.status.Phase = "current"
		c.status.AvailableVersion = ""
		c.status.Notes = ""
		c.status.Message = "已是最新版本"
		return
	}
	c.status.Phase = "available"
	c.status.AvailableVersion = up.Version()
	c.status.Notes = up.Notes()
	c.status.Message = "发现新版本，可下载后手动重启安装。"
}
func (c *Controller) Download() (Status, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status.Phase == "downloading" || c.status.Phase == "ready" {
		return c.status, nil
	}
	if c.ctx.Err() != nil {
		return c.status, errors.New("更新服务已停止")
	}
	if c.candidate == nil || c.status.Phase == "checking" {
		return c.status, errors.New("请先检查更新")
	}
	up := c.candidate
	c.status.Phase = "downloading"
	c.status.Downloaded = 0
	c.status.Total = 0
	c.status.Message = "正在下载并校验更新…"
	go func() {
		ctx, cancel := context.WithTimeout(c.ctx, 20*time.Minute)
		defer cancel()
		err := up.Install(ctx, func(downloaded, total int64) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.status.Downloaded = downloaded
			c.status.Total = total
		})
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.ctx.Err() != nil {
			return
		}
		if err != nil {
			c.status.Phase = "error"
			c.status.Message = "更新下载或校验失败，当前版本未更换，可重试下载。"
			return
		}
		c.status.Phase = "ready"
		c.status.Message = "更新已校验，点击重启后安装。"
	}()
	return c.status, nil
}
func (c *Controller) Restart() error {
	c.mu.Lock()
	if c.status.Phase != "ready" {
		c.mu.Unlock()
		return errors.New("更新尚未准备完成")
	}
	if c.restartRequested {
		c.mu.Unlock()
		return nil
	}
	c.restartRequested = true
	c.mu.Unlock()
	c.driver.Restart()
	return nil
}
