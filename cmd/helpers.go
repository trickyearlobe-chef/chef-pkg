package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/trickyearlobe-chef/chef-pkg/pkg/chefapi"
)

// filterPackages filters a slice of FlatPackage by platform and/or architecture
// using case-insensitive substring matching. If both filters are empty, the
// original slice is returned unmodified.
func filterPackages(packages []chefapi.FlatPackage, platform, arch string) []chefapi.FlatPackage {
	if platform == "" && arch == "" {
		return packages
	}

	var filtered []chefapi.FlatPackage
	for _, pkg := range packages {
		if platform != "" && !strings.Contains(strings.ToLower(pkg.Platform), strings.ToLower(platform)) {
			continue
		}
		if arch != "" && !strings.Contains(strings.ToLower(pkg.Architecture), strings.ToLower(arch)) {
			continue
		}
		filtered = append(filtered, pkg)
	}
	return filtered
}

// outputTable prints a slice of FlatPackage as an aligned text table to stdout.
func outputTable(packages []chefapi.FlatPackage) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PLATFORM\tVERSION\tARCH\tPACKAGE VERSION\tURL\tSHA256")
	fmt.Fprintln(w, "--------\t-------\t----\t---------------\t---\t------")
	for _, pkg := range packages {
		sha := pkg.SHA256
		if len(sha) > 12 {
			sha = sha[:12] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			pkg.Platform,
			pkg.PlatformVersion,
			pkg.Architecture,
			pkg.Version,
			pkg.URL,
			sha,
		)
	}
	return w.Flush()
}

// outputJSON prints a slice of FlatPackage as pretty-printed JSON to stdout.
func outputJSON(packages []chefapi.FlatPackage) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(packages)
}
