package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/internal/ideconfig"
)

// version is set at build time via ldflags.
var version = "dev"

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "chef-pkg",
	Short: "Fetch Chef client packages from the Progress Chef downloads API",
	Long: `chef-pkg is a CLI utility for querying the Progress Chef commercial
downloads API to discover, download, and upload Chef client packages
to artifact repositories like Nexus and Artifactory.`,
}

// Execute runs the root command. It checks for --install/--uninstall before
// Cobra dispatch because those flags live on the root and Cobra shows help
// (instead of running PersistentPreRunE) when no subcommand is given.
func Execute() {
	// Parse flags early so we can inspect --install/--uninstall
	rootCmd.InitDefaultHelpFlag()
	rootCmd.ParseFlags(os.Args[1:])

	install, _ := rootCmd.Flags().GetBool("install")
	uninstall, _ := rootCmd.Flags().GetBool("uninstall")

	if install || uninstall {
		runInstallUninstall(install, uninstall)
		return
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runInstallUninstall handles --install and --uninstall, then exits.
func runInstallUninstall(install, uninstall bool) {
	if install && uninstall {
		fmt.Fprintln(os.Stderr, "Error: --install and --uninstall are mutually exclusive")
		os.Exit(1)
	}

	binaryPath, err := ideconfig.ResolveBinaryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var results []ideconfig.Result
	if install {
		results = ideconfig.Install("chef-pkg", binaryPath, []string{"serve"})
	} else {
		results = ideconfig.Uninstall("chef-pkg")
	}

	// Print results
	maxNameLen := 0
	for _, r := range results {
		if len(r.IDE) > maxNameLen {
			maxNameLen = len(r.IDE)
		}
	}
	for _, r := range results {
		status := r.Status
		if r.Message != "" {
			status += " (" + r.Message + ")"
		}
		fmt.Fprintf(os.Stderr, "%-*s %s\n", maxNameLen+1, r.IDE+":", status)
	}

	if ideconfig.HasErrors(results) {
		os.Exit(1)
	}
	os.Exit(0)
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.chef-pkg.toml)")
	rootCmd.PersistentFlags().String("license-id", "", "Chef license ID (or set CHEFPKG_CHEF_LICENSE_ID)")
	rootCmd.PersistentFlags().String("base-url", "https://commercial-acceptance.downloads.chef.co", "Base URL of the Chef downloads API")
	rootCmd.PersistentFlags().String("channel", "stable", "Release channel (e.g. stable, current)")
	rootCmd.PersistentFlags().Bool("no-progress", false, "Force line-by-line output even in interactive mode")

	rootCmd.Flags().Bool("install", false, "Register as MCP server in supported IDEs and exit")
	rootCmd.Flags().Bool("uninstall", false, "Remove from supported IDE MCP configs and exit")

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
