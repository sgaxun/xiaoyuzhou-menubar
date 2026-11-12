package main

import (
	"errors"
	"strconv"
	"strings"
)

// Only stable x.y.z versions are accepted, including in the privileged entry
// point. No URLs, paths, manifests or signing keys cross the elevation boundary.
func stableVersion(v string) ([3]uint64, error) {
	var result [3]uint64
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return result, errors.New("expected stable x.y.z version")
	}
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return result, errors.New("invalid version component")
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return result, errors.New("invalid version component")
			}
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return result, err
		}
		result[i] = n
	}
	return result, nil
}

func newerStableVersion(next, current string) (bool, error) {
	a, err := stableVersion(next)
	if err != nil {
		return false, err
	}
	b, err := stableVersion(current)
	if err != nil {
		return false, err
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i], nil
		}
	}
	return false, nil
}
