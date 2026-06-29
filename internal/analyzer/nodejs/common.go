package nodejs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type packageJSON struct {
	Name         string            `json:"name"`
	Dependencies map[string]string `json:"dependencies"`
	DevDeps      map[string]string `json:"devDependencies"`
	Scripts      map[string]string `json:"scripts"`
	Workspaces   any               `json:"workspaces"`
}

func discoverPackages(repo string, result *framework.Result, serviceNames map[string]string) int {
	packages := 0
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
		if filepath.Base(path) != "package.json" {
			return nil
		}
		packages++
		root := filepath.Dir(path)
		if serviceNames[root] != "" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Warnings = append(result.Warnings, readErr.Error())
			return nil
		}
		pkg := parsePackage(content)
		serviceName := serviceNameFromPackage(root, pkg)
		serviceNames[root] = serviceName
		result.ServiceNames[root] = serviceName
		ensureService(result, model.Service{Name: serviceName, Root: basecommon.RelPath(repo, root), Source: "config"})
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
			Kind:    "service-name",
			Name:    "package.name",
			Value:   serviceName,
			Source:  basecommon.RelPath(repo, path),
			Service: serviceName,
		})
		result.ConfigFindings = append(result.ConfigFindings, packageFrameworkFindings(serviceName, basecommon.RelPath(repo, path), pkg)...)
		return nil
	})
	return packages
}

func parsePackage(content []byte) packageJSON {
	var pkg packageJSON
	_ = json.Unmarshal(content, &pkg)
	return pkg
}

func serviceNameFromPackage(root string, pkg packageJSON) string {
	name := strings.TrimSpace(pkg.Name)
	if name == "" {
		return basecommon.SanitizeServiceName(filepath.Base(root))
	}
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		name = name[slash+1:]
	}
	return basecommon.SanitizeServiceName(name)
}

func packageFrameworkFindings(serviceName string, source string, pkg packageJSON) []model.ConfigFinding {
	deps := map[string]string{}
	for name := range pkg.Dependencies {
		deps[name] = "dependency"
	}
	for name := range pkg.DevDeps {
		if deps[name] == "" {
			deps[name] = "devDependency"
		}
	}
	var findings []model.ConfigFinding
	for dep := range deps {
		switch dep {
		case "express", "fastify", "@nestjs/core", "@nestjs/common", "next", "koa", "@koa/router", "hapi", "@hapi/hapi", "hono":
			findings = append(findings, model.ConfigFinding{Kind: "nodejs-web-framework", Name: dep, Value: "detected", Source: source, Service: serviceName})
		case "prisma", "@prisma/client", "typeorm", "sequelize", "knex", "mongoose", "mongodb", "pg", "mysql", "mysql2", "mssql", "sqlite3", "redis", "ioredis":
			findings = append(findings, model.ConfigFinding{Kind: "nodejs-dependency-framework", Name: dep, Value: "detected", Source: source, Service: serviceName})
		case "kafkajs", "node-rdkafka", "amqplib", "bullmq", "bull", "bee-queue", "@aws-sdk/client-sqs", "@aws-sdk/client-sns", "@google-cloud/pubsub", "@azure/service-bus":
			findings = append(findings, model.ConfigFinding{Kind: "nodejs-messaging-framework", Name: dep, Value: "detected", Source: source, Service: serviceName})
		case "@opentelemetry/api", "@opentelemetry/sdk-node", "@opentelemetry/resources":
			findings = append(findings, model.ConfigFinding{Kind: "instrumentation", Name: "opentelemetry-js", Value: "detected", Source: source, Service: serviceName})
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

func isJavaScriptSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".mjs", ".cjs", ".jsx", ".ts", ".mts", ".cts", ".tsx":
		return true
	default:
		return false
	}
}

func isGeneratedJS(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, ".min.js") ||
		strings.HasSuffix(name, ".bundle.js") ||
		strings.HasSuffix(name, ".chunk.js") ||
		strings.HasSuffix(name, ".generated.js") ||
		strings.HasSuffix(name, ".generated.ts") ||
		strings.Contains(filepath.ToSlash(path), "/generated/")
}

func isConfigRootFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == "serverless.yml" ||
		lower == "serverless.yaml" ||
		lower == "nest-cli.json" ||
		strings.HasPrefix(lower, "next.config.")
}
