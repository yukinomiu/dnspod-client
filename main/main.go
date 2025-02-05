package main

import (
	"dnspod-ddns-client/internal/config"
	"dnspod-ddns-client/internal/modifier"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var (
		cfg    *config.Config
		stopCh = make(chan os.Signal, 1)
		s      os.Signal
	)

	// config
	if c, err := config.Get(); err != nil {
		panic(err)
	} else {
		cfg = c
		slog.Info("config", slog.String("value", fmt.Sprintf("%+v", *cfg)))
	}

	// D-DNS update
	go func() {
		modifier.Run(cfg)
	}()

	// block
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(stopCh)
		close(stopCh)
	}()
	s = <-stopCh
	slog.Info("stop signal received", slog.String("signal", s.String()))
}
