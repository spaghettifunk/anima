package cmd

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/entityresolver"
	"github.com/spaghettifunk/anima/internal/storage/postgres"
	"github.com/spf13/cobra"
)

var entityResolverCmd = &cobra.Command{
	Use:   "entityresolver",
	Short: "Resolve raw OTLP telemetry into entity topology",
	Long: `Run the entity resolver.

The resolver consumes raw OTLP metrics, logs, and traces from Kafka, extracts
canonical entities and relationships, writes current topology to Memgraph, and
stores service health windows and change history in Postgres.`,
	Run: func(cmd *cobra.Command, args []string) {
		graph := entityresolver.NewMemgraphStoreFromConfig(*animaConfig)
		analytics := postgres.NewFromConfig(*animaConfig)
		resolver, err := entityresolver.New(*animaConfig, graph, analytics)
		if err != nil {
			panic(err)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

		go func() {
			<-sigCh
			_ = resolver.Shutdown()
			log.Info().Msg("entity resolver shutdown...")
		}()

		if err := resolver.Initialize(); err != nil {
			panic(err)
		}

		wg := &sync.WaitGroup{}
		wg.Add(1)
		wg.Go(func() {
			defer wg.Done()
			if err := resolver.Start(); err != nil {
				log.Fatal().Err(err).Msg("failed to consume entity resolver messages")
			}
		})
		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(entityResolverCmd)
}
