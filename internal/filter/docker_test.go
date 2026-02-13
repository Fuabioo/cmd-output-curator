package filter

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// DockerBuildStrategy
// ---------------------------------------------------------------------------

func TestDockerBuildStrategy_CanHandle(t *testing.T) {
	s := &DockerBuildStrategy{}

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{"docker build", "docker", []string{"build", "."}, true},
		{"docker -H host build", "docker", []string{"-H", "tcp://host:2375", "build", "."}, true},
		{"docker buildx build", "docker", []string{"buildx", "build", "."}, true},
		{"docker compose build", "docker", []string{"compose", "build"}, true},
		{"docker buildx --builder foo build", "docker", []string{"buildx", "--builder", "mybuilder", "build", "."}, true},
		{"docker compose -f file build", "docker", []string{"compose", "-f", "docker-compose.yml", "build"}, true},
		{"docker run", "docker", []string{"run", "alpine"}, false},
		{"docker ps", "docker", []string{"ps"}, false},
		{"podman build", "podman", []string{"build", "."}, false},
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

func TestDockerBuildStrategy_Name(t *testing.T) {
	s := &DockerBuildStrategy{}
	if got := s.Name(); got != "docker-build" {
		t.Errorf("Name() = %q, want %q", got, "docker-build")
	}
}

func TestDockerBuildStrategy_Filter(t *testing.T) {
	s := &DockerBuildStrategy{}

	t.Run("successful legacy build", func(t *testing.T) {
		input := "Sending build context to Docker daemon  2.048kB\n" +
			"Step 1/3 : FROM alpine:3.18\n" +
			" ---> 8ca4688f4f35\n" +
			" ---> Using cache\n" +
			"Step 2/3 : COPY app /app\n" +
			" ---> 1a2b3c4d5e6f\n" +
			"Removing intermediate container 9f8e7d6c5b4a\n" +
			"Step 3/3 : RUN chmod +x /app\n" +
			" ---> Running in abc123def456\n" +
			" ---> 2b3c4d5e6f7a\n" +
			"Removing intermediate container abc123def456\n" +
			"Successfully built 2b3c4d5e6f7a\n" +
			"Successfully tagged myapp:latest\n" +
			"some extra line 1\n" +
			"some extra line 2\n"

		result := s.Filter([]byte(input), "docker", []string{"build", "."}, 0)

		// Step lines should be kept
		if !strings.Contains(result.Filtered, "Step 1/3") {
			t.Error("Step 1/3 line should be preserved")
		}
		if !strings.Contains(result.Filtered, "Step 2/3") {
			t.Error("Step 2/3 line should be preserved")
		}
		if !strings.Contains(result.Filtered, "Step 3/3") {
			t.Error("Step 3/3 line should be preserved")
		}

		// Successfully lines should be kept
		if !strings.Contains(result.Filtered, "Successfully built") {
			t.Error("Successfully built line should be preserved")
		}
		if !strings.Contains(result.Filtered, "Successfully tagged") {
			t.Error("Successfully tagged line should be preserved")
		}

		// Legacy noise should be stripped
		if strings.Contains(result.Filtered, "Sending build context") {
			t.Error("Sending build context line should be stripped")
		}
		if strings.Contains(result.Filtered, "Removing intermediate container") {
			t.Error("Removing intermediate container lines should be stripped")
		}
		if strings.Contains(result.Filtered, "---> Using cache") {
			t.Error("---> Using cache lines should be stripped")
		}
		// Intermediate container hash lines should be stripped
		if strings.Contains(result.Filtered, "---> 8ca4688f4f35") {
			t.Error("intermediate hash lines should be stripped")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true since noise was stripped")
		}
	})

	t.Run("successful buildkit build", func(t *testing.T) {
		input := "#1 [internal] load build definition from Dockerfile\n" +
			"#1 sha256:abc123def456 0B / 0B\n" +
			"#1 transferring dockerfile: 234B\n" +
			"#2 [1/3] FROM docker.io/library/alpine:3.18\n" +
			"#2 CACHED\n" +
			"#3 [2/3] COPY app /app\n" +
			"#3 sha256:def456abc789 0B / 1.2kB\n" +
			"#3 DONE 0.1s\n" +
			"#4 [3/3] RUN chmod +x /app\n" +
			"#4 sha256:789abc123def 0B / 0B\n" +
			"#4 0.234 some output\n" +
			"#4 DONE 0.5s\n" +
			"#5 exporting to image\n" +
			"#5 exporting layers 0.1s\n" +
			"#5 DONE 0.2s\n"

		result := s.Filter([]byte(input), "docker", []string{"buildx", "build", "."}, 0)

		// DONE and CACHED lines should be kept
		if !strings.Contains(result.Filtered, "#2 CACHED") {
			t.Error("#2 CACHED line should be preserved")
		}
		if !strings.Contains(result.Filtered, "#3 DONE 0.1s") {
			t.Error("#3 DONE line should be preserved")
		}
		if !strings.Contains(result.Filtered, "#4 DONE 0.5s") {
			t.Error("#4 DONE line should be preserved")
		}
		if !strings.Contains(result.Filtered, "#5 DONE 0.2s") {
			t.Error("#5 DONE line should be preserved")
		}

		// sha256 lines should be stripped
		if strings.Contains(result.Filtered, "sha256:abc123def456") {
			t.Error("sha256 lines should be stripped")
		}
		if strings.Contains(result.Filtered, "sha256:def456abc789") {
			t.Error("sha256 lines should be stripped")
		}

		if !result.WasReduced {
			t.Error("expected WasReduced=true since BuildKit noise was stripped")
		}
	})

	t.Run("failed build", func(t *testing.T) {
		input := "Step 1/3 : FROM alpine:3.18\n" +
			" ---> 8ca4688f4f35\n" +
			"Step 2/3 : COPY nonexistent /app\n" +
			"COPY failed: file not found in build context\n" +
			"  --> Dockerfile:5\n" +
			"error building image: COPY failed\n" +
			"some context line 1\n" +
			"some context line 2\n" +
			"some context line 3\n" +
			"some context line 4\n" +
			"some context line 5\n" +
			"some context line 6\n" +
			"some context line 7\n" +
			"some context line 8\n" +
			"The command returned a non-zero exit code\n"

		result := s.Filter([]byte(input), "docker", []string{"build", "."}, 1)

		// Error lines should be kept
		if !strings.Contains(result.Filtered, "COPY failed: file not found") {
			t.Error("error line should be preserved")
		}
		if !strings.Contains(result.Filtered, "error building image") {
			t.Error("error building image line should be preserved")
		}
		// Dockerfile pointer should be kept
		if !strings.Contains(result.Filtered, "--> Dockerfile:5") {
			t.Error("--> pointer line should be preserved")
		}
	})

	t.Run("small output", func(t *testing.T) {
		input := "Step 1/1 : FROM alpine\n" +
			"Successfully built abc123\n"

		result := s.Filter([]byte(input), "docker", []string{"build", "."}, 0)

		if result.WasReduced {
			t.Error("small output (< 15 lines) should not be reduced")
		}
		if result.Filtered != input {
			t.Errorf("small output should pass through unchanged\ngot:  %q\nwant: %q", result.Filtered, input)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := s.Filter([]byte(""), "docker", []string{"build", "."}, 0)

		if result.WasReduced {
			t.Error("empty input should not be reduced")
		}
		if result.Filtered != "" {
			t.Errorf("empty input should produce empty output, got: %q", result.Filtered)
		}
	})
}
