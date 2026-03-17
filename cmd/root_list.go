package cmd

import (
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List Chef products, versions, or packages",
	Long: `Query the Progress Chef commercial downloads API to list available
products, product versions, or downloadable packages.

Use a subcommand to specify what to list.`,
}

func init() {
	rootCmd.AddCommand(listCmd)
}
