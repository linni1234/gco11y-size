package dotnet

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type projectFile struct {
	PropertyGroups []projectPropertyGroup `xml:"PropertyGroup"`
}

type projectPropertyGroup struct {
	AssemblyName  string `xml:"AssemblyName"`
	RootNamespace string `xml:"RootNamespace"`
	PackageID     string `xml:"PackageId"`
}

func discoverProjects(repo string, result *framework.Result, serviceNames map[string]string) int {
	projects := 0
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
		if !strings.EqualFold(filepath.Ext(path), ".csproj") {
			return nil
		}
		projects++
		root := filepath.Dir(path)
		if serviceNames[root] != "" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Warnings = append(result.Warnings, readErr.Error())
			return nil
		}
		serviceName := serviceNameFromProject(path, content)
		serviceNames[root] = serviceName
		result.ServiceNames[root] = serviceName
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
			Kind:    "service-name",
			Name:    "dotnet.project",
			Value:   serviceName,
			Source:  basecommon.RelPath(repo, path),
			Service: serviceName,
		})
		return nil
	})
	return projects
}

func serviceNameFromProject(path string, content []byte) string {
	var project projectFile
	if err := xml.Unmarshal(content, &project); err == nil {
		for _, group := range project.PropertyGroups {
			for _, candidate := range []string{group.AssemblyName, group.RootNamespace, group.PackageID} {
				if strings.TrimSpace(candidate) != "" {
					return basecommon.SanitizeServiceName(candidate)
				}
			}
		}
	}
	return basecommon.SanitizeServiceName(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

func dotnetConfigMetadata(content []byte) (serviceNames []configValue, environments []configValue) {
	var root any
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, nil
	}
	flat := map[string]string{}
	flattenJSON("", root, flat)
	for key, value := range flat {
		lowerKey := strings.ToLower(key)
		switch {
		case strings.HasSuffix(lowerKey, "otel_service_name"), strings.HasSuffix(lowerKey, "service.name"), strings.HasSuffix(lowerKey, "servicename"), strings.HasSuffix(lowerKey, "applicationname"):
			serviceNames = append(serviceNames, configValue{Name: key, Value: basecommon.SanitizeServiceName(value)})
		case strings.HasSuffix(lowerKey, "otel_resource_attributes"):
			if serviceName := serviceNameFromResourceAttributes(value); serviceName != "" {
				serviceNames = append(serviceNames, configValue{Name: key + ".service.name", Value: serviceName})
			}
		case strings.HasSuffix(lowerKey, "aspnetcore_environment"), strings.HasSuffix(lowerKey, "dotnet_environment"), strings.HasSuffix(lowerKey, "deployment.environment"):
			environments = append(environments, configValue{Name: key, Value: strings.TrimSpace(value)})
		case strings.HasSuffix(lowerKey, "environment") && !strings.Contains(lowerKey, "logging"):
			environments = append(environments, configValue{Name: key, Value: strings.TrimSpace(value)})
		}
	}
	return serviceNames, environments
}

type configValue struct {
	Name  string
	Value string
}

func flattenJSON(prefix string, value any, out map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenJSON(next, child, out)
		}
	case []any:
		for i, child := range typed {
			next := fmt.Sprintf("%s[%d]", prefix, i)
			flattenJSON(next, child, out)
		}
	case string:
		out[prefix] = typed
	}
}

func serviceNameFromResourceAttributes(value string) string {
	for _, part := range strings.Split(value, ",") {
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == "service.name" {
			return basecommon.SanitizeServiceName(val)
		}
	}
	return ""
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
