package fsx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func IsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// CopyFile copies src to dst, creating parent directories, preserving mode.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// CopyDir recursively copies src into dst.
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		return CopyFile(p, target)
	})
}

// ReplaceDir atomically swaps dst with src (src must be on the same filesystem).
func ReplaceDir(src, dst string) error {
	backup := dst + ".old"
	_ = os.RemoveAll(backup)
	if Exists(dst) {
		if err := os.Rename(dst, backup); err != nil {
			return fmt.Errorf("move old dir aside: %w", err)
		}
	}
	if err := os.Rename(src, dst); err != nil {
		_ = os.Rename(backup, dst)
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}
