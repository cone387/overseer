//go:build !windows

package locale

// detectLanguage uses environment variables on non-Windows platforms.
func detectLanguage() string {
	return detectFromEnv()
}
