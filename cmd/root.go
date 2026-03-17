package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "chef-pkg",
	Short: "Fetch Chef client packages from the Progress Chef downloads API",
	Long: `chef-pkg is a CLI utility for querying the Progress Chef commercial
downloads API to discover, download, and upload Chef client packages
to artifact repositories like Nexus and Artifactory.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.chef-pkg.toml)")
	rootCmd.PersistentFlags().String("license-id", "", "Chef license ID (or set CHEFPKG_CHEF_LICENSE_ID)")
	rootCmd.PersistentFlags().String("base-url", "https://commercial-acceptance.downloads.chef.co", "Base URL of the Chef downloads API")
	rootCmd.PersistentFlags().String("channel", "current", "Release channel (e.g. current, stable)")
	rootCmd.PersistentFlags().Bool("no-progress", false, "Force line-by-line output even in interactive mode")

	viper.BindPFlag("chef.license_id", rootCmd.PersistentFlags().Lookup("license-id"))
	viper.BindPFlag("chef.base_url", rootCmd.PersistentFlags().Lookup("base-url"))
	viper.BindPFlag("chef.channel", rootCmd.PersistentFlags().Lookup("channel"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: could not determine home directory:", err)
		} else {
			viper.AddConfigPath(home)
			viper.SetConfigType("toml")
			viper.SetConfigName(".chef-pkg")
		}
	}

	viper.SetEnvPrefix("CHEFPKG")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintln(os.Stderr, "Warning: error reading config file:", err)
		}
	}
}
