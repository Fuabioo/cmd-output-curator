package filter

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ProgressStripStrategy
// ---------------------------------------------------------------------------

func TestProgressStripStrategy_CanHandle(t *testing.T) {
	s := &ProgressStripStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"npm install", "npm", []string{"install"}, true},
		{"npm ci", "npm", []string{"ci"}, true},
		{"docker pull", "docker", []string{"pull", "alpine"}, true},
		{"pip install", "pip", []string{"install", "requests"}, true},
		{"pip3 install", "pip3", []string{"install", "flask"}, true},
		{"yarn add", "yarn", []string{"add", "lodash"}, true},
		{"docker -H host pull", "docker", []string{"-H", "tcp://host:2375", "pull", "alpine"}, true},
		{"npm --prefix path install", "npm", []string{"--prefix", "/some/path", "install"}, true},
		{"pip --target dir install", "pip", []string{"--target", "/some/dir", "install", "requests"}, true},
		{"yarn --cwd dir add", "yarn", []string{"--cwd", "/some/dir", "add", "lodash"}, true},
		{"docker push", "docker", []string{"push", "myimage"}, true},
		{"yarn install", "yarn", []string{"install"}, true},
		{"npm test", "npm", []string{"test"}, false},
		{"npm run", "npm", []string{"run", "dev"}, false},
		{"go install", "go", []string{"install"}, false},
		{"docker build", "docker", []string{"build", "."}, false},
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

func TestProgressStripStrategy_Name(t *testing.T) {
	s := &ProgressStripStrategy{}
	if got := s.Name(); got != "progress-strip" {
		t.Errorf("Name() = %q, want %q", got, "progress-strip")
	}
}

func TestProgressStripStrategy_Filter(t *testing.T) {
	s := &ProgressStripStrategy{}

	t.Run("npm install with progress", func(t *testing.T) {
		input := "npm WARN deprecated mkdirp@0.5.1: Legacy versions\n" +
			"npm WARN deprecated request@2.88.2: request has been deprecated\n" +
			"⠋ reify:lodash: timing reifyNode\n" +
			"⠙ reify:express: timing reifyNode\n" +
			"⠹ reify:body-parser: timing reifyNode\n" +
			"⠸ reify:cookie: timing reifyNode\n" +
			"⠼ reify:debug: timing reifyNode\n" +
			"⠴ reify:ms: timing reifyNode\n" +
			"added 50 packages in 3.456s\n" +
			"some final line\n" +
			"another final line\n"

		result := s.Filter([]byte(input), "npm", []string{"install"}, 0)

		// WARN lines should be kept
		if !strings.Contains(result.Filtered, "npm WARN deprecated mkdirp") {
			t.Error("npm WARN lines should be preserved")
		}
		if !strings.Contains(result.Filtered, "npm WARN deprecated request") {
			t.Error("npm WARN lines should be preserved")
		}
		// Summary should be kept
		if !strings.Contains(result.Filtered, "added 50 packages") {
			t.Error("added packages summary should be preserved")
		}
		// Spinner lines should be stripped
		if strings.Contains(result.Filtered, "reify:lodash") {
			t.Error("spinner/progress lines should be stripped")
		}
		if strings.Contains(result.Filtered, "reify:express") {
			t.Error("spinner/progress lines should be stripped")
		}

		// Should have progress header
		if !strings.Contains(result.Filtered, "Progress output stripped") {
			t.Error("expected progress stripped header")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true since progress was stripped")
		}
	})

	t.Run("docker pull with layer progress", func(t *testing.T) {
		input := "Using default tag: latest\n" +
			"latest: Pulling from library/alpine\n" +
			"abc123: Pulling fs layer\n" +
			"def456: Pulling fs layer\n" +
			"abc123: Downloading [==>                  ] 5MB/50MB\n" +
			"abc123: Downloading [========>            ] 20MB/50MB\n" +
			"def456: Waiting\n" +
			"abc123: Pull complete\n" +
			"def456: Extracting [=>                   ] 1MB/25MB\n" +
			"def456: Pull complete\n" +
			"ghi789: Already exists\n" +
			"Digest: sha256:abcdef123456\n" +
			"Status: Downloaded newer image\n"

		result := s.Filter([]byte(input), "docker", []string{"pull", "alpine"}, 0)

		// Pull complete and Already exists should be kept
		if !strings.Contains(result.Filtered, "abc123: Pull complete") {
			t.Error("Pull complete lines should be preserved")
		}
		if !strings.Contains(result.Filtered, "def456: Pull complete") {
			t.Error("Pull complete lines should be preserved")
		}
		if !strings.Contains(result.Filtered, "ghi789: Already exists") {
			t.Error("Already exists lines should be preserved")
		}

		// Progress lines should be stripped
		if strings.Contains(result.Filtered, "Pulling fs layer") {
			t.Error("Pulling fs layer lines should be stripped")
		}
		if strings.Contains(result.Filtered, "Downloading") {
			t.Error("Downloading progress lines should be stripped")
		}
		if strings.Contains(result.Filtered, "Extracting") {
			t.Error("Extracting progress lines should be stripped")
		}
		if strings.Contains(result.Filtered, "Waiting") {
			t.Error("Waiting lines should be stripped")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true since layer progress was stripped")
		}
	})

	t.Run("carriage return cleanup", func(t *testing.T) {
		input := "Downloading package 1...\rDownloading package 1... 50%\rDownloading package 1... done\n" +
			"Downloading package 2...\rDownloading package 2... 50%\rDownloading package 2... done\n" +
			"Downloading package 3...\rDownloading package 3... done\n" +
			"Downloading package 4...\rDownloading package 4... done\n" +
			"Downloading package 5...\rDownloading package 5... done\n" +
			"Downloading package 6...\rDownloading package 6... done\n" +
			"Downloading package 7...\rDownloading package 7... done\n" +
			"Downloading package 8...\rDownloading package 8... done\n" +
			"Installation complete\n" +
			"Summary: 8 packages installed\n"

		result := s.Filter([]byte(input), "pip", []string{"install", "requests"}, 0)

		// Should keep only content after last \r
		if strings.Contains(result.Filtered, "50%") {
			t.Error("intermediate carriage return content should be cleaned up")
		}
		if !strings.Contains(result.Filtered, "done") {
			t.Error("final content after last \\r should be preserved")
		}
		if !strings.Contains(result.Filtered, "Installation complete") {
			t.Error("non-CR lines should be preserved")
		}
	})

	t.Run("small output", func(t *testing.T) {
		input := "npm WARN deprecated pkg@1.0.0: old\n" +
			"added 5 packages in 1.2s\n"

		result := s.Filter([]byte(input), "npm", []string{"install"}, 0)

		if result.WasReduced {
			t.Error("small output (< 10 lines) should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("small output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := s.Filter([]byte(""), "npm", []string{"install"}, 0)

		if result.WasReduced {
			t.Error("empty input should not be reduced")
		}
		if result.Filtered != "" {
			t.Errorf("empty input should produce empty output, got: %q", result.Filtered)
		}
	})
}
