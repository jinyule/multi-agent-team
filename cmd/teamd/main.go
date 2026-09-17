package main

import (
	"context"
	"flag"
	"fmt"
	"multi-agent-team/internal/daemon"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	stateDir, err := daemon.DefaultStateDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	flag.StringVar(&stateDir, "state-dir", stateDir, "private state directory (absolute path)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := daemon.Run(ctx, stateDir, func(socket string) { fmt.Printf("teamd ready: %s (protocol 1)\n", socket) }); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
