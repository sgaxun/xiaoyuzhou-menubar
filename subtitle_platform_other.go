//go:build !windows

package main

// Only Windows uses the separate desktop subtitle strip. macOS keeps its
// existing menu icon renderer in MenuSubtitle.
func (b *Backend) platformSubtitle(text string, progress float64) error { return nil }

func (b *Backend) verifyPlatformSubtitle() error { return nil }
