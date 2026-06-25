package jaxrs

import (
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type annotationMapping struct {
	methods []string
	paths   []string
}

type Options struct {
	BasePath              string
	Detector              string
	Confidence            string
	SkipRestClientClasses bool
}

func Operations(serviceName string, sourcePath string, source string) []model.Operation {
	return OperationsWithOptions(serviceName, sourcePath, source, Options{SkipRestClientClasses: true})
}

func OperationsWithOptions(serviceName string, sourcePath string, source string, opts Options) []model.Operation {
	detector := opts.Detector
	if detector == "" {
		detector = "jax-rs"
	}
	confidence := opts.Confidence
	if confidence == "" {
		confidence = "high"
	}
	clean := javacommon.StripJavaComments(source)
	lines := strings.Split(clean, "\n")
	var pending []string
	classPaths := []string{""}
	className := ""
	skipClass := false
	var operations []model.Operation

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			block := trimmed
			balance := javacommon.ParenBalance(block)
			for balance > 0 && i+1 < len(lines) {
				i++
				next := strings.TrimSpace(lines[i])
				block += " " + next
				balance += javacommon.ParenBalance(next)
			}
			pending = append(pending, block)
			continue
		}
		if matches := javacommon.JavaClassDeclRE.FindStringSubmatch(trimmed); len(matches) == 2 {
			className = matches[1]
			skipClass = opts.SkipRestClientClasses && hasAnnotation(pending, "RegisterRestClient")
			if paths := paths(pending); len(paths) > 0 {
				classPaths = paths
			} else {
				classPaths = []string{""}
			}
			pending = nil
			continue
		}
		if matches := javacommon.JavaMethodDeclRE.FindStringSubmatch(trimmed); len(matches) == 2 {
			methodName := matches[1]
			if !skipClass {
				for _, mapping := range methodMappings(pending) {
					for _, base := range classPaths {
						for _, path := range mapping.paths {
							for _, method := range mapping.methods {
								operations = append(operations, javacommon.Operation(
									serviceName,
									"SERVER",
									"http",
									method,
									basecommon.NormalizeRoute(javacommon.JoinRoutes(opts.BasePath, javacommon.JoinRoutes(base, path))),
									strings.Trim(className+"."+methodName, "."),
									sourcePath,
									detector,
									confidence,
								))
							}
						}
					}
				}
			}
			pending = nil
			continue
		}
		if strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import ") || strings.HasSuffix(trimmed, ";") {
			pending = nil
		}
	}
	return operations
}

func hasAnnotation(annotations []string, name string) bool {
	for _, annotation := range annotations {
		if javacommon.SimpleAnnotationName(annotation) == name {
			return true
		}
	}
	return false
}

func paths(annotations []string) []string {
	for _, annotation := range annotations {
		if javacommon.SimpleAnnotationName(annotation) == "Path" {
			paths := javacommon.ExtractAnnotationStrings(javacommon.AnnotationArgs(annotation))
			if len(paths) > 0 {
				return paths
			}
		}
	}
	return nil
}

func methodMappings(annotations []string) []annotationMapping {
	var mappings []annotationMapping
	methodPaths := []string{""}
	if annotationPaths := paths(annotations); len(annotationPaths) > 0 {
		methodPaths = annotationPaths
	}
	for _, annotation := range annotations {
		method := ""
		switch javacommon.SimpleAnnotationName(annotation) {
		case "GET":
			method = "GET"
		case "POST":
			method = "POST"
		case "PUT":
			method = "PUT"
		case "DELETE":
			method = "DELETE"
		case "PATCH":
			method = "PATCH"
		case "HEAD":
			method = "HEAD"
		case "OPTIONS":
			method = "OPTIONS"
		}
		if method != "" {
			mappings = append(mappings, annotationMapping{methods: []string{method}, paths: methodPaths})
		}
	}
	return mappings
}
