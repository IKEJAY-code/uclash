package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"gitee.com/IKEJAY-code/uclash/internal/core"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	var (
		lines  int
		follow bool
	)
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show the core log (optionally follow it)",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !follow {
				text := core.TailLog(a.LogFile(), lines)
				if text == "" {
					fmt.Fprintf(out, "log is empty or missing: %s\n", a.LogFile())
					return nil
				}
				fmt.Fprintln(out, text)
				return nil
			}
			return followLog(cmd.Context(), a.LogFile(), out, lines)
		},
	}
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "number of trailing lines to show")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing new log lines")
	return cmd
}

func followLog(ctx context.Context, path string, out io.Writer, lines int) error {
	if text := core.TailLog(path, lines); text != "" {
		fmt.Fprintln(out, text)
	}
	var offset int64
	if fi, err := os.Stat(path); err == nil {
		offset = fi.Size()
	}
	buf := make([]byte, 16<<10)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		f, err := os.Open(path)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		fi, err := f.Stat()
		if err == nil {
			if fi.Size() < offset {
				offset = 0
			}
			if _, err := f.Seek(offset, io.SeekStart); err == nil {
				n, _ := f.Read(buf)
				if n > 0 {
					if _, werr := out.Write(buf[:n]); werr != nil {
						f.Close()
						return werr
					}
					offset += int64(n)
				}
			}
		}
		f.Close()
		time.Sleep(300 * time.Millisecond)
	}
}
