package python

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	projectSectionRE  = regexp.MustCompile(`(?ms)^\[project\]\s*(.*?)(?:^\[|\z)`)
	poetrySectionRE   = regexp.MustCompile(`(?ms)^\[tool\.poetry\]\s*(.*?)(?:^\[|\z)`)
	metadataSectionRE = regexp.MustCompile(`(?ms)^\[metadata\]\s*(.*?)(?:^\[|\z)`)
	nameLineRE        = regexp.MustCompile(`(?m)^\s*name\s*=\s*["']?([^"'\n]+)["']?`)
	setupNameRE       = regexp.MustCompile(`(?s)\bsetup\s*\(.*?\bname\s*=\s*["']([^"']+)["']`)
	envServiceNameRE  = regexp.MustCompile(`(?m)\bOTEL_SERVICE_NAME\s*=\s*["']?([A-Za-z0-9_.-]+)["']?`)
)

func discoverPackages(repo string, result *framework.Result, serviceNames map[string]string) int {
	roots := map[string]bool{}
	_ = filepath.WalkDir(repo, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			result.Warnings = append(result.Warnings, err.Error())
			return nil
		}
		if entry.IsDir() {
			if basecommon.ShouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isPythonRootFile(filepath.Base(path)) {
			return nil
		}
		root := filepath.Dir(path)
		if filepath.Base(path) == "manage.py" || strings.EqualFold(filepath.Base(path), "Dockerfile") || filepath.Base(path) == "Procfile" {
			root = inferPythonServiceRoot(repo, path)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Warnings = append(result.Warnings, readErr.Error())
			return nil
		}
		serviceName := serviceNames[root]
		if serviceName == "" {
			serviceName = serviceNameFromConfig(root, filepath.Base(path), string(content))
			serviceNames[root] = serviceName
			result.ServiceNames[root] = serviceName
		}
		roots[root] = true
		ensureService(result, model.Service{Name: serviceName, Root: basecommon.RelPath(repo, root), Source: "config"})
		source := basecommon.RelPath(repo, path)
		if finding := serviceNameFinding(serviceName, filepath.Base(path), string(content), source); finding.Name != "" {
			finding.Service = serviceName
			result.ConfigFindings = append(result.ConfigFindings, finding)
		}
		result.ConfigFindings = append(result.ConfigFindings, packageFrameworkFindings(serviceName, source, string(content))...)
		return nil
	})
	return len(roots)
}

func serviceNameFromConfig(root string, name string, content string) string {
	if service := serviceNameInContent(name, content); service != "" {
		return basecommon.SanitizeServiceName(service)
	}
	return basecommon.SanitizeServiceName(filepath.Base(root))
}

func serviceNameInContent(name string, content string) string {
	switch strings.ToLower(name) {
	case "pyproject.toml":
		if value := sectionName(projectSectionRE, content); value != "" {
			return value
		}
		if value := sectionName(poetrySectionRE, content); value != "" {
			return value
		}
	case "setup.cfg":
		if value := sectionName(metadataSectionRE, content); value != "" {
			return value
		}
	case "setup.py":
		if match := setupNameRE.FindStringSubmatch(content); len(match) == 2 {
			return match[1]
		}
	}
	if match := envServiceNameRE.FindStringSubmatch(content); len(match) == 2 {
		return match[1]
	}
	return ""
}

func serviceNameFinding(serviceName string, fileName string, content string, source string) model.ConfigFinding {
	switch strings.ToLower(fileName) {
	case "pyproject.toml":
		if sectionName(projectSectionRE, content) != "" {
			return model.ConfigFinding{Kind: "service-name", Name: "project.name", Value: serviceName, Source: source}
		}
		if sectionName(poetrySectionRE, content) != "" {
			return model.ConfigFinding{Kind: "service-name", Name: "tool.poetry.name", Value: serviceName, Source: source}
		}
	case "setup.cfg":
		if sectionName(metadataSectionRE, content) != "" {
			return model.ConfigFinding{Kind: "service-name", Name: "metadata.name", Value: serviceName, Source: source}
		}
	case "setup.py":
		if setupNameRE.MatchString(content) {
			return model.ConfigFinding{Kind: "service-name", Name: "setup.name", Value: serviceName, Source: source}
		}
	}
	if envServiceNameRE.MatchString(content) {
		return model.ConfigFinding{Kind: "service-name", Name: "OTEL_SERVICE_NAME", Value: serviceName, Source: source}
	}
	return model.ConfigFinding{}
}

func sectionName(sectionRE *regexp.Regexp, content string) string {
	match := sectionRE.FindStringSubmatch(content)
	if len(match) != 2 {
		return ""
	}
	name := nameLineRE.FindStringSubmatch(match[1])
	if len(name) != 2 {
		return ""
	}
	return strings.TrimSpace(name[1])
}

