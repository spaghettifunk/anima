package cmd

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/indexer"
	"github.com/spf13/cobra"
)

// indexerCmd represents the indexer command
var indexerCmd = &cobra.Command{
	Use:   "indexer",
	Short: "Index collected metric observations into Postgres",
	Long: `Run the metric observation indexer.

The indexer consumes normalized metric observations from the configured Kafka
metrics topic, batches them, and upserts daily observation records into
Postgres for later usage analysis and reporting.`,
	Run: func(cmd *cobra.Command, args []string) {
		idx, err := indexer.New(*animaConfig)
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
			_ = idx.Shutdown()
			log.Info().Msg("indexer shutdown...")
		}()

		// initialize the consumers
		if err := idx.Initialize(); err != nil {
			panic(err)
		}

		wg := &sync.WaitGroup{}
		wg.Add(1)
		wg.Go(func() {
			defer wg.Done()
			if err := idx.Start(); err != nil {
				log.Fatal().Err(err).Msg("failed to consume messages...")
			}
		})
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(indexerCmd)
}
