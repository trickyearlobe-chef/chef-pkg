package cmd

import (
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download Chef packages to local disk",
	Long: `Download Chef packages from the Progress Chef commercial downloads API
to a local directory.

Use a subcommand to specify what to download.`,
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}
