package core

import "os"

func removeAll(p string) error { return os.RemoveAll(p) }
func rename(a, b string) error { return os.Rename(a, b) }
