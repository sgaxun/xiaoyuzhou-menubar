package main

import (
	"net/url"
	"runtime"
	"testing"
)

func TestPowerShellLiteral(t *testing.T) {
	if got := powershellLiteral(`C:\Program Files\O'Brien\app.exe`); got != `'C:\Program Files\O''Brien\app.exe'` {
		t.Fatal(got)
	}
}

func TestWindowsUpdateFeedConfiguration(t *testing.T) {
	original := buildConfig
	t.Cleanup(func() { buildConfig = original })
	for _, config := range []string{
		`{`, `{}`, `{"updates":{"github":"owner/repo"}}`,
		`{"updates":{"publicKey":"key"}}`,
		`{"updates":{"github":"owner/repo","publicKey":"key","url":"https://other.example/feed"}}`,
		`{"updates":{"github":"owner/repo","publicKey":"key","tagPrefix":"beta-"}}`,
	} {
		buildConfig = []byte(config)
		if got := windowsUpdateFeed(); got != "" {
			t.Fatalf("unsupported config enabled feed: %s", got)
		}
	}
	buildConfig = []byte(`{"updates":{"github":"owner/repo","publicKey":"key"}}`)
	u, err := url.Parse(windowsUpdateFeed())
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Host != "github.com" || u.Path != "/owner/repo/releases/latest/download/update-windows-"+runtime.GOARCH+".json" {
		t.Fatalf("unexpected discovery URL: %s", u)
	}
}
