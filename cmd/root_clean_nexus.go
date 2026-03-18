package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trickyearlobe-chef/chef-pkg/pkg/nexus"
)

var cleanNexusCmd = &cobra.Command{
	Use:    "nexus",
	Short:  "Remove Chef-owned Nexus repositories",
	Hidden: true,
	Long: `Remove Nexus repositories created by chef-pkg.

This command deletes repositories whose names start with chef or a known Chef
product prefix such as chef-ice or inspec.`,
	RunE: runCleanNexus,
}

func init() {
	cleanCmd.AddCommand(cleanNexusCmd)

	cleanNexusCmd.Flags().String("nexus-url", "", "Nexus server URL")
	cleanNexusCmd.Flags().String("nexus-username", "", "Nexus username")
	cleanNexusCmd.Flags().String("nexus-password", "", "Nexus password")

	_ = viper.BindPFlag("nexus.url", cleanNexusCmd.Flags().Lookup("nexus-url"))
	_ = viper.BindPFlag("nexus.username", cleanNexusCmd.Flags().Lookup("nexus-username"))
	_ = viper.BindPFlag("nexus.password", cleanNexusCmd.Flags().Lookup("nexus-password"))
}

func shouldCleanNexusRepo(repoName string) bool {
	if strings.HasPrefix(repoName, "chef-") {
		return true
	}
	for _, prefix := range []string{"chef-ice-", "inspec-"} {
		if strings.HasPrefix(repoName, prefix) {
			return true
		}
	}
	return false
}

func runCleanNexus(cmd *cobra.Command, args []string) error {
	nexusURL := viper.GetString("nexus.url")
	nexusUser := viper.GetString("nexus.username")
	nexusPass := viper.GetString("nexus.password")

	if nexusURL == "" {
		return fmt.Errorf("nexus URL is required: set --nexus-url, config nexus.url, or CHEFPKG_NEXUS_URL env var")
	}
	if nexusUser == "" {
		return fmt.Errorf("nexus username is required: set --nexus-username, config nexus.username, or CHEFPKG_NEXUS_USERNAME env var")
	}
	if nexusPass == "" {
		return fmt.Errorf("nexus password is required: set --nexus-password, config nexus.password, or CHEFPKG_NEXUS_PASSWORD env var")
	}

	nexusURL = strings.TrimRight(nexusURL, "/")
	client := nexus.NewClient(nexusURL, nexusUser, nexusPass)

	repos, err := client.Repos(cmd.Context())
	if err != nil {
		return err
	}

	var deleted, skipped int
	for _, repoName := range repos {
		if !shouldCleanNexusRepo(repoName) {
			skipped++
			continue
		}
		if err := client.DeleteRepo(cmd.Context(), repoName); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Deleted Nexus repo %s\n", repoName)
		deleted++
	}

	fmt.Fprintf(os.Stderr, "Done: %d deleted, %d skipped\n", deleted, skipped)
	return nil
}
