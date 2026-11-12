package main

import "testing"

func TestWindowsUpdateVersionBoundary(t *testing.T) {
	for _, tc := range []struct {
		next, current  string
		newer, invalid bool
	}{
		{"0.3.14", "0.3.13", true, false},
		{"0.10.0", "0.9.99", true, false},
		{"1.0.0", "0.99.99", true, false},
		{"0.3.13", "0.3.13", false, false},
		{"0.3.12", "0.3.13", false, false},
		{"0.3.14-beta.1", "0.3.13", false, true},
		{"0.3.14 -File evil", "0.3.13", false, true},
		{"0.3.'14", "0.3.13", false, true},
		{"0.03.14", "0.3.13", false, true},
		{"0.3.+14", "0.3.13", false, true},
		{"0.3.4294967296", "0.3.13", false, true},
		{"0.3.14", "invalid", false, true},
	} {
		t.Run(tc.next+"/"+tc.current, func(t *testing.T) {
			got, err := newerStableVersion(tc.next, tc.current)
			if got != tc.newer || (err != nil) != tc.invalid {
				t.Fatalf("got (%v, %v)", got, err)
			}
		})
	}
}
