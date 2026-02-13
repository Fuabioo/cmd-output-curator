package filter

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// CargoTestStrategy
// ---------------------------------------------------------------------------

func TestCargoTestStrategy_CanHandle(t *testing.T) {
	s := &CargoTestStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"cargo test bare", "cargo", []string{"test"}, true},
		{"cargo test with manifest-path", "cargo", []string{"--manifest-path", "Cargo.toml", "test"}, true},
		{"cargo build", "cargo", []string{"build"}, false},
		{"rustc test", "rustc", []string{"test"}, false},
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

func TestCargoTestStrategy_Name(t *testing.T) {
	s := &CargoTestStrategy{}
	if got := s.Name(); got != "cargo-test" {
		t.Errorf("Name() = %q, want %q", got, "cargo-test")
	}
}

func TestCargoTestStrategy_Filter(t *testing.T) {
	s := &CargoTestStrategy{}

	t.Run("all tests passing", func(t *testing.T) {
		input := "   Compiling myproject v0.1.0 (/home/user/myproject)\n" +
			"    Finished test [unoptimized + debuginfo] target(s) in 2.34s\n" +
			"     Running unittests src/lib.rs (target/debug/deps/myproject-abc123)\n" +
			"\n" +
			"running 6 tests\n" +
			"test tests::test_add ... ok\n" +
			"test tests::test_subtract ... ok\n" +
			"test tests::test_multiply ... ok\n" +
			"test tests::test_divide ... ok\n" +
			"test tests::test_modulo ... ok\n" +
			"test tests::test_power ... ok\n" +
			"\n" +
			"test result: ok. 6 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.03s\n"

		result := s.Filter([]byte(input), "cargo", []string{"test"}, 0)

		// Should keep running line and test result summary
		if !strings.Contains(result.Filtered, "running 6 tests") {
			t.Error("expected 'running 6 tests' line to be preserved")
		}
		if !strings.Contains(result.Filtered, "test result: ok. 6 passed") {
			t.Error("expected 'test result:' summary to be preserved")
		}
		if !strings.Contains(result.Filtered, "all tests passed (6 total)") {
			t.Errorf("expected 'all tests passed (6 total)', got:\n%s", result.Filtered)
		}

		// Individual test ok lines should be stripped
		if strings.Contains(result.Filtered, "test tests::test_add ... ok") {
			t.Error("individual test ok lines should be stripped on success")
		}
		if strings.Contains(result.Filtered, "test tests::test_multiply ... ok") {
			t.Error("individual test ok lines should be stripped on success")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true since output was significantly reduced")
		}
	})

	t.Run("some tests failing", func(t *testing.T) {
		input := "   Compiling myproject v0.1.0 (/home/user/myproject)\n" +
			"    Finished test [unoptimized + debuginfo] target(s) in 1.50s\n" +
			"     Running unittests src/lib.rs (target/debug/deps/myproject-abc123)\n" +
			"\n" +
			"running 4 tests\n" +
			"test tests::test_add ... ok\n" +
			"test tests::test_subtract ... ok\n" +
			"test tests::test_divide ... FAILED\n" +
			"test tests::test_multiply ... ok\n" +
			"\n" +
			"failures:\n" +
			"\n" +
			"---- tests::test_divide stdout ----\n" +
			"thread 'tests::test_divide' panicked at 'assertion failed: `(left == right)`\n" +
			"  left: `0`,\n" +
			" right: `1`', src/lib.rs:42:9\n" +
			"\n" +
			"failures:\n" +
			"    tests::test_divide\n" +
			"\n" +
			"test result: FAILED. 3 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.02s\n"

		result := s.Filter([]byte(input), "cargo", []string{"test"}, 101)

		// "running N tests" header should be preserved for multi-target context
		if !strings.Contains(result.Filtered, "running 4 tests") {
			t.Error("'running N tests' header should be preserved on failure for multi-target context")
		}

		// Failures section should be preserved
		if !strings.Contains(result.Filtered, "failures:") {
			t.Error("failures: section should be preserved")
		}
		if !strings.Contains(result.Filtered, "tests::test_divide stdout") {
			t.Error("failure details should be preserved")
		}
		if !strings.Contains(result.Filtered, "assertion failed") {
			t.Error("failure assertion message should be preserved")
		}

		// test result line should be preserved
		if !strings.Contains(result.Filtered, "test result: FAILED. 3 passed; 1 failed") {
			t.Error("test result summary should be preserved")
		}

		// test ... FAILED lines should be preserved
		if !strings.Contains(result.Filtered, "test tests::test_divide ... FAILED") {
			t.Error("test ... FAILED lines should be preserved")
		}

		// Passing test ok lines should be stripped
		if strings.Contains(result.Filtered, "test tests::test_add ... ok") {
			t.Error("passing test ok lines should be stripped on failure")
		}
		if strings.Contains(result.Filtered, "test tests::test_subtract ... ok") {
			t.Error("passing test ok lines should be stripped on failure")
		}

		// Compiling/Finished noise should be stripped
		if strings.Contains(result.Filtered, "Compiling myproject") {
			t.Error("Compiling lines should be stripped on failure")
		}
	})

	t.Run("small output", func(t *testing.T) {
		input := "running 1 test\n" +
			"test tests::test_add ... ok\n" +
			"\n" +
			"test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s\n"

		result := s.Filter([]byte(input), "cargo", []string{"test"}, 0)

		if result.WasReduced {
			t.Error("small output (< 10 lines) should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("small output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := s.Filter([]byte(""), "cargo", []string{"test"}, 0)

		if result.WasReduced {
			t.Error("empty input should not be reduced")
		}
		if result.Filtered != "" {
			t.Errorf("empty input should produce empty output, got: %q", result.Filtered)
		}
	})
}

// ---------------------------------------------------------------------------
// CargoBuildStrategy
// ---------------------------------------------------------------------------

func TestCargoBuildStrategy_CanHandle(t *testing.T) {
	s := &CargoBuildStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"cargo build bare", "cargo", []string{"build"}, true},
		{"cargo check bare", "cargo", []string{"check"}, true},
		{"cargo clippy bare", "cargo", []string{"clippy"}, true},
		{"cargo test", "cargo", []string{"test"}, false},
		{"cargo run", "cargo", []string{"run"}, false},
		{"gcc build", "gcc", []string{"build"}, false},
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

func TestCargoBuildStrategy_Name(t *testing.T) {
	s := &CargoBuildStrategy{}
	if got := s.Name(); got != "cargo-build" {
		t.Errorf("Name() = %q, want %q", got, "cargo-build")
	}
}

func TestCargoBuildStrategy_Filter(t *testing.T) {
	s := &CargoBuildStrategy{}

	t.Run("successful build", func(t *testing.T) {
		input := "   Compiling libc v0.2.150\n" +
			"   Compiling cfg-if v1.0.0\n" +
			"   Compiling myproject v0.1.0 (/home/user/myproject)\n" +
			"    Finished dev [unoptimized + debuginfo] target(s) in 5.23s\n"

		result := s.Filter([]byte(input), "cargo", []string{"build"}, 0)

		if result.WasReduced {
			t.Error("successful build (exit 0) should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("success output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("compilation errors", func(t *testing.T) {
		input := "   Compiling myproject v0.1.0 (/home/user/myproject)\n" +
			"error[E0308]: mismatched types\n" +
			"  --> src/main.rs:10:5\n" +
			"   |\n" +
			"10 |     let x: u32 = \"hello\";\n" +
			"   |                  ^^^^^^^ expected `u32`, found `&str`\n" +
			"   |\n" +
			"= note: expected type `u32`\n" +
			"           found type `&str`\n" +
			"\n" +
			"For more information about this error, try `rustc --explain E0308`.\n" +
			"error: could not compile `myproject` due to previous error\n" +
			"aborting due to previous error\n"

		result := s.Filter([]byte(input), "cargo", []string{"build"}, 101)

		if !result.WasReduced {
			t.Error("expected WasReduced=true when Compiling noise is stripped")
		}

		// Error lines should be preserved
		if !strings.Contains(result.Filtered, "error[E0308]: mismatched types") {
			t.Error("error[E0308] line should be preserved")
		}
		if !strings.Contains(result.Filtered, "--> src/main.rs:10:5") {
			t.Error("--> location line should be preserved")
		}
		if !strings.Contains(result.Filtered, "aborting due to previous error") {
			t.Error("aborting line should be preserved")
		}
		if !strings.Contains(result.Filtered, "For more information about this error") {
			t.Error("For more information line should be preserved")
		}
		if !strings.Contains(result.Filtered, "error: could not compile") {
			t.Error("error: line should be preserved")
		}
		if !strings.Contains(result.Filtered, "= note: expected type") {
			t.Error("= note line should be preserved")
		}

		// Compiling noise should be stripped
		if strings.Contains(result.Filtered, "Compiling myproject") {
			t.Error("Compiling lines should be stripped on failure")
		}

		// Source context lines (pipe-prefixed) should be preserved
		if !strings.Contains(result.Filtered, "let x: u32") {
			t.Error("source code lines (pipe-prefixed) should be preserved for diagnostic context")
		}
		if !strings.Contains(result.Filtered, "expected `u32`, found `&str`") {
			t.Error("inline annotation lines should be preserved for diagnostic context")
		}
		// Pipe-only separator lines should be preserved
		if !strings.Contains(result.Filtered, "|") {
			t.Error("pipe separator lines should be preserved")
		}
	})

	t.Run("warnings only", func(t *testing.T) {
		input := "   Compiling myproject v0.1.0 (/home/user/myproject)\n" +
			"warning: unused variable: `x`\n" +
			"  --> src/main.rs:5:9\n" +
			"   |\n" +
			"5  |     let x = 42;\n" +
			"   |         ^ help: if this is intentional, prefix it with an underscore: `_x`\n" +
			"   |\n" +
			"= note: `#[warn(unused_variables)]` on by default\n" +
			"\n" +
			"warning: `myproject` (bin \"myproject\") generated 1 warning\n" +
			"    Finished dev [unoptimized + debuginfo] target(s) in 0.50s\n"

		result := s.Filter([]byte(input), "cargo", []string{"check"}, 0)

		// Exit 0 → pass through unchanged
		if result.WasReduced {
			t.Error("warnings with exit 0 should not be reduced (passthrough)")
		}
		if result.Filtered != input {
			t.Errorf("exit 0 output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := s.Filter([]byte(""), "cargo", []string{"build"}, 0)

		if result.WasReduced {
			t.Error("empty input should not be reduced")
		}
		if result.Filtered != "" {
			t.Errorf("empty input should produce empty output, got: %q", result.Filtered)
		}
	})
}
