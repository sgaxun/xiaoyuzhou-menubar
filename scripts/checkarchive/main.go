// Verify with Go's archive reader, the same reader used by the native updater.
// Windows bsdtar can reject UTF-8 Chinese names depending on the system locale.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func verify(archive, source, executable string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	target, err := os.MkdirTemp("", "xiaoyuzhou-archive-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(target)
	reader := tar.NewReader(gz)
	found := false
	for {
		h, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.FromSlash(h.Name)
		if !filepath.IsLocal(name) || strings.Contains(h.Name, "\\") {
			return fmt.Errorf("invalid archive path %q", h.Name)
		}
		path := filepath.Join(target, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(path, 0700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return err
			}
			out, err := os.Create(path)
			if err != nil {
				return err
			}
			hash := sha256.New()
			_, err = io.Copy(io.MultiWriter(out, hash), reader)
			closeErr := out.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
			original, err := os.ReadFile(filepath.Join(source, name))
			if err != nil {
				return err
			}
			want := sha256.Sum256(original)
			if !bytes.Equal(hash.Sum(nil), want[:]) {
				return fmt.Errorf("archive content differs: %s", name)
			}
			if name == executable {
				found = true
			}
		default:
			return fmt.Errorf("unexpected archive entry type %d: %s", h.Typeflag, name)
		}
	}
	if !found {
		return fmt.Errorf("archive lacks %s", executable)
	}
	return nil
}
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: checkarchive archive source-directory executable")
		os.Exit(1)
	}
	if err := verify(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Update archive extracted and matched native build")
}
