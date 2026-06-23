package source

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestClassifyLocalPath(t *testing.T) {
	dir := t.TempDir()
	spec, err := Classify(dir)
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if spec.Type != TypeLocal {
		t.Fatalf("type = %q, want %q", spec.Type, TypeLocal)
	}
}

func TestClassifyRemoteInputs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cloneURL string
		provider string
	}{
		{
			name:     "github https",
			input:    "https://github.com/acme/orders.git",
			cloneURL: "https://github.com/acme/orders.git",
			provider: ProviderGitHub,
		},
		{
			name:     "gitlab https",
			input:    "https://gitlab.com/acme/platform/orders.git",
			cloneURL: "https://gitlab.com/acme/platform/orders.git",
			provider: ProviderGitLab,
		},
		{
			name:     "ssh scp",
			input:    "git@github.com:acme/orders.git",
			cloneURL: "git@github.com:acme/orders.git",
			provider: ProviderGitHub,
		},
		{
			name:     "github shorthand",
			input:    "github.com/acme/orders",
			cloneURL: "https://github.com/acme/orders.git",
			provider: ProviderGitHub,
		},
		{
			name:     "gitlab shorthand",
			input:    "gitlab.com/acme/platform/orders",
			cloneURL: "https://gitlab.com/acme/platform/orders.git",
			provider: ProviderGitLab,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := Classify(tt.input)
			if err != nil {
				t.Fatalf("Classify returned error: %v", err)
			}
			if spec.Type != TypeGit || spec.CloneURL != tt.cloneURL || spec.Provider != tt.provider {
				t.Fatalf("spec = %#v, want type=%s cloneURL=%s provider=%s", spec, TypeGit, tt.cloneURL, tt.provider)
			}
		})
	}
}

func TestClassifyRedactsCredentialedURL(t *testing.T) {
	spec, err := Classify("https://token@example.com/acme/orders.git")
	if err != nil {
		t.Fatalf("Classify returned error: %v", err)
	}
	if strings.Contains(spec.Original, "token") {
		t.Fatalf("original was not redacted: %s", spec.Original)
	}
	if spec.CloneURL != "https://token@example.com/acme/orders.git" {
		t.Fatalf("clone URL should preserve credentials for git, got %s", spec.CloneURL)
	}
}

func TestBuildGitCommands(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want [][]string
	}{
		{
			name: "default shallow clone",
			want: [][]string{{"clone", "--depth", "1", "--single-branch", "https://github.com/acme/orders.git", "/tmp/orders"}},
		},
		{
			name: "branch ref",
			ref:  "main",
			want: [][]string{{"clone", "--depth", "1", "--single-branch", "--branch", "main", "https://github.com/acme/orders.git", "/tmp/orders"}},
		},
		{
			name: "commit sha",
			ref:  "0123456789abcdef",
			want: [][]string{
				{"clone", "--depth", "1", "--single-branch", "https://github.com/acme/orders.git", "/tmp/orders"},
				{"fetch", "--depth", "1", "origin", "0123456789abcdef"},
				{"checkout", "--detach", "0123456789abcdef"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands := buildGitCommands("https://github.com/acme/orders.git", tt.ref, "/tmp/orders")
			var got [][]string
			for _, command := range commands {
				got = append(got, command.Args)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("commands = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestResolveRemoteWithFakeRunner(t *testing.T) {
	runner := &recordingRunner{output: "abcdef123456\n"}
	workdir := t.TempDir()
	resolved, err := Resolve(context.Background(), Options{
		Repo:         "github.com/acme/orders",
		Ref:          "main",
		Workdir:      workdir,
		KeepWorktree: true,
		Runner:       runner,
		Stdin:        strings.NewReader(""),
		Stdout:       io.Discard,
		Stderr:       io.Discard,
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if resolved.Source.Type != TypeGit || resolved.Source.Provider != ProviderGitHub {
		t.Fatalf("source = %#v, want GitHub git source", resolved.Source)
	}
	if resolved.Cleanup != nil {
		t.Fatalf("cleanup should be nil when worktree is kept")
	}
	if got, want := runner.commands[0], []string{"clone", "--depth", "1", "--single-branch", "--branch", "main", "https://github.com/acme/orders.git", resolved.Path}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clone command = %#v, want %#v", got, want)
	}
}

func TestResolveRemoteMissingGit(t *testing.T) {
	_, err := Resolve(context.Background(), Options{
		Repo:   "https://github.com/acme/orders.git",
		Runner: &recordingRunner{lookPathErr: errors.New("missing")},
	})
	if err == nil || !strings.Contains(err.Error(), "require git") {
		t.Fatalf("error = %v, want missing git error", err)
	}
}

func TestResolveRemoteFromLocalBareRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	sourceRepo := t.TempDir()
	runGit(t, sourceRepo, "init")
	runGit(t, sourceRepo, "checkout", "-b", "main")
	writeFile(t, filepath.Join(sourceRepo, "src/main/java/com/example/App.java"), "class App {}\n")
	runGit(t, sourceRepo, "add", ".")
	runGit(t, sourceRepo, "commit", "-m", "initial")
	runGit(t, sourceRepo, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(sourceRepo, "feature.txt"), "feature\n")
	runGit(t, sourceRepo, "add", ".")
	runGit(t, sourceRepo, "commit", "-m", "feature")

	bareRoot := t.TempDir()
	bareRepo := filepath.Join(bareRoot, "orders.git")
	runGit(t, "", "clone", "--bare", sourceRepo, bareRepo)

	resolved, err := Resolve(context.Background(), Options{
		Repo:         "file://" + filepath.ToSlash(bareRepo),
		Ref:          "feature",
		Workdir:      t.TempDir(),
		KeepWorktree: true,
		Stdin:        strings.NewReader(""),
		Stdout:       io.Discard,
		Stderr:       io.Discard,
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if resolved.Source.Type != TypeGit || resolved.Source.ResolvedRef == "" {
		t.Fatalf("source = %#v, want resolved git source", resolved.Source)
	}
	if _, err := os.Stat(filepath.Join(resolved.Path, "feature.txt")); err != nil {
		t.Fatalf("feature branch file was not checked out: %v", err)
	}
}

type recordingRunner struct {
	commands    [][]string
	output      string
	lookPathErr error
}

func (r *recordingRunner) LookPath(name string) (string, error) {
	if r.lookPathErr != nil {
		return "", r.lookPathErr
	}
	return "/usr/bin/" + name, nil
}

func (r *recordingRunner) Run(ctx context.Context, dir string, stdin io.Reader, stdout io.Writer, stderr io.Writer, args ...string) error {
	r.commands = append(r.commands, append([]string(nil), args...))
	return nil
}

func (r *recordingRunner) Output(ctx context.Context, dir string, args ...string) (string, error) {
	return r.output, nil
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=gco11y-size test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=gco11y-size test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
