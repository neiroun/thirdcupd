package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/neiroun/thirdcupd/internal/checks"
	"github.com/neiroun/thirdcupd/internal/config"
	"github.com/neiroun/thirdcupd/internal/events"
	"github.com/neiroun/thirdcupd/internal/server"
	"github.com/neiroun/thirdcupd/internal/state"
)

var ErrUnhealthy = errors.New("unhealthy checks")

type Daemon struct {
	cfg     config.Config
	checks  []checks.Checker
	store   *state.Store
	events  *events.Writer
	servers []*server.Server
}

func New(cfg config.Config) (*Daemon, error) {
	writer, err := events.New(cfg.Observability.EventLog)
	if err != nil {
		return nil, err
	}

	store := state.New(cfg.Daemon.FailureThreshold, cfg.Daemon.SuccessThreshold)
	app := &Daemon{
		cfg:    cfg,
		checks: checks.Build(cfg),
		store:  store,
		events: writer,
	}
	if cfg.Observability.ListenAddr != "" {
		app.servers = append(app.servers, server.New(cfg.Observability.ListenAddr, store))
	}
	return app, nil
}

func (d *Daemon) Run(ctx context.Context, once bool) error {
	serverErrs := make(chan error, len(d.servers))
	if !once {
		for _, srv := range d.servers {
			server := srv
			go func() {
				if err := server.Run(ctx); err != nil {
					serverErrs <- fmt.Errorf("observability server: %w", err)
				}
			}()
		}
	}

	if err := d.runCycle(ctx); err != nil {
		return err
	}
	if once {
		if d.store.Snapshot().Overall != state.StatusHealthy {
			return ErrUnhealthy
		}
		return nil
	}

	ticker := time.NewTicker(d.cfg.Daemon.Interval.Duration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serverErrs:
			return err
		case <-ticker.C:
			if err := d.runCycle(ctx); err != nil {
				return err
			}
		}
	}
}

func (d *Daemon) Close() error {
	return d.events.Close()
}

func (d *Daemon) runCycle(ctx context.Context) error {
	var wg sync.WaitGroup
	results := make(chan checks.Result, len(d.checks))

	for _, checker := range d.checks {
		checker := checker
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- checker.Check(ctx)
		}()
	}

	wg.Wait()
	close(results)

	for result := range results {
		update := d.store.Apply(result)
		if err := d.events.Write(result, update); err != nil {
			return err
		}
	}
	return nil
}
