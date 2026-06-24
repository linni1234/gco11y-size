package golang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var moduleRE = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)

func discoverGoModules(repo string, result *framework.Result, serviceNames map[string]string) int {
	modules := 0
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
		if filepath.Base(path) != "go.mod" {
			return nil
		}
		modules++
		root := filepath.Dir(path)
		if serviceNames[root] != "" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Warnings = append(result.Warnings, readErr.Error())
			return nil
		}
		match := moduleRE.FindStringSubmatch(string(content))
		if len(match) != 2 {
			return nil
		}
		serviceName := serviceNameFromModule(match[1])
		serviceNames[root] = serviceName
		result.ServiceNames[root] = serviceName
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
			Kind:    "service-name",
			Name:    "go.module",
			Value:   serviceName,
			Source:  basecommon.RelPath(repo, path),
			Service: serviceName,
		})
		return nil
	})
	return modules
}

func serviceNameFromModule(modulePath string) string {
	modulePath = strings.TrimSpace(modulePath)
	modulePath = strings.TrimSuffix(modulePath, ".git")
	modulePath = strings.Trim(modulePath, "/")
	if slash := strings.LastIndex(modulePath, "/"); slash >= 0 {
		modulePath = modulePath[slash+1:]
	}
	return basecommon.SanitizeServiceName(modulePath)
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
