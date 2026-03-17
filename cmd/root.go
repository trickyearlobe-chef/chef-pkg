package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	licenseID string
	baseURL   string
	channel   string
)

var rootCmd = &cobra.Command{
	Use:   "chef-pkg",
	Short: "Fetch Chef client packages from the Progress Chef downloads API",
	Long: `chef-pkg is a CLI utility for querying the Progress Chef commercial
downloads API to discover and list available Chef client packages.

It supports filtering by platform, architecture, and product, and can
output results as a human-readable table or as JSON.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&licenseID, "license-id", "", "Chef license ID (required, or set CHEF_LICENSE_ID env var)")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "https://commercial-acceptance.downloads.chef.co", "Base URL of the Chef downloads API")
	rootCmd.PersistentFlags().StringVar(&channel, "channel", "current", "Release channel (e.g. current, stable)")
}
