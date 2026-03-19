package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rigel-labs/rigel-jd-collector/internal/adapter/jdclient"
	"github.com/rigel-labs/rigel-jd-collector/internal/app"
	"github.com/rigel-labs/rigel-jd-collector/internal/config"
	"github.com/rigel-labs/rigel-jd-collector/internal/repository/postgres"
	collectorservice "github.com/rigel-labs/rigel-jd-collector/internal/service/collector"
)

func main() {
	configPathFlag := flag.String("config", "", "path to YAML config file")
	flag.Parse()

	configPath := *configPathFlag
	if configPath == "" {
		configPath = os.Getenv("RIGEL_CONFIG_PATH")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repo, err := postgres.New(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("init repository: %v", err)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			log.Printf("close repository: %v", err)
		}
	}()

	client := jdclient.NewMockClient()
	if cfg.JDCollectorMode != "mock" {
		log.Printf("jd collector mode %q requested; falling back to mock client until real adapter is implemented", cfg.JDCollectorMode)
	}

	collector := collectorservice.New(repo, client, time.Now)
	go func() {
		if err := collector.RunScheduleLoop(ctx, cfg.ServiceName, cfg.JDCollectorMode); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("jd collector schedule loop stopped: %v", err)
		}
	}()

	application := app.New(cfg, collector)
	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      application.Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown server: %v", err)
		}
	}()

	log.Printf("starting %s on :%s", cfg.ServiceName, cfg.HTTPPort)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server exited: %v", err)
	}
}
