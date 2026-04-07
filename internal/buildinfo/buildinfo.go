package buildinfo

import "fmt"

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func SetBuildInfo(version, date, commit string) {
	buildVersion = version
	buildDate = date
	buildCommit = commit
}

func PrintBuildInfo() {
	fmt.Printf("Build version: %s\nBuild date: %s\nBuild commit: %s\n",
		normalizeValue(buildVersion),
		normalizeValue(buildDate),
		normalizeValue(buildCommit))
}

func normalizeValue(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}
