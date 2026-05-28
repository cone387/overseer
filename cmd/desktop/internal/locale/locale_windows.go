//go:build windows

package locale

import "syscall"

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")
)

// detectLanguage uses the Windows GetUserDefaultUILanguage API.
func detectLanguage() string {
	ret, _, _ := procGetUserDefaultUILanguage.Call()
	langID := uint16(ret)

	// Primary language is the low 10 bits
	// LANG_CHINESE = 0x04
	primaryLang := langID & 0x3FF
	if primaryLang == 0x04 {
		return "zh"
	}

	// Fallback to env vars
	return detectFromEnv()
}
