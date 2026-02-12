package filter

import "regexp"

// ansiPattern matches ANSI escape sequences (CSI, OSC, simple escapes).
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;:?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[()][AB012]|\x1b[=>]`)

// StripANSI removes ANSI escape sequences from a byte slice.
func StripANSI(b []byte) []byte {
	return ansiPattern.ReplaceAll(b, nil)
}

// StripANSIString removes ANSI escape sequences from a string.
func StripANSIString(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}
