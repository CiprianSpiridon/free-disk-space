package scan

import "strings"

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if isShellSafe(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isShellSafe(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '/', '.', '-', '_', '=', '@', '+', ',', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func rmRf(path string) string {
	return "rm -rf " + shellQuote(path)
}
