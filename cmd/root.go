package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/spaghettifunk/anima/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var animaConfig *config.Config

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "anima",
	Short: "Run anima services",
	Long: `Anima runs the services that collect, index, enrich, and
report on metric usage data.

Each subcommand starts one service process using settings loaded from
config.toml in the current directory or ./anima.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	animaConfig = config.New()

	// Default logging level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if animaConfig.Logger.Level != "" {
		level, err := zerolog.ParseLevel(strings.ToLower(animaConfig.Logger.Level))
		if err != nil {
			level = zerolog.InfoLevel
		}
		zerolog.SetGlobalLevel(level)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
}

func requireValidConfig() {
	if err := animaConfig.Validate(); err != nil {
		fmt.Println("configuration did not pass validation:", err)
		os.Exit(1)
	}
}

func init() {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./telemetry")
	if err := viper.ReadInConfig(); err != nil {
		if e, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			fmt.Println("config.toml not found")
		} else {
			// Config file was found but another error was produced
			fmt.Println(e.Error())
		}
	}

	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})

	viper.WatchConfig()
}
