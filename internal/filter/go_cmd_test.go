package filter

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GoTestStrategy
// ---------------------------------------------------------------------------

func TestGoTestStrategy_CanHandle(t *testing.T) {
	s := &GoTestStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"go test bare", "go", []string{"test"}, true},
		{"go test with flags", "go", []string{"test", "./..."}, true},
		{"go test with leading flag", "go", []string{"-v", "test"}, true},
		{"go build", "go", []string{"build"}, false},
		{"not go command", "notgo", []string{"test"}, false},
		{"empty args", "go", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.CanHandle(tc.command, tc.args)
			if got != tc.want {
				t.Errorf("CanHandle(%q, %v) = %v, want %v", tc.command, tc.args, got, tc.want)
			}
		})
	}
}

func TestGoTestStrategy_Name(t *testing.T) {
	s := &GoTestStrategy{}
	if got := s.Name(); got != "go-test" {
		t.Errorf("Name() = %q, want %q", got, "go-test")
	}
}

func TestGoTestStrategy_Filter_AllPass(t *testing.T) {
	s := &GoTestStrategy{}

	input := "=== RUN   TestFoo\n" +
		"--- PASS: TestFoo (0.00s)\n" +
		"=== RUN   TestBar\n" +
		"--- PASS: TestBar (0.00s)\n" +
		"=== RUN   TestBaz\n" +
		"    baz_test.go:10: some log output\n" +
		"--- PASS: TestBaz (0.01s)\n" +
		"=== RUN   TestQux\n" +
		"--- PASS: TestQux (0.00s)\n" +
		"ok  \tgithub.com/example/pkg1\t0.234s\n" +
		"=== RUN   TestAlpha\n" +
		"--- PASS: TestAlpha (0.00s)\n" +
		"=== RUN   TestBeta\n" +
		"--- PASS: TestBeta (0.00s)\n" +
		"ok  \tgithub.com/example/pkg2\t0.123s\n" +
		"?   \tgithub.com/example/pkg3\t[no test files]\n"

	result := s.Filter([]byte(input), "go", []string{"test", "./..."}, 0)

	// Exit code 0: only package summaries + summary line
	if !strings.Contains(result.Filtered, "ok  \tgithub.com/example/pkg1\t0.234s") {
		t.Error("expected pkg1 summary line in output")
	}
	if !strings.Contains(result.Filtered, "ok  \tgithub.com/example/pkg2\t0.123s") {
		t.Error("expected pkg2 summary line in output")
	}
	if !strings.Contains(result.Filtered, "?   \tgithub.com/example/pkg3\t[no test files]") {
		t.Error("expected pkg3 no-test-files line in output")
	}
	if !strings.Contains(result.Filtered, "all tests passed (3 packages)") {
		t.Errorf("expected 'all tests passed (3 packages)', got:\n%s", result.Filtered)
	}

	// Individual test details should be stripped
	if strings.Contains(result.Filtered, "=== RUN") {
		t.Error("individual test run lines should be stripped on success")
	}
	if strings.Contains(result.Filtered, "--- PASS:") {
		t.Error("individual test pass lines should be stripped on success")
	}
	if strings.Contains(result.Filtered, "baz_test.go:10:") {
		t.Error("individual test log output should be stripped on success")
	}

	// WasReduced is based on length comparison
	if !result.WasReduced {
		t.Error("expected WasReduced=true since output was significantly reduced")
	}
}

