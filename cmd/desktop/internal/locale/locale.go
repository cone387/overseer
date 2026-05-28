package locale

import (
	"os"
	"strings"
)

// Lang is the detected language code.
var Lang = "zh"

// activeStrings holds the active language pack.
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
func T(key string) string {
	if s, ok := activeStrings[key]; ok {
		return s
	}
	if s, ok := enStrings[key]; ok {
		return s
	}
	return key
}

// detectLanguage detects the OS language. Platform-specific implementations
// are in locale_windows.go and locale_other.go.
// This is the fallback that checks environment variables.
func detectFromEnv() string {
	for _, env := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		if val := os.Getenv(env); val != "" {
			if strings.HasPrefix(strings.ToLower(val), "zh") {
				return "zh"
			}
			return "en"
		}
	}
	return "zh" // default to Chinese
}
