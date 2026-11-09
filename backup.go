package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"xiaoyuzhou-menubar/internal/player"
)

// ExportLocalBackup returns false on cancellation; no account/cache data is exported.
func (b *Backend) ExportLocalBackup() (bool, error) {
	raw, err := b.service.ExportLocalBackup()
	if err != nil {
		return false, err
	}
	b.nativeShareDialogs.Add(1)
	defer b.nativeShareDialogs.Add(-1)
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: b.win, Title: "导出本地备份", DefaultPath: "小宇宙本地备份.json", Filters: []mygo.FileFilter{{Name: "JSON 备份", Extensions: []string{"json"}}}})
	if err != nil || path == "" {
		return false, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".local-backup-*.tmp")
	if err != nil {
		return false, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return false, err
	}
	if _, err = f.Write(raw); err != nil {
		return false, err
	}
	if err = f.Sync(); err != nil {
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

// ImportLocalBackup returns true only after validation and durable state storage.
func (b *Backend) ImportLocalBackup() (bool, error) {
	b.nativeShareDialogs.Add(1)
	defer b.nativeShareDialogs.Add(-1)
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: b.win, Title: "导入本地备份", Filters: []mygo.FileFilter{{Name: "JSON 备份", Extensions: []string{"json"}}}})
	if err != nil || len(paths) == 0 {
		return false, err
	}
	if len(paths) != 1 {
		return false, fmt.Errorf("请选择一个备份文件")
	}
	f, err := os.Open(paths[0])
	if err != nil {
		return false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() > player.MaxLocalBackupBytes {
		return false, fmt.Errorf("请选择不超过 8MB 的备份文件")
	}
	raw, err := io.ReadAll(io.LimitReader(f, player.MaxLocalBackupBytes+1))
	if err != nil {
		return false, err
	}
	if err = b.service.ImportLocalBackup(raw); err != nil {
		return false, err
	}
	return true, nil
}