func TestGoTestStrategy_Filter_SomeFail(t *testing.T) {
	s := &GoTestStrategy{}

	input := "=== RUN   TestGood\n" +
		"--- PASS: TestGood (0.00s)\n" +
		"=== RUN   TestBroken\n" +
		"    broken_test.go:42: expected 5, got 3\n" +
		"    broken_test.go:43: additional context\n" +
		"--- FAIL: TestBroken (0.01s)\n" +
		"=== RUN   TestAlsoGood\n" +
		"--- PASS: TestAlsoGood (0.00s)\n" +
		"FAIL\n" +
		"FAIL\tgithub.com/example/failing\t0.234s\n" +
		"=== RUN   TestOk\n" +
		"--- PASS: TestOk (0.00s)\n" +
		"ok  \tgithub.com/example/passing\t0.123s\n"

	result := s.Filter([]byte(input), "go", []string{"test", "./..."}, 1)

	// Failing test block should be preserved
	if !strings.Contains(result.Filtered, "=== RUN   TestBroken") {
		t.Error("failing test RUN line should be preserved")
	}
	if !strings.Contains(result.Filtered, "broken_test.go:42: expected 5, got 3") {
		t.Error("failing test error detail should be preserved")
	}
	if !strings.Contains(result.Filtered, "broken_test.go:43: additional context") {
		t.Error("failing test additional context should be preserved")
	}
	if !strings.Contains(result.Filtered, "--- FAIL: TestBroken") {
		t.Error("failing test FAIL line should be preserved")
	}

	// Package summaries should be preserved
	if !strings.Contains(result.Filtered, "FAIL\tgithub.com/example/failing\t0.234s") {
		t.Error("failing package summary should be preserved")
	}
	if !strings.Contains(result.Filtered, "ok  \tgithub.com/example/passing\t0.123s") {
		t.Error("passing package summary should be preserved")
	}

	// Standalone FAIL line
	if !strings.Contains(result.Filtered, "FAIL\n") {
		t.Error("standalone FAIL line should be preserved")
	}

	// Passing test details should be stripped
	if strings.Contains(result.Filtered, "=== RUN   TestGood") {
		t.Error("passing test RUN lines should be stripped on failure")
	}
	if strings.Contains(result.Filtered, "--- PASS: TestGood") {
		t.Error("passing test PASS lines should be stripped on failure")
	}
}

