package common

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func ShouldSkipDir(name string) bool {
	switch name {
	case ".git", ".idea", ".vscode", ".vs", "target", "build", "out", "bin", "obj", "TestResults", "node_modules", ".gradle", ".venv", "venv", "__pycache__", "vendor", "packages":
		return true
	default:
		return false
	}
}

func IsTestPath(repo string, path string) bool {
	rel := RelPath(repo, path)
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "src" && parts[i+1] == "test" {
			return true
		}
	}
	for _, part := range parts {
		lower := strings.ToLower(part)
		if lower == "test" || lower == "tests" || lower == "__tests__" {
			return true
		}
	}
	return false
}

func IsConfigFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if strings.HasPrefix(name, "application.") || strings.HasPrefix(name, "bootstrap.") || strings.HasPrefix(name, "appsettings.") || name == "appsettings.json" || name == "launchsettings.json" {
		return true
	}
	switch ext {
	case ".properties", ".yaml", ".yml", ".conf", ".river", ".json", ".xml", ".toml":
		return true
	default:
		return false
	}
}

func RelPath(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	if rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func InferServiceRoot(repo string, file string) string {
	rel, err := filepath.Rel(repo, file)
	if err != nil {
		return repo
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		if part == "src" && i > 0 {
			root := repo
			for _, prefix := range parts[:i] {
				root = filepath.Join(root, prefix)
			}
			return root
		}
	}
	for dir := filepath.Dir(file); dir != repo && dir != "."; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "pom.xml")) || fileExists(filepath.Join(dir, "build.gradle")) || fileExists(filepath.Join(dir, "build.gradle.kts")) || fileExists(filepath.Join(dir, "go.mod")) || hasFileWithExt(dir, ".csproj") || hasFileWithExt(dir, ".sln") {
			return dir
		}
	}
	return repo
}

func FallbackServiceName(repo string, root string) string {
	if root == repo {
		base := filepath.Base(repo)
		if base != "" && base != "." {
			return SanitizeServiceName(base)
		}
		return "application"
	}
	return SanitizeServiceName(filepath.Base(root))
}

func SanitizeServiceName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, `"'`)
	if name == "" {
		return "application"
	}
	name = strings.ReplaceAll(name, "_", "-")
	return strings.ToLower(name)
}

func NormalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "/"
	}
	if !strings.HasPrefix(route, "/") {
		route = "/" + route
	}
	route = regexp.MustCompile(`/+`).ReplaceAllString(route, "/")
	route = regexp.MustCompile(`\{([^}:]+):[^}]+\}`).ReplaceAllString(route, `{$1}`)
	if len(route) > 1 {
		route = strings.TrimRight(route, "/")
	}
	return route
}

func AppendUnique(values []string, extra ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range extra {
		if value != "" && !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func LooksHighCardinalityAttribute(attribute string) bool {
	keywords := []string{"user", "email", "session", "request.id", "trace.id", "span.id", "uuid", ".id", "_id", "token", "jwt"}
	for _, keyword := range keywords {
		if strings.Contains(attribute, keyword) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func hasFileWithExt(dir string, ext string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ext) {
			return true
		}
	}
	return false
}
