package cmd

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/collector"
	"github.com/spf13/cobra"
)

const collectorPprofAddress = "127.0.0.1:6060"

// collectorCmd represents the collector command
var collectorCmd = &cobra.Command{
	Use:   "collector",
	Short: "Receive telemetry and publish it to Kafka",
	Long: `Start the telemetry collector HTTP service.

The collector accepts OpenTelemetry logs, metrics, and traces under
/telemetry/v1, validates JSON or protobuf payloads, normalizes metric
observations, and publishes the resulting telemetry messages to Kafka.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireValidConfig()

		col, err := collector.New(*animaConfig)
		if err != nil {
			panic(err)
		}

		// signal channel to capture system calls
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

		// start shutdown goroutine
		go func() {
			// capture sigterm and other system call here
			<-sigCh
			if err := col.Shutdown(); err != nil {
				log.Error().Err(err).Msg("collector shutdown failed")
				return
			}
			log.Info().Msg("collector shutdown...")
		}()

		go func() {
			log.Info().Str("address", collectorPprofAddress).Msg("starting collector pprof server")
			if err := http.ListenAndServe(collectorPprofAddress, nil); err != nil {
				log.Warn().Err(err).Str("address", collectorPprofAddress).Msg("collector pprof server stopped")
			}
		}()

		// start http server
		if err := col.Listen(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(collectorCmd)
}
