package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/neiroun/thirdcupd/internal/config"
	"github.com/neiroun/thirdcupd/internal/daemon"
)

const version = "0.1.0"

func main() {
	configPath := flag.String("config", "thirdcupd.json", "path to config file")
	once := flag.Bool("once", false, "run one check cycle and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	app, err := daemon.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon error: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, *once); err != nil {
		if errors.Is(err, daemon.ErrUnhealthy) {
			fmt.Fprintln(os.Stderr, "one or more checks are unhealthy")
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}