func TestGoTestStrategy_Filter_SmallOutput(t *testing.T) {
	s := &GoTestStrategy{}

	// Small output: < 10 lines and <= 2 packages
	input := "=== RUN   TestFoo\n" +
		"--- PASS: TestFoo (0.00s)\n" +
		"ok  \tgithub.com/example/pkg1\t0.234s\n"

	result := s.Filter([]byte(input), "go", []string{"test"}, 0)

	if result.WasReduced {
		t.Error("small output (< 10 lines, <= 2 packages) should not be reduced")
	}
	if result.Filtered != input {
		t.Errorf("small output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
	}
}

func TestGoTestStrategy_Filter_CompilationError(t *testing.T) {
	s := &GoTestStrategy{}

	input := "# github.com/example/pkg\n" +
		"./main.go:10:5: undefined: foo\n" +
		"./main.go:15:2: syntax error: unexpected newline\n" +
		"FAIL\tgithub.com/example/pkg [build failed]\n"

	result := s.Filter([]byte(input), "go", []string{"test", "./..."}, 2)

	// Compilation errors should be preserved as orphaned lines
	if !strings.Contains(result.Filtered, "# github.com/example/pkg") {
		t.Errorf("package header should be preserved, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "./main.go:10:5: undefined: foo") {
		t.Errorf("error line should be preserved, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "./main.go:15:2: syntax error") {
		t.Errorf("error line should be preserved, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "FAIL\tgithub.com/example/pkg [build failed]") {
		t.Errorf("FAIL summary should be preserved, got:\n%s", result.Filtered)
	}
}

// ---------------------------------------------------------------------------
// GoBuildStrategy
// ---------------------------------------------------------------------------

func TestGoBuildStrategy_CanHandle(t *testing.T) {
	s := &GoBuildStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"go build bare", "go", []string{"build"}, true},
		{"go build with flags", "go", []string{"build", "./..."}, true},
		{"go vet bare", "go", []string{"vet"}, true},
		{"go vet with flags", "go", []string{"vet", "./..."}, true},
		{"go install", "go", []string{"install"}, true},
		{"go install with leading flags", "go", []string{"-v", "install"}, true},
		{"go test", "go", []string{"test"}, false},
		{"go run", "go", []string{"run"}, false},
		{"not go command", "cargo", []string{"build"}, false},
		{"empty args", "go", nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.CanHandle(tc.command, tc.args)
			if got != tc.want {
				t.Errorf("CanHandle(%q, %v) = %v, want %v", tc.command, tc.args, got, tc.want)
			}
		})
	}
}

func TestGoBuildStrategy_Name(t *testing.T) {
	s := &GoBuildStrategy{}
	if got := s.Name(); got != "go-build" {
		t.Errorf("Name() = %q, want %q", got, "go-build")
	}
}

func TestGoBuildStrategy_Filter_Success(t *testing.T) {
	s := &GoBuildStrategy{}

	// Success: empty output is typical
	result := s.Filter([]byte(""), "go", []string{"build", "./..."}, 0)

	if result.WasReduced {
		t.Error("successful build (exit 0) should not be reduced")
	}
	if result.Filtered != "" {
		t.Errorf("empty success output should pass through unchanged, got: %q", result.Filtered)
	}
}

func TestGoBuildStrategy_Filter_SuccessWithOutput(t *testing.T) {
	s := &GoBuildStrategy{}

	// Some builds have verbose output on success
	input := "building github.com/example/pkg\n"
	result := s.Filter([]byte(input), "go", []string{"build", "-v", "./..."}, 0)

	if result.WasReduced {
		t.Error("successful build (exit 0) should not be reduced regardless of output")
	}
	if result.Filtered != input {
		t.Errorf("success output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
	}
}

func TestGoBuildStrategy_Filter_Failure(t *testing.T) {
	s := &GoBuildStrategy{}

	input := "# github.com/example/pkg\n" +
		"./main.go:10:5: undefined: foo\n" +
		"./main.go:15:2: syntax error: unexpected newline\n" +
		"./helper.go:3:8: imported and not used: \"fmt\"\n"

	result := s.Filter([]byte(input), "go", []string{"build", "./..."}, 1)

	// All lines are either package headers or error lines, so nothing to strip
	// The filter keeps package headers (# prefix) and compiler error lines
	if !strings.Contains(result.Filtered, "# github.com/example/pkg") {
		t.Error("package header should be preserved")
	}
	if !strings.Contains(result.Filtered, "./main.go:10:5: undefined: foo") {
		t.Error("error line should be preserved")
	}
	if !strings.Contains(result.Filtered, "./main.go:15:2: syntax error: unexpected newline") {
		t.Error("error line should be preserved")
	}
	if !strings.Contains(result.Filtered, "./helper.go:3:8: imported and not used: \"fmt\"") {
		t.Error("error line should be preserved")
	}
}

func TestGoBuildStrategy_Filter_FailureWithNoise(t *testing.T) {
	s := &GoBuildStrategy{}

	// Build failure with some non-error noise lines
	input := "# github.com/example/pkg\n" +
		"some verbose info line\n" +
		"another info line\n" +
		"./main.go:10:5: undefined: foo\n" +
		"./helper.go:3:8: imported and not used: \"fmt\"\n" +
		"yet another info line\n"

	result := s.Filter([]byte(input), "go", []string{"build", "./..."}, 1)

	if !result.WasReduced {
		t.Error("expected WasReduced=true when noise lines are stripped")
	}

	// Error lines and package header should be preserved
	if !strings.Contains(result.Filtered, "# github.com/example/pkg") {
		t.Error("package header should be preserved")
	}
	if !strings.Contains(result.Filtered, "./main.go:10:5: undefined: foo") {
		t.Error("error line should be preserved")
	}
	if !strings.Contains(result.Filtered, "./helper.go:3:8: imported and not used: \"fmt\"") {
		t.Error("error line should be preserved")
	}

	// Noise lines should be stripped
	if strings.Contains(result.Filtered, "some verbose info line") {
		t.Error("noise lines should be stripped")
	}
	if strings.Contains(result.Filtered, "another info line") {
		t.Error("noise lines should be stripped")
	}
	if strings.Contains(result.Filtered, "yet another info line") {
		t.Error("noise lines should be stripped")
	}
}
