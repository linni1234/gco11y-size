package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

const (
	TypeLocal = "local"
	TypeGit   = "git"

	ProviderGitHub  = "github"
	ProviderGitLab  = "gitlab"
	ProviderGeneric = "generic-git"
)

type Options struct {
	Repo         string
	Ref          string
	Workdir      string
	KeepWorktree bool
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Runner       GitRunner
}

type Resolved struct {
	Path     string
	Source   model.SourceMetadata
	Cleanup  func() error
	tempRoot string
}

type Spec struct {
	Type     string
	Provider string
	CloneURL string
	Original string
}

type GitRunner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, dir string, stdin io.Reader, stdout io.Writer, stderr io.Writer, args ...string) error
	Output(ctx context.Context, dir string, args ...string) (string, error)
}

type execGitRunner struct{}

type gitCommand struct {
	Dir  string
	Args []string
}

var commitSHARE = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
var scpStyleRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+@([A-Za-z0-9_.-]+):(.+)$`)

func Resolve(ctx context.Context, opts Options) (Resolved, error) {
	spec, err := Classify(opts.Repo)
	if err != nil {
		return Resolved{}, err
	}
	if spec.Type == TypeLocal {
		abs, err := filepath.Abs(opts.Repo)
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{
			Path: abs,
			Source: model.SourceMetadata{
				Original:          sanitizeForReport(opts.Repo),
				Type:              TypeLocal,
				ResolvedPath:      abs,
				RequestedRef:      opts.Ref,
				WorktreeTemporary: false,
				WorktreeRetained:  true,
			},
		}, nil
	}
	return resolveGit(ctx, spec, opts)
}

func Classify(repo string) (Spec, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return Spec{}, fmt.Errorf("--repo cannot be empty")
	}
	if info, err := os.Stat(repo); err == nil {
		if !info.IsDir() {
			return Spec{}, fmt.Errorf("local repository path %q exists but is not a directory", repo)
		}
		return Spec{Type: TypeLocal, Original: repo}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return Spec{}, fmt.Errorf("could not inspect repository path %q: %v", repo, err)
	}

	if cloneURL, provider, ok := normalizeRemote(repo); ok {
		return Spec{
			Type:     TypeGit,
			Provider: provider,
			CloneURL: cloneURL,
			Original: sanitizeForReport(repo),
		}, nil
	}
	return Spec{}, fmt.Errorf("local repository path %q does not exist and it was not recognized as a Git URL", repo)
}

func normalizeRemote(raw string) (cloneURL string, provider string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if matches := scpStyleRE.FindStringSubmatch(raw); len(matches) == 3 {
		return raw, providerForHost(matches[1]), true
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Scheme != "" {
		switch parsed.Scheme {
		case "https", "http", "ssh", "git", "file":
			return raw, providerForHost(parsed.Hostname()), true
		default:
			return "", "", false
		}
	}
	if strings.HasPrefix(raw, "github.com/") || strings.HasPrefix(raw, "gitlab.com/") {
		parts := strings.Split(raw, "/")
		if len(parts) < 3 || parts[1] == "" || parts[2] == "" {
			return "", "", false
		}
		normalized := "https://" + raw
		if !strings.HasSuffix(normalized, ".git") {
			normalized += ".git"
		}
		return normalized, providerForHost(parts[0]), true
	}
	return "", "", false
}

func providerForHost(host string) string {
	host = strings.ToLower(host)
	switch {
	case host == "github.com" || strings.HasSuffix(host, ".github.com"):
		return ProviderGitHub
	case host == "gitlab.com" || strings.HasSuffix(host, ".gitlab.com"):
		return ProviderGitLab
	default:
		return ProviderGeneric
	}
}

func resolveGit(ctx context.Context, spec Spec, opts Options) (Resolved, error) {
	runner := opts.Runner
	if runner == nil {
		runner = execGitRunner{}
	}
	if _, err := runner.LookPath("git"); err != nil {
		return Resolved{}, fmt.Errorf("remote repository scans require git to be installed and available on PATH")
	}

	parent, err := cloneParent(opts.Workdir)
	if err != nil {
		return Resolved{}, err
	}
	tempRoot, err := os.MkdirTemp(parent, "gco11y-size-*")
	if err != nil {
		return Resolved{}, fmt.Errorf("could not create temporary worktree directory: %v", err)
	}
	dest := filepath.Join(tempRoot, repoDirectoryName(spec.CloneURL))
	cleanup := func() error {
		return os.RemoveAll(tempRoot)
	}

	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	commands := buildGitCommands(spec.CloneURL, opts.Ref, dest)
	for _, command := range commands {
		if err := runner.Run(ctx, command.Dir, stdin, stdout, stderr, command.Args...); err != nil {
			_ = cleanup()
			return Resolved{}, gitCommandError(command.Args, err)
		}
	}
	resolvedRef, err := runner.Output(ctx, dest, "rev-parse", "HEAD")
	if err != nil {
		_ = cleanup()
		return Resolved{}, fmt.Errorf("cloned repository but could not resolve HEAD: %v", err)
	}
	resolvedRef = strings.TrimSpace(resolvedRef)
	absDest, err := filepath.Abs(dest)
	if err != nil {
		_ = cleanup()
		return Resolved{}, err
	}
	if opts.KeepWorktree {
		cleanup = nil
	}
	return Resolved{
		Path:     absDest,
		Cleanup:  cleanup,
		tempRoot: tempRoot,
		Source: model.SourceMetadata{
			Original:          spec.Original,
			Type:              TypeGit,
			Provider:          spec.Provider,
			CloneURL:          sanitizeForReport(spec.CloneURL),
			ResolvedPath:      absDest,
			RequestedRef:      opts.Ref,
			ResolvedRef:       resolvedRef,
			WorktreeTemporary: true,
			WorktreeRetained:  opts.KeepWorktree,
		},
	}, nil
}

func cloneParent(workdir string) (string, error) {
	if strings.TrimSpace(workdir) == "" {
		return os.TempDir(), nil
	}
	abs, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", fmt.Errorf("could not create --workdir %q: %v", workdir, err)
	}
	return abs, nil
}

func buildGitCommands(cloneURL string, ref string, dest string) []gitCommand {
	ref = strings.TrimSpace(ref)
	cloneArgs := []string{"clone", "--depth", "1", "--single-branch"}
	if ref != "" && !isCommitSHA(ref) {
		cloneArgs = append(cloneArgs, "--branch", ref)
	}
	cloneArgs = append(cloneArgs, cloneURL, dest)
	commands := []gitCommand{{Args: cloneArgs}}
	if ref != "" && isCommitSHA(ref) {
		commands = append(commands,
			gitCommand{Dir: dest, Args: []string{"fetch", "--depth", "1", "origin", ref}},
			gitCommand{Dir: dest, Args: []string{"checkout", "--detach", ref}},
		)
	}
	return commands
}

func isCommitSHA(ref string) bool {
	return commitSHARE.MatchString(strings.TrimSpace(ref))
}

func gitCommandError(args []string, err error) error {
	if len(args) >= 1 && args[0] == "clone" {
		return fmt.Errorf("git clone failed; check the repository URL, ref, and local Git authentication: %v", err)
	}
	if len(args) >= 1 && args[0] == "fetch" {
		return fmt.Errorf("git fetch failed for requested commit; check that the commit exists and is reachable: %v", err)
	}
	if len(args) >= 1 && args[0] == "checkout" {
		return fmt.Errorf("git checkout failed for requested ref: %v", err)
	}
	return fmt.Errorf("git command failed: %v", err)
}

func repoDirectoryName(cloneURL string) string {
	value := cloneURL
	if parsed, err := url.Parse(cloneURL); err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	if matches := scpStyleRE.FindStringSubmatch(cloneURL); len(matches) == 3 {
		value = matches[2]
	}
	value = strings.TrimRight(value, "/")
	base := filepath.Base(value)
	base = strings.TrimSuffix(base, ".git")
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "repository"
	}
	return base
}

func sanitizeForReport(value string) string {
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" {
		parsed.User = nil
		return parsed.String()
	}
	return value
}

func (execGitRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (execGitRunner) Run(ctx context.Context, dir string, stdin io.Reader, stdout io.Writer, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (execGitRunner) Output(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
