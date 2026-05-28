package locale

import (
	"os"
	"runtime"
	"strings"
)

// Lang is the detected language code.
var Lang = "zh"

// strings holds the active language pack.
var activeStrings map[string]string

func init() {
	Lang = detectLanguage()
	switch Lang {
	case "zh":
		activeStrings = zhStrings
	default:
		activeStrings = enStrings
	}
}

// T returns the localized string for the given key.
// Falls back to the key itself if not found.
func T(key string) string {
	if s, ok := activeStrings[key]; ok {
		return s
	}
	// Fallback to English, then key
	if s, ok := enStrings[key]; ok {
		return s
	}
	return key
}

// detectLanguage detects the OS language and returns "zh" or "en".
func detectLanguage() string {
	switch runtime.GOOS {
	case "windows":
		return detectWindows()
	default:
		return detectUnix()
	}
}

func detectWindows() string {
	// Check common environment variables first
	for _, env := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		if val := os.Getenv(env); val != "" {
			if strings.HasPrefix(strings.ToLower(val), "zh") {
				return "zh"
			}
			return "en"
		}
	}
	// Default to Chinese for Windows (most Overseer users are Chinese)
	return "zh"
}

func detectUnix() string {
	for _, env := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		if val := os.Getenv(env); val != "" {
			lower := strings.ToLower(val)
			if strings.HasPrefix(lower, "zh") {
				return "zh"
			}
			return "en"
		}
	}
	return "zh"
}