func packageFrameworkFindings(serviceName string, source string, content string) []model.ConfigFinding {
	lower := strings.ToLower(content)
	var findings []model.ConfigFinding
	added := map[string]bool{}
	add := func(kind string, name string) {
		key := kind + "\x00" + name
		if added[key] {
			return
		}
		added[key] = true
		findings = append(findings, model.ConfigFinding{Kind: kind, Name: name, Value: "detected", Source: source, Service: serviceName})
	}
	for dep, kind := range map[string]string{
		"fastapi":                       "python-web-framework",
		"starlette":                     "python-web-framework",
		"flask":                         "python-web-framework",
		"quart":                         "python-web-framework",
		"django":                        "python-web-framework",
		"djangorestframework":           "python-web-framework",
		"sanic":                         "python-web-framework",
		"aiohttp":                       "python-web-framework",
		"tornado":                       "python-web-framework",
		"falcon":                        "python-web-framework",
		"bottle":                        "python-web-framework",
		"litestar":                      "python-web-framework",
		"starlite":                      "python-web-framework",
		"sqlalchemy":                    "python-dependency-framework",
		"flask-sqlalchemy":              "python-dependency-framework",
		"psycopg":                       "python-dependency-framework",
		"psycopg2":                      "python-dependency-framework",
		"asyncpg":                       "python-dependency-framework",
		"pymysql":                       "python-dependency-framework",
		"pymongo":                       "python-dependency-framework",
		"motor":                         "python-dependency-framework",
		"redis":                         "python-dependency-framework",
		"elasticsearch":                 "python-dependency-framework",
		"opensearch-py":                 "python-dependency-framework",
		"qdrant-client":                 "python-dependency-framework",
		"weaviate-client":               "python-dependency-framework",
		"pinecone":                      "python-dependency-framework",
		"chromadb":                      "python-dependency-framework",
		"pymilvus":                      "python-dependency-framework",
		"celery":                        "python-messaging-framework",
		"rq":                            "python-messaging-framework",
		"dramatiq":                      "python-messaging-framework",
		"huey":                          "python-messaging-framework",
		"apscheduler":                   "python-messaging-framework",
		"kafka-python":                  "python-messaging-framework",
		"confluent-kafka":               "python-messaging-framework",
		"aiokafka":                      "python-messaging-framework",
		"pika":                          "python-messaging-framework",
		"aio-pika":                      "python-messaging-framework",
		"boto3":                         "python-cloud-sdk",
		"google-cloud-pubsub":           "python-cloud-sdk",
		"azure-servicebus":              "python-cloud-sdk",
		"opentelemetry-api":             "instrumentation",
		"opentelemetry-sdk":             "instrumentation",
		"opentelemetry-instrumentation": "instrumentation",
		"ddtrace":                       "instrumentation",
		"sentry-sdk":                    "instrumentation",
		"openai":                        "python-ai-sdk",
		"anthropic":                     "python-ai-sdk",
		"langchain":                     "python-ai-sdk",
		"llama-index":                   "python-ai-sdk",
	} {
		if strings.Contains(lower, dep) {
			name := dep
			if dep == "djangorestframework" {
				name = "django-rest-framework"
			}
			if kind == "instrumentation" && strings.HasPrefix(dep, "opentelemetry") {
				name = "opentelemetry-python"
			}
			add(kind, name)
		}
	}
	return findings
}

func serviceNameForRoot(repo string, root string, serviceNames map[string]string) (string, string) {
	if serviceName := serviceNames[root]; serviceName != "" {
		return serviceName, "config"
	}
	return basecommon.FallbackServiceName(repo, root), "path"
}

func inferPythonServiceRoot(repo string, file string) string {
	for dir := filepath.Dir(file); dir != repo && dir != "."; dir = filepath.Dir(dir) {
		for _, marker := range []string{"pyproject.toml", "setup.cfg", "setup.py", "requirements.txt", "Pipfile", "uv.lock", "poetry.lock", "manage.py", "serverless.yml", "serverless.yaml", "Procfile"} {
			if fileExists(filepath.Join(dir, marker)) {
				return dir
			}
		}
		if fileExists(filepath.Join(dir, "Dockerfile")) {
			return dir
		}
	}
	rel, err := filepath.Rel(repo, file)
	if err == nil {
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
	}
	return repo
}

func ensureService(result *framework.Result, service model.Service) {
	if service.Name == "" {
		return
	}
	for _, existing := range result.Services {
		if existing.Name == service.Name {
			return
		}
	}
	result.Services = append(result.Services, service)
}

func copyServiceNames(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func appendUnique(values []string, extra ...string) []string {
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

func isPythonRootFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == "pyproject.toml" ||
		lower == "setup.cfg" ||
		lower == "setup.py" ||
		strings.HasPrefix(lower, "requirements") && strings.HasSuffix(lower, ".txt") ||
		lower == "pipfile" ||
		lower == "poetry.lock" ||
		lower == "uv.lock" ||
		lower == "manage.py" ||
		lower == "procfile" ||
		lower == "dockerfile" ||
		lower == "serverless.yml" ||
		lower == "serverless.yaml"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
