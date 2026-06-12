package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/reporter"
	"github.com/spf13/cobra"
)

// reporterCmd represents the reporter command
var reporterCmd = &cobra.Command{
	Use:   "reporter",
	Short: "Serve metric usage reports over HTTP",
	Long: `Start the reporting HTTP service.

The reporter reads indexed observations and usage evidence from Postgres and
serves report endpoints for metric summaries, metric details, and supporting
evidence under /api/v1/reports.`,
	Run: func(cmd *cobra.Command, args []string) {
		requireValidConfig()

		rep, err := reporter.New(*animaConfig)
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
			_ = rep.Shutdown()
			log.Info().Msg("reporter shutdown...")
		}()

		if err := rep.Initialize(); err != nil {
			panic(err)
		}

		// start http server
		if err := rep.Listen(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(reporterCmd)
}
