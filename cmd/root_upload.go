package cmd

import (
	"github.com/spf13/cobra"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload downloaded Chef packages to an artifact repository",
	Long: `Upload previously downloaded Chef packages to an artifact repository
such as Sonatype Nexus or JFrog Artifactory.

Packages must first be downloaded using 'download packages', or you can
use the --fetch flag on the subcommands to fetch from the Chef API, download,
and upload in a single pipeline.

Use a subcommand to specify the target repository type.`,
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
