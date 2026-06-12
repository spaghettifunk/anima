package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spaghettifunk/anima/queryaudit"
	"github.com/spf13/cobra"
)

var queryAuditProxyCmd = &cobra.Command{
	Use:   "query-audit-proxy",
	Short: "Proxy Thanos queries and publish audit events",
	Long: `Start the Query Audit Proxy service.

The proxy forwards Prometheus-compatible HTTP API requests to the configured
Thanos query frontend while publishing query audit events asynchronously to
Kafka.`,
	Run: func(cmd *cobra.Command, args []string) {
		proxy, err := queryaudit.New(*animaConfig)
		if err != nil {
			panic(err)
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

		go func() {
			<-sigCh
			if err := proxy.Shutdown(); err != nil {
				log.Error().Err(err).Msg("query audit proxy shutdown failed")
				return
			}
			log.Info().Msg("query audit proxy shutdown...")
		}()

		log.Info().
			Str("address", proxyAddress()).
			Str("upstream", animaConfig.QueryAuditProxy.Upstream).
			Str("audit_topic", animaConfig.QueryAuditProxy.AuditTopic).
			Msg("starting query audit proxy")
		if err := proxy.Listen(); err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(queryAuditProxyCmd)
}

func proxyAddress() string {
	return animaConfig.QueryAuditProxy.Address + ":" + fmt.Sprint(animaConfig.QueryAuditProxy.Port)
}
