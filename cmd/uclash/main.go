package main

import (
	"fmt"
	"os"

	"gitee.com/IKEJAY-code/uclash/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "uclash:", err)
		os.Exit(1)
	}
}
