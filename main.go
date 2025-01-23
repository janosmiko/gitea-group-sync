package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/janosmiko/gitea-ldap-sync/internal/app"
	"github.com/janosmiko/gitea-ldap-sync/internal/config"
	"github.com/janosmiko/gitea-ldap-sync/internal/logger"
)

//nolint:gochecknoglobals
var conf *config.Config

func main() {
	logger.Configure()

	log := log.Logger.With().Str("tag", "[main]").Logger()

	var err error

	conf, err = config.New()
	if err != nil {
		log.Fatal().Err(err).Msg("Error")
	}

	mainJob() // First run for check settings

	if !conf.CronEnabled {
		log.Info().Msg("Cron is disabled, shutting down...")

		return
	}

	runCron()
}

func runCron() {
	log := log.Logger.With().Str("tag", "[cron]").Logger()

	sig := make(chan os.Signal, 1)
	signal.Notify(
		sig,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.VerbosePrintfLogger(logger.CronLogger()))))
	_, _ = c.AddFunc(conf.CronTimer, mainJob)

	c.Start()

	<-sig
	log.Info().Msgf("Received signal: %v", sig)

	ctx := c.Stop()

	// Wait for running jobs to complete
	const timeout = 60

	select {
	case <-ctx.Done():
		log.Info().Msg("All jobs completed, shutting down...")
	case <-time.After(timeout * time.Second):
		log.Info().Msg("Shutdown timed out after 60 seconds")
	}
}

func mainJob() {
	log := log.Logger.With().Str("tag", "[mainjob]").Logger()
	log.Info().Msg("Job started")

	c, err := app.New(conf)
	if err != nil {
		log.Fatal().Msgf("Error: %s", err)
	}
	defer c.Close()

	if err := c.Run(); err != nil {
		log.Panic().Msgf("Error: %s", err)
	}

	log.Info().Msg("Job done")
}
