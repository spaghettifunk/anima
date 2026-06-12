package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/enricher"
	"github.com/spf13/cobra"
)

// enricherCmd represents the enricher command
var enricherCmd = &cobra.Command{
	Use:   "enricher",
	Short: "Extract metric usage evidence from observability sources",
	Long: `Run the usage enricher.

The enricher reads PromQL expressions from configured Thanos rules and Grafana
dashboards, extracts the metrics referenced by those expressions, and stores
daily usage evidence in Postgres for reporting.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireValidConfig()

		enr, err := enricher.New(*animaConfig)
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
			_ = enr.Shutdown()
			log.Info().Msg("enricher shutdown...")
		}()

		// initialize the enricher object
		if err := enr.Initialize(); err != nil {
			panic(err)
		}

		if err := enr.Start(); err != nil {
			log.Fatal().Err(err).Msg("failed to start the enricher process...")
		}
	},
}

func init() {
	rootCmd.AddCommand(enricherCmd)
}
