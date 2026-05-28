package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	// GitHub repo for release checks
	repoOwner = "cone387"
	repoName  = "overseer"
	checkInterval = 24 * time.Hour
)

// githubRelease represents the GitHub API response for latest release.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Name    string `json:"name"`
}

// Result holds the update check result.
type Result struct {
	Available   bool
	Version     string
	DownloadURL string
}

// Check queries GitHub for a newer version.
// Returns nil if no update is available or if the check should be skipped.
func Check(currentVersion string, lastCheck time.Time) *Result {
	// Skip if checked recently
	if !lastCheck.IsZero() && time.Since(lastCheck) < checkInterval {
		return nil
	}

	// Skip if version is "dev" (local build)
	if currentVersion == "" || currentVersion == "dev" {
		return nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[updater] failed to check for updates: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil
	}

	// Compare versions (strip "desktop-v" prefix)
	latestVersion := strings.TrimPrefix(release.TagName, "desktop-v")
	currentClean := strings.TrimPrefix(currentVersion, "desktop-v")

	if isNewer(latestVersion, currentClean) {
		return &Result{
			Available:   true,
			Version:     release.TagName,
			DownloadURL: release.HTMLURL,
		}
	}

	return nil
}

// isNewer returns true if a > b using simple string comparison.
// For proper semver, a more sophisticated comparison could be used.
func isNewer(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] > bParts[i] {
			return true
		}
		if aParts[i] < bParts[i] {
			return false
		}
	}
	return len(aParts) > len(bParts)
}
