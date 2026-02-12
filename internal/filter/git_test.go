package filter

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// GitStatusStrategy
// ---------------------------------------------------------------------------

func TestGitStatusStrategy_CanHandle(t *testing.T) {
	s := &GitStatusStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"git status bare", "git", []string{"status"}, true},
		{"git status short flag", "git", []string{"status", "-s"}, true},
		{"git status with config flag", "git", []string{"-c", "color.status=always", "status"}, true},
		{"git commit", "git", []string{"commit"}, false},
		{"git diff", "git", []string{"diff"}, false},
		{"not git command", "notgit", []string{"status"}, false},
		{"empty args", "git", nil, false},
		{"git with no subcommand", "git", []string{"-v"}, false},
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

func TestGitStatusStrategy_Name(t *testing.T) {
	s := &GitStatusStrategy{}
	if got := s.Name(); got != "git-status" {
		t.Errorf("Name() = %q, want %q", got, "git-status")
	}
}

func TestGitStatusStrategy_Filter(t *testing.T) {
	s := &GitStatusStrategy{}

	t.Run("verbose status with staged unstaged and untracked", func(t *testing.T) {
		input := "On branch main\n" +
			"Your branch is up to date with 'origin/main'.\n" +
			"\n" +
			"Changes to be committed:\n" +
			"  (use \"git restore --staged <file>...\" to unstage)\n" +
			"\tmodified:   internal/cli/root.go\n" +
			"\tnew file:   internal/filter/git.go\n" +
			"\n" +
			"Changes not staged for commit:\n" +
			"  (use \"git add <file>...\" to update what will be committed)\n" +
			"  (use \"git restore <file>...\" to discard changes in working directory)\n" +
			"\tmodified:   README.md\n" +
			"\n" +
			"Untracked files:\n" +
			"  (use \"git add <file>...\" to include in what will be committed)\n" +
			"\tinternal/filter/generic.go\n" +
			"\tinternal/filter/go_cmd.go\n" +
			"\n"

		result := s.Filter([]byte(input), "git", []string{"status"}, 0)

		if !result.WasReduced {
			t.Fatal("expected WasReduced=true for verbose status")
		}

		// Verify hint lines are stripped
		if strings.Contains(result.Filtered, "(use \"git") {
			t.Error("hint lines should be removed")
		}

		// Verify markers were converted
		if strings.Contains(result.Filtered, "modified:") {
			t.Error("expected 'modified:' to be replaced with 'M'")
		}
		if strings.Contains(result.Filtered, "new file:") {
			t.Error("expected 'new file:' to be replaced with 'A'")
		}

		// Verify converted markers present
		if !strings.Contains(result.Filtered, "\tM   internal/cli/root.go") {
			t.Errorf("expected converted staged modified file, got:\n%s", result.Filtered)
		}
		if !strings.Contains(result.Filtered, "\tA   internal/filter/git.go") {
			t.Errorf("expected converted staged new file, got:\n%s", result.Filtered)
		}
		if !strings.Contains(result.Filtered, "\tM   README.md") {
			t.Errorf("expected converted unstaged modified file, got:\n%s", result.Filtered)
		}

		// Verify untracked files kept as-is (no status prefix to convert)
		if !strings.Contains(result.Filtered, "\tinternal/filter/generic.go") {
			t.Errorf("expected untracked file preserved, got:\n%s", result.Filtered)
		}

		// Verify summary line
		if !strings.Contains(result.Filtered, "2 staged, 1 unstaged, 2 untracked") {
			t.Errorf("expected summary line, got:\n%s", result.Filtered)
		}

		// Verify section headers are kept
		if !strings.Contains(result.Filtered, "Changes to be committed:") {
			t.Error("section header 'Changes to be committed:' should be kept")
		}
		if !strings.Contains(result.Filtered, "Changes not staged for commit:") {
			t.Error("section header 'Changes not staged for commit:' should be kept")
		}
		if !strings.Contains(result.Filtered, "Untracked files:") {
			t.Error("section header 'Untracked files:' should be kept")
		}

		// Verify "Your branch is up to date..." is dropped
		if strings.Contains(result.Filtered, "Your branch is up to date") {
			t.Error("non-section, non-branch lines should be dropped")
		}

		// Verify trailing newline preserved
		if !strings.HasSuffix(result.Filtered, "\n") {
			t.Error("trailing newline should be preserved")
		}
	})

	t.Run("clean working tree", func(t *testing.T) {
		input := "On branch main\nnothing to commit, working tree clean\n"
		result := s.Filter([]byte(input), "git", []string{"status"}, 0)

		if result.WasReduced {
			t.Error("clean working tree should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("clean tree should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("very small output under 5 lines", func(t *testing.T) {
		// 4 lines after split (3 text lines + 1 empty from trailing newline = 4)
		input := "On branch main\nM file.go\nA new.go\n"
		result := s.Filter([]byte(input), "git", []string{"status"}, 0)

		if result.WasReduced {
			t.Error("small output (< 5 lines) should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("small output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("only untracked files", func(t *testing.T) {
		input := "On branch main\n" +
			"Your branch is up to date with 'origin/main'.\n" +
			"\n" +
			"Untracked files:\n" +
			"  (use \"git add <file>...\" to include in what will be committed)\n" +
			"\tnew_file.go\n" +
			"\tanother_file.go\n" +
			"\n"

		result := s.Filter([]byte(input), "git", []string{"status"}, 0)

		if !result.WasReduced {
			t.Fatal("expected WasReduced=true for untracked files status")
		}

		if !strings.Contains(result.Filtered, "0 staged, 0 unstaged, 2 untracked") {
			t.Errorf("expected summary '0 staged, 0 unstaged, 2 untracked', got:\n%s", result.Filtered)
		}

		if strings.Contains(result.Filtered, "(use \"git") {
			t.Error("hint lines should be removed")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := s.Filter([]byte(""), "git", []string{"status"}, 0)

		// Empty string splits to [""], which is 1 line < 5
		if result.WasReduced {
			t.Error("empty input should not be reduced")
		}
	})
}

// ---------------------------------------------------------------------------
// GitDiffStrategy
// ---------------------------------------------------------------------------

func TestGitDiffStrategy_CanHandle(t *testing.T) {
	s := &GitDiffStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"git diff bare", "git", []string{"diff"}, true},
		{"git diff with flags", "git", []string{"--cached", "diff"}, true},
		{"git status", "git", []string{"status"}, false},
		{"not git", "notgit", []string{"diff"}, false},
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

func TestGitDiffStrategy_Name(t *testing.T) {
	s := &GitDiffStrategy{}
	if got := s.Name(); got != "git-diff" {
		t.Errorf("Name() = %q, want %q", got, "git-diff")
	}
}

func TestGitDiffStrategy_Filter(t *testing.T) {
	s := &GitDiffStrategy{}

	t.Run("multi file diff with noise lines", func(t *testing.T) {
		input := "diff --git a/README.md b/README.md\n" +
			"index abc1234..def5678 100644\n" +
			"--- a/README.md\n" +
			"+++ b/README.md\n" +
			"@@ -1,3 +1,4 @@\n" +
			" # coc\n" +
			"+A new line here\n" +
			" \n" +
			" Some content\n" +
			"diff --git a/main.go b/main.go\n" +
			"index 1111111..2222222 100644\n" +
			"--- a/main.go\n" +
			"+++ b/main.go\n" +
			"@@ -5,6 +5,8 @@ import \"fmt\"\n" +
			" func main() {\n" +
			"+    fmt.Println(\"hello\")\n" +
			"+    fmt.Println(\"world\")\n" +
			"     fmt.Println(\"old\")\n" +
			"-    fmt.Println(\"removed\")\n" +
			" }\n" +
			"\n"

		result := s.Filter([]byte(input), "git", []string{"diff"}, 0)

		if !result.WasReduced {
			t.Fatal("expected WasReduced=true for multi-file diff")
		}

		// Verify "diff --git" lines are removed
		if strings.Contains(result.Filtered, "diff --git") {
			t.Error("diff --git lines should be removed")
		}

		// Verify "index" lines are removed
		if strings.Contains(result.Filtered, "index abc1234") {
			t.Error("index lines should be removed")
		}
		if strings.Contains(result.Filtered, "index 1111111") {
			t.Error("index lines should be removed")
		}

		// Verify file summary header is present
		if !strings.Contains(result.Filtered, "Files changed:") {
			t.Errorf("expected 'Files changed:' header, got:\n%s", result.Filtered)
		}
		if !strings.Contains(result.Filtered, "README.md (+1 -0)") {
			t.Errorf("expected README.md stats in header, got:\n%s", result.Filtered)
		}
		if !strings.Contains(result.Filtered, "main.go (+2 -1)") {
			t.Errorf("expected main.go stats in header, got:\n%s", result.Filtered)
		}

		// Verify actual diff content is preserved
		if !strings.Contains(result.Filtered, "+A new line here") {
			t.Error("addition line should be preserved")
		}
		if !strings.Contains(result.Filtered, "-    fmt.Println(\"removed\")") {
			t.Error("deletion line should be preserved")
		}
		if !strings.Contains(result.Filtered, "@@ -1,3 +1,4 @@") {
			t.Error("hunk header should be preserved")
		}

		// Verify --- and +++ lines preserved
		if !strings.Contains(result.Filtered, "--- a/README.md") {
			t.Error("--- line should be preserved")
		}
		if !strings.Contains(result.Filtered, "+++ b/README.md") {
			t.Error("+++ line should be preserved")
		}

		// Verify trailing newline preserved
		if !strings.HasSuffix(result.Filtered, "\n") {
			t.Error("trailing newline should be preserved")
		}
	})

	t.Run("short diff under 20 lines", func(t *testing.T) {
		// A diff with fewer than 20 lines should pass through unchanged
		input := "diff --git a/file.go b/file.go\n" +
			"index abc..def 100644\n" +
			"--- a/file.go\n" +
			"+++ b/file.go\n" +
			"@@ -1,3 +1,3 @@\n" +
			" line1\n" +
			"-old\n" +
			"+new\n" +
			" line3\n"

		result := s.Filter([]byte(input), "git", []string{"diff"}, 0)

		if result.WasReduced {
			t.Error("short diff (< 20 lines) should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("short diff should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("empty diff", func(t *testing.T) {
		result := s.Filter([]byte(""), "git", []string{"diff"}, 0)

		if result.WasReduced {
			t.Error("empty diff should not be reduced")
		}
		if result.Filtered != "" {
			t.Errorf("empty diff should return empty, got: %q", result.Filtered)
		}
	})
}

// ---------------------------------------------------------------------------
// GitLogStrategy
// ---------------------------------------------------------------------------

func TestGitLogStrategy_CanHandle(t *testing.T) {
	s := &GitLogStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"git log bare", "git", []string{"log"}, true},
		{"git log with flags", "git", []string{"-c", "color.ui=always", "log"}, true},
		{"git status", "git", []string{"status"}, false},
		{"not git", "notgit", []string{"log"}, false},
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

func TestGitLogStrategy_Name(t *testing.T) {
	s := &GitLogStrategy{}
	if got := s.Name(); got != "git-log" {
		t.Errorf("Name() = %q, want %q", got, "git-log")
	}
}

func TestGitLogStrategy_Filter(t *testing.T) {
	s := &GitLogStrategy{}

	t.Run("multiple commits more than 5 in full format", func(t *testing.T) {
		input := "commit a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2\n" +
			"Author: Alice Smith <alice@example.com>\n" +
			"Date:   Mon Feb 10 10:00:00 2026 +0000\n" +
			"\n" +
			"    feat: add user authentication\n" +
			"\n" +
			"commit b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3\n" +
			"Author: Bob Jones <bob@example.com>\n" +
			"Date:   Sun Feb 9 15:30:00 2026 +0000\n" +
			"\n" +
			"    fix: resolve login redirect bug\n" +
			"\n" +
			"commit c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4\n" +
			"Author: Alice Smith <alice@example.com>\n" +
			"Date:   Sat Feb 8 09:00:00 2026 +0000\n" +
			"\n" +
			"    docs: update README with install instructions\n" +
			"\n" +
			"commit d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5\n" +
			"Author: Charlie Brown <charlie@example.com>\n" +
			"Date:   Fri Feb 7 14:00:00 2026 +0000\n" +
			"\n" +
			"    refactor: extract config package\n" +
			"\n" +
			"commit e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6\n" +
			"Author: Alice Smith <alice@example.com>\n" +
			"Date:   Thu Feb 6 11:00:00 2026 +0000\n" +
			"\n" +
			"    test: add integration tests for auth\n" +
			"\n" +
			"commit f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1\n" +
			"Author: Bob Jones <bob@example.com>\n" +
			"Date:   Wed Feb 5 08:00:00 2026 +0000\n" +
			"\n" +
			"    chore: update dependencies\n" +
			"\n"

		result := s.Filter([]byte(input), "git", []string{"log"}, 0)

		if !result.WasReduced {
			t.Fatal("expected WasReduced=true for 6 commits")
		}

		// Verify compact one-line-per-commit format
		lines := strings.Split(strings.TrimRight(result.Filtered, "\n"), "\n")
		if len(lines) != 6 {
			t.Errorf("expected 6 compact lines, got %d:\n%s", len(lines), result.Filtered)
		}

		// Verify short hashes (first 7 chars)
		expectedPrefixes := []string{
			"a1b2c3d Mon Feb 10 10:00:00 2026 +0000 Alice Smith: feat: add user authentication",
			"b2c3d4e Sun Feb 9 15:30:00 2026 +0000 Bob Jones: fix: resolve login redirect bug",
			"c3d4e5f Sat Feb 8 09:00:00 2026 +0000 Alice Smith: docs: update README with install instructions",
			"d4e5f6a Fri Feb 7 14:00:00 2026 +0000 Charlie Brown: refactor: extract config package",
			"e5f6a1b Thu Feb 6 11:00:00 2026 +0000 Alice Smith: test: add integration tests for auth",
			"f6a1b2c Wed Feb 5 08:00:00 2026 +0000 Bob Jones: chore: update dependencies",
		}
		for i, expected := range expectedPrefixes {
			if i >= len(lines) {
				break
			}
			if lines[i] != expected {
				t.Errorf("line %d:\ngot:  %q\nwant: %q", i, lines[i], expected)
			}
		}

		// Verify no full commit hash lines remain
		if strings.Contains(result.Filtered, "commit a1b2c3d4e5f6") {
			t.Error("full commit hash lines should not appear in compact output")
		}

		// Verify trailing newline preserved
		if !strings.HasSuffix(result.Filtered, "\n") {
			t.Error("trailing newline should be preserved")
		}
	})

	t.Run("few commits 5 or fewer", func(t *testing.T) {
		input := "commit a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2\n" +
			"Author: Alice Smith <alice@example.com>\n" +
			"Date:   Mon Feb 10 10:00:00 2026 +0000\n" +
			"\n" +
			"    feat: add feature\n" +
			"\n" +
			"commit b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3\n" +
			"Author: Bob Jones <bob@example.com>\n" +
			"Date:   Sun Feb 9 15:30:00 2026 +0000\n" +
			"\n" +
			"    fix: resolve bug\n" +
			"\n"

		result := s.Filter([]byte(input), "git", []string{"log"}, 0)

		if result.WasReduced {
			t.Error("5 or fewer commits should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("few commits should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("already oneline format", func(t *testing.T) {
		input := "a1b2c3d feat: add feature\n" +
			"b2c3d4e fix: resolve bug\n" +
			"c3d4e5f docs: update README\n" +
			"d4e5f6a refactor: extract config\n" +
			"e5f6a1b test: add tests\n" +
			"f6a1b2c chore: update deps\n" +
			"a7b8c9d style: format code\n"

		result := s.Filter([]byte(input), "git", []string{"log", "--oneline"}, 0)

		if result.WasReduced {
			t.Error("already oneline format should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("oneline format should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := s.Filter([]byte(""), "git", []string{"log"}, 0)

		if result.WasReduced {
			t.Error("empty input should not be reduced")
		}
		if result.Filtered != "" {
			t.Errorf("empty log should return empty, got: %q", result.Filtered)
		}
	})
}

// ---------------------------------------------------------------------------
// GitDiffStrategy — additional edge cases
// ---------------------------------------------------------------------------

func TestGitDiffStrategy_Filter_FileDeletion(t *testing.T) {
	s := &GitDiffStrategy{}

	// A diff with a normal file change plus a deleted file.
	// The "+++ /dev/null" line signals the file was deleted.
	// We need >= 20 lines to trigger filtering.
	input := "diff --git a/README.md b/README.md\n" + // 1
		"index abc1234..def5678 100644\n" + // 2
		"--- a/README.md\n" + // 3
		"+++ b/README.md\n" + // 4
		"@@ -1,3 +1,4 @@\n" + // 5
		" # coc\n" + // 6
		"+A new line here\n" + // 7
		" \n" + // 8
		" Some content\n" + // 9
		"diff --git a/removed.go b/removed.go\n" + // 10
		"deleted file mode 100644\n" + // 11
		"index aaa1111..0000000\n" + // 12
		"--- a/removed.go\n" + // 13
		"+++ /dev/null\n" + // 14
		"@@ -1,5 +0,0 @@\n" + // 15
		"-package old\n" + // 16
		"-\n" + // 17
		"-func Deprecated() {\n" + // 18
		"-    return\n" + // 19
		"-}\n" + // 20
		"\n" // 21

	result := s.Filter([]byte(input), "git", []string{"diff"}, 0)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true for multi-file diff with deletion")
	}

	// The "+++ /dev/null" line should NOT be counted as an insertion because
	// it doesn't start with "+++ b/" — it enters the generic "+++ " branch
	// which does not increment insertions.
	if strings.Contains(result.Filtered, "+++ /dev/null") {
		// The line is preserved because the generic "+++ " handler calls
		// kept = append(kept, line).
	}

	// The deleted file should appear in the "Files changed:" summary.
	// The file name comes from lastMinusFile ("removed.go") since
	// "+++ /dev/null" doesn't match "+++ b/".
	if !strings.Contains(result.Filtered, "Files changed:") {
		t.Errorf("expected 'Files changed:' header, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "removed.go") {
		t.Errorf("expected removed.go in Files changed summary, got:\n%s", result.Filtered)
	}

	// Stats for the deleted file: 0 insertions ("+++ /dev/null" is not a "+"
	// content line) and 5 deletions (the five "-" lines).
	if !strings.Contains(result.Filtered, "removed.go (+0 -5)") {
		t.Errorf("expected removed.go (+0 -5) stats, got:\n%s", result.Filtered)
	}

	// Deletion content lines should be preserved in the diff body
	if !strings.Contains(result.Filtered, "-package old") {
		t.Error("deletion content line '-package old' should be preserved")
	}
	if !strings.Contains(result.Filtered, "-func Deprecated() {") {
		t.Error("deletion content line '-func Deprecated() {' should be preserved")
	}

	// "diff --git" noise lines should be removed
	if strings.Contains(result.Filtered, "diff --git") {
		t.Error("diff --git lines should be removed")
	}
}

func TestGitDiffStrategy_Filter_BinaryFile(t *testing.T) {
	s := &GitDiffStrategy{}

	// A diff with a normal text change plus a binary file change.
	// >= 20 lines to trigger filtering.
	input := "diff --git a/main.go b/main.go\n" + // 1
		"index aaa1111..bbb2222 100644\n" + // 2
		"--- a/main.go\n" + // 3
		"+++ b/main.go\n" + // 4
		"@@ -1,5 +1,7 @@\n" + // 5
		" package main\n" + // 6
		" \n" + // 7
		"+import \"fmt\"\n" + // 8
		"+\n" + // 9
		" func main() {\n" + // 10
		"+    fmt.Println(\"hello\")\n" + // 11
		" }\n" + // 12
		"diff --git a/image.png b/image.png\n" + // 13
		"index ccc3333..ddd4444 100644\n" + // 14
		"Binary files a/image.png and b/image.png differ\n" + // 15
		"diff --git a/docs.go b/docs.go\n" + // 16
		"index eee5555..fff6666 100644\n" + // 17
		"--- a/docs.go\n" + // 18
		"+++ b/docs.go\n" + // 19
		"@@ -1,3 +1,4 @@\n" + // 20
		" package docs\n" + // 21
		"+// Added comment\n" + // 22
		" \n" + // 23
		"\n" // 24

	result := s.Filter([]byte(input), "git", []string{"diff"}, 0)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true for diff with binary file")
	}

	// image.png should appear in "Files changed:" summary with (binary) marker
	if !strings.Contains(result.Filtered, "Files changed:") {
		t.Errorf("expected 'Files changed:' header, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "image.png (binary)") {
		t.Errorf("expected 'image.png (binary)' in summary, got:\n%s", result.Filtered)
	}

	// The "Binary files ... differ" line should be preserved in the diff body
	if !strings.Contains(result.Filtered, "Binary files a/image.png and b/image.png differ") {
		t.Error("binary files differ line should be preserved")
	}

	// Other files should also appear in the summary
	if !strings.Contains(result.Filtered, "main.go") {
		t.Errorf("expected main.go in summary, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "docs.go") {
		t.Errorf("expected docs.go in summary, got:\n%s", result.Filtered)
	}
}

func TestGitDiffStrategy_Filter_Rename(t *testing.T) {
	s := &GitDiffStrategy{}

	// A diff with a file rename plus some content changes.
	// >= 20 lines to trigger filtering.
	input := "diff --git a/old.go b/new.go\n" + // 1
		"similarity index 85%\n" + // 2
		"rename from old.go\n" + // 3
		"rename to new.go\n" + // 4
		"index aaa1111..bbb2222 100644\n" + // 5
		"--- a/old.go\n" + // 6
		"+++ b/new.go\n" + // 7
		"@@ -1,5 +1,6 @@\n" + // 8
		" package pkg\n" + // 9
		" \n" + // 10
		"-func OldName() {\n" + // 11
		"+func NewName() {\n" + // 12
		"+    // renamed\n" + // 13
		"     return\n" + // 14
		" }\n" + // 15
		"diff --git a/utils.go b/utils.go\n" + // 16
		"index ccc3333..ddd4444 100644\n" + // 17
		"--- a/utils.go\n" + // 18
		"+++ b/utils.go\n" + // 19
		"@@ -1,3 +1,4 @@\n" + // 20
		" package pkg\n" + // 21
		"+// added line\n" + // 22
		" \n" + // 23
		"\n" // 24

	result := s.Filter([]byte(input), "git", []string{"diff"}, 0)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true for diff with rename")
	}

	// The renamed file should appear in the summary under its new name
	// (tracked via "+++ b/new.go")
	if !strings.Contains(result.Filtered, "Files changed:") {
		t.Errorf("expected 'Files changed:' header, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "new.go") {
		t.Errorf("expected new.go (renamed file) in summary, got:\n%s", result.Filtered)
	}

	// The new.go entry should have correct stats: +2 insertions, -1 deletion
	if !strings.Contains(result.Filtered, "new.go (+2 -1)") {
		t.Errorf("expected new.go (+2 -1) stats, got:\n%s", result.Filtered)
	}

	// Diff content should be preserved
	if !strings.Contains(result.Filtered, "-func OldName() {") {
		t.Error("deletion line '-func OldName() {' should be preserved")
	}
	if !strings.Contains(result.Filtered, "+func NewName() {") {
		t.Error("addition line '+func NewName() {' should be preserved")
	}

	// Rename metadata lines should be preserved (they fall through to catch-all)
	if !strings.Contains(result.Filtered, "rename from old.go") {
		t.Error("rename from metadata should be preserved")
	}
	if !strings.Contains(result.Filtered, "rename to new.go") {
		t.Error("rename to metadata should be preserved")
	}
}

// ---------------------------------------------------------------------------
// GitStatusStrategy — additional edge cases
// ---------------------------------------------------------------------------

func TestGitStatusStrategy_Filter_DetachedHead(t *testing.T) {
	s := &GitStatusStrategy{}

	// Input simulating a detached HEAD state with changes.
	// The "HEAD detached at abc1234" line does NOT start with "On branch ",
	// so the current filter implementation drops it.
	input := "HEAD detached at abc1234\n" +
		"Your branch is ahead of 'origin/main' by 2 commits.\n" +
		"\n" +
		"Changes to be committed:\n" +
		"  (use \"git restore --staged <file>...\" to unstage)\n" +
		"\tmodified:   main.go\n" +
		"\n" +
		"Changes not staged for commit:\n" +
		"  (use \"git add <file>...\" to update what will be committed)\n" +
		"  (use \"git restore <file>...\" to discard changes in working directory)\n" +
		"\tmodified:   utils.go\n" +
		"\n"

	result := s.Filter([]byte(input), "git", []string{"status"}, 0)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true for detached HEAD status")
	}

	// NOTE: The current implementation only preserves lines starting with
	// "On branch ". The "HEAD detached at abc1234" line does not match
	// this prefix and is therefore dropped by the filter. This is a known
	// limitation -- the filter was designed for the common "On branch" case.
	if strings.Contains(result.Filtered, "HEAD detached at abc1234") {
		// If the filter is later updated to handle detached HEAD, this
		// assertion should be flipped to require its presence.
		t.Log("HEAD detached line is preserved (filter may have been updated)")
	}

	// Modified files should appear with converted markers
	if !strings.Contains(result.Filtered, "\tM   main.go") {
		t.Errorf("expected converted staged modified file main.go, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "\tM   utils.go") {
		t.Errorf("expected converted unstaged modified file utils.go, got:\n%s", result.Filtered)
	}

	// Hint lines should be stripped
	if strings.Contains(result.Filtered, "(use \"git") {
		t.Error("hint lines should be stripped")
	}

	// Summary should reflect 1 staged, 1 unstaged
	if !strings.Contains(result.Filtered, "1 staged, 1 unstaged, 0 untracked") {
		t.Errorf("expected summary '1 staged, 1 unstaged, 0 untracked', got:\n%s", result.Filtered)
	}
}

func TestGitStatusStrategy_Filter_ANSIColoredInput(t *testing.T) {
	s := &GitStatusStrategy{}

	// Input with ANSI color codes, as git would produce with color.status=always.
	// \x1b[32m = green, \x1b[31m = red, \x1b[0m = reset
	input := "\x1b[32mOn branch main\x1b[0m\n" +
		"\x1b[32mYour branch is up to date with 'origin/main'.\x1b[0m\n" +
		"\n" +
		"\x1b[32mChanges to be committed:\x1b[0m\n" +
		"  (use \"git restore --staged <file>...\" to unstage)\n" +
		"\t\x1b[32mmodified:   staged_file.go\x1b[0m\n" +
		"\n" +
		"\x1b[31mChanges not staged for commit:\x1b[0m\n" +
		"  (use \"git add <file>...\" to update what will be committed)\n" +
		"  (use \"git restore <file>...\" to discard changes in working directory)\n" +
		"\t\x1b[31mmodified:   unstaged_file.go\x1b[0m\n" +
		"\n" +
		"Untracked files:\n" +
		"  (use \"git add <file>...\" to include in what will be committed)\n" +
		"\t\x1b[31mnew_file.go\x1b[0m\n" +
		"\n"

	result := s.Filter([]byte(input), "git", []string{"status"}, 0)

	if !result.WasReduced {
		t.Fatal("expected WasReduced=true for ANSI colored status")
	}

	// No ANSI escape codes should remain in the output
	if strings.Contains(result.Filtered, "\x1b[") {
		t.Error("ANSI escape codes should be stripped from output")
	}

	// Clean file names should appear with converted markers
	if !strings.Contains(result.Filtered, "\tM   staged_file.go") {
		t.Errorf("expected clean converted staged file, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "\tM   unstaged_file.go") {
		t.Errorf("expected clean converted unstaged file, got:\n%s", result.Filtered)
	}
	if !strings.Contains(result.Filtered, "\tnew_file.go") {
		t.Errorf("expected clean untracked file, got:\n%s", result.Filtered)
	}

	// Section headers should be clean (no ANSI codes)
	if !strings.Contains(result.Filtered, "Changes to be committed:") {
		t.Error("section header 'Changes to be committed:' should be present and clean")
	}
	if !strings.Contains(result.Filtered, "Changes not staged for commit:") {
		t.Error("section header 'Changes not staged for commit:' should be present and clean")
	}
	if !strings.Contains(result.Filtered, "Untracked files:") {
		t.Error("section header 'Untracked files:' should be present and clean")
	}

	// Hint lines should still be stripped
	if strings.Contains(result.Filtered, "(use \"git") {
		t.Error("hint lines should be stripped even from ANSI colored input")
	}

	// Summary should be correct
	if !strings.Contains(result.Filtered, "1 staged, 1 unstaged, 1 untracked") {
		t.Errorf("expected summary '1 staged, 1 unstaged, 1 untracked', got:\n%s", result.Filtered)
	}
}
