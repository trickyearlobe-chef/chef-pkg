package cmd

func resolveRepoPrefix(repoPrefix string) string {
	if repoPrefix != "" {
		return repoPrefix
	}
	return "chef"
}
