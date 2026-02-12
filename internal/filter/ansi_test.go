package filter

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no escape sequences", "hello world", "hello world"},
		{"bold text", "\x1b[1mhello\x1b[0m", "hello"},
		{"color text", "\x1b[31mred\x1b[0m", "red"},
		{"256 color", "\x1b[38;5;123mcolored\x1b[0m", "colored"},
		{"true color", "\x1b[38;2;255;128;0mtrue\x1b[0m", "true"},
		{"cursor show", "\x1b[?25h", ""},
		{"cursor hide", "\x1b[?25l", ""},
		{"alternate screen on", "\x1b[?1049h", ""},
		{"alternate screen off", "\x1b[?1049l", ""},
		{"bracketed paste on", "\x1b[?2004h", ""},
		{"bracketed paste off", "\x1b[?2004l", ""},
		{"OSC with BEL", "\x1b]0;title\x07text", "text"},
		{"OSC with ST", "\x1b]0;title\x1b\\text", "text"},
		{"charset selection B", "\x1b(B", ""},
		{"charset selection 0", "\x1b(0", ""},
		{"charset selection paren close", "\x1b)A", ""},
		{"keypad application mode", "\x1b=", ""},
		{"keypad numeric mode", "\x1b>", ""},
		{"mixed content", "line1\n\x1b[32mgreen\x1b[0m\nline3", "line1\ngreen\nline3"},
		{"empty input", "", ""},
		{"only escapes", "\x1b[1m\x1b[31m\x1b[0m", ""},
		{"cursor movement", "\x1b[10;20H", ""},
		{"erase line", "\x1b[2K", ""},
		{"scroll up", "\x1b[3S", ""},
		{"sgr reset", "\x1b[m", ""},
		{"multiple color params", "\x1b[1;31;42mbold red on green\x1b[0m", "bold red on green"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSIString(tt.input)
			if got != tt.want {
				t.Errorf("StripANSIString(%q) = %q, want %q", tt.input, got, tt.want)
			}

			// Also test the []byte version
			gotBytes := string(StripANSI([]byte(tt.input)))
			if gotBytes != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.input, gotBytes, tt.want)
			}
		})
	}
}

func TestStripANSINilInput(t *testing.T) {
	got := StripANSI(nil)
	if got != nil {
		t.Errorf("StripANSI(nil) = %v, want nil", got)
	}
}

func TestStripANSIPreservesPlainText(t *testing.T) {
	// Ensure normal text with various characters is not altered
	inputs := []string{
		"hello world",
		"path/to/file.go:42",
		"func main() { fmt.Println(\"hi\") }",
		"error: something went wrong!",
		"tab\there",
		"newline\nhere",
		"   indented   ",
		"special chars: @#$%^&*()",
	}
	for _, input := range inputs {
		got := StripANSIString(input)
		if got != input {
			t.Errorf("StripANSIString(%q) = %q, want unchanged", input, got)
		}
	}
}
