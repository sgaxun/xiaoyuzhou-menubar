package main

import "fmt"

// The player service serializes partial preference patches with all other
// state writes. Never read/modify/write state.json outside that lock.
func (b *Backend) SubtitlePreference() (map[string]any, error) {
	result, err := b.service.Request("state", nil)
	if err != nil {
		return nil, err
	}
	response, ok := result.(map[string]any)
	if !ok || response["ok"] != true {
		return nil, fmt.Errorf("读取字幕设置失败：%v", result)
	}
	state, ok := response["state"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("字幕设置格式错误")
	}
	// Null distinguishes a legacy installation from an explicitly disabled
	// preference, so the UI can migrate its existing localStorage switch once.
	var enabled any
	if value, ok := state["menubarTranscript"].(bool); ok {
		enabled = value
	}
	return map[string]any{"enabled": enabled}, nil
}

func (b *Backend) SaveSubtitlePreference(enabled bool) error {
	result, err := b.service.Request("savePrefs", map[string]any{"menubarTranscript": enabled})
	if err != nil {
		return err
	}
	response, ok := result.(map[string]any)
	if !ok || response["ok"] != true {
		return fmt.Errorf("保存字幕设置失败：%v", result)
	}
	return nil
}
