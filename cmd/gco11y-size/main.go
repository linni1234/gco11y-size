package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer"
	"github.com/nilslindholm/metricgenerationsizer/internal/config"
	"github.com/nilslindholm/metricgenerationsizer/internal/estimator"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
	"github.com/nilslindholm/metricgenerationsizer/internal/report"
	"github.com/nilslindholm/metricgenerationsizer/internal/source"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return fmt.Errorf("missing command")
	}
	switch args[0] {
	case "scan":
		return runScan(args[1:])
	case "-h", "--help", "help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runScan(args []string) error {
	var dimensions config.DimensionFlags
	var repos stringList
	var environmentNames stringList
	var serviceInstances serviceInstanceFlags
	var serviceEnvironments serviceEnvironmentFlags
	defaultProcessors := strings.Join(config.DefaultProcessors, ",")
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	flags.Var(&repos, "repo", "application repository path or Git URL to scan; may be repeated")
	input := flags.String("input", "", "optional JSON workspace input file containing defaults, repos, and service overrides")
	otelConfig := flags.String("otel-config", "", "optional Alloy, OTel Collector, Spring, or Kubernetes config file/directory")
	ref := flags.String("ref", "", "optional branch, tag, or commit SHA for remote Git repositories")
	workdir := flags.String("workdir", "", "optional parent directory for temporary remote Git worktrees")
	keepWorktree := flags.Bool("keep-worktree", false, "keep temporary remote Git worktree after scanning")
	gatewayMethodCount := flags.Int("gateway-method-count", 0, "optional method multiplier for Spring Cloud Gateway ANY routes when http.method is a span metric dimension")
	gatewayMethods := flags.String("gateway-methods", "", "optional comma-separated HTTP methods for Spring Cloud Gateway ANY routes, for example GET,POST,PUT,DELETE,PATCH")
	outHTML := flags.String("out", "report.html", "standalone HTML report path; use empty string to skip")
	outJSON := flags.String("json", "report.json", "JSON report path; use empty string to skip")
	processors := flags.String("processors", defaultProcessors, "comma-separated processors: span-metrics-count,span-metrics-latency,span-metrics-size,service-graph,host-info")
	histogramType := flags.String("histogram-type", config.HistogramTypeNative, "histogram implementation for span metrics and service graphs: native, classic, or both")
	histogramBuckets := flags.Int("histogram-buckets", 14, "emitted classic histogram _bucket series per label set for --histogram-type=classic or both; include +Inf")
	nativeHistograms := flags.Bool("native-histograms", false, "deprecated alias for --histogram-type=both")
	statusValues := flags.Int("status-values", 3, "expected status_code label values per operation")
	environmentCount := flags.Int("environments", 1, "number of deployment environments")
	flags.Var(&environmentNames, "environment", "deployment environment name; may be repeated and overrides --environments count for sizing")
	instancesPerService := flags.Int("instances-per-service", 1, "expected service instances per environment for host info")
	instanceLabel := flags.String("instance-label", "enabled", "generated trace metrics instance label: enabled or disabled")
	includeClientProducer := flags.Bool("include-client-producer", false, "include CLIENT and PRODUCER span kinds in span-metric operation sizing")
	grafanaQuery := flags.Bool("grafana-query", false, "query Grafana Cloud Prometheus when GRAFANA_CLOUD_PROM_URL and token env vars are set")
	flags.Var(&dimensions, "dimension", "custom span dimension as name=cardinality; may be repeated")
	flags.Var(&serviceInstances, "service-instances", "service instance override as service=count or repo:service=count; may be repeated")
	flags.Var(&serviceEnvironments, "service-environments", "service environment override as service=prod,staging or repo:service=prod,staging; may be repeated")

	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gco11y-size scan --repo <path> [--repo <path>] [options]")
		fmt.Fprintln(os.Stderr, "       gco11y-size scan --input sizing.json [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	visited := map[string]bool{}
	flags.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	resolvedHistogramType := config.NormalizeHistogramType(*histogramType)
	if visited["native-histograms"] && *nativeHistograms {
		if visited["histogram-type"] && resolvedHistogramType != config.HistogramTypeBoth {
			return fmt.Errorf("--native-histograms is deprecated; use --histogram-type=both instead")
		}
		resolvedHistogramType = config.HistogramTypeBoth
	}
	instanceLabelEnabled, err := parseEnabledDisabled(*instanceLabel)
	if err != nil {
		return fmt.Errorf("--instance-label must be enabled or disabled")
	}

	defaultOpts := model.Options{
		OtelConfig:            *otelConfig,
		Ref:                   *ref,
		Workdir:               *workdir,
		KeepWorktree:          *keepWorktree,
		GatewayMethodCount:    *gatewayMethodCount,
		GatewayMethods:        splitGatewayMethods(*gatewayMethods),
		Profile:               "grafana-cloud-app-o11y",
		Processors:            config.SplitProcessors(*processors),
		HistogramBuckets:      *histogramBuckets,
		HistogramType:         resolvedHistogramType,
		StatusValues:          *statusValues,
		Environments:          *environmentCount,
		EnvironmentNames:      append([]string(nil), environmentNames...),
		InstancesPerService:   *instancesPerService,
		InstanceLabelEnabled:  instanceLabelEnabled,
		ServiceOverrides:      mergeServiceOverrideFlags(serviceInstances, serviceEnvironments),
		CustomDimensions:      append([]model.Dimension(nil), dimensions...),
		IncludeClientProducer: *includeClientProducer,
		GrafanaQuery:          *grafanaQuery,
	}
	if len(defaultOpts.EnvironmentNames) > 0 {
		defaultOpts.Environments = len(defaultOpts.EnvironmentNames)
	}
	repoInputs := repositoryInputsFromFlags(repos)
	if *input != "" {
		loadedDefaults, loadedRepos, err := loadWorkspaceInput(*input, defaultOpts, visited)
		if err != nil {
			return err
		}
		defaultOpts = loadedDefaults
		repoInputs = loadedRepos
	}
	if len(repoInputs) == 0 {
		repoInputs = []model.RepositoryInput{{Repo: "."}}
	}
	if len(repoInputs) == 1 && *input == "" {
		return runSingleScan(repoInputs[0], defaultOpts, *outHTML, *outJSON)
	}
	return runWorkspaceScan(repoInputs, defaultOpts, *outHTML, *outJSON)
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}
	*s = append(*s, value)
	return nil
}

type serviceInstanceFlags []model.ServiceSizingOverride

func (s *serviceInstanceFlags) String() string {
	return ""
}

func (s *serviceInstanceFlags) Set(value string) error {
	service, repository, rawCount, err := parseServiceOverrideAssignment(value)
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(rawCount)
	if err != nil || count < 1 {
		return fmt.Errorf("service instance override %q must use a positive integer", value)
	}
	*s = append(*s, model.ServiceSizingOverride{Repository: repository, Service: service, InstancesPerService: count})
	return nil
}

type serviceEnvironmentFlags []model.ServiceSizingOverride

func (s *serviceEnvironmentFlags) String() string {
	return ""
}

func (s *serviceEnvironmentFlags) Set(value string) error {
	service, repository, rawEnvironments, err := parseServiceOverrideAssignment(value)
	if err != nil {
		return err
	}
	environments := splitList(rawEnvironments)
	if len(environments) == 0 {
		return fmt.Errorf("service environment override %q must include at least one environment", value)
	}
	*s = append(*s, model.ServiceSizingOverride{Repository: repository, Service: service, EnvironmentNames: environments})
	return nil
}

func parseServiceOverrideAssignment(value string) (service string, repository string, rawValue string, err error) {
	left, right, ok := strings.Cut(strings.TrimSpace(value), "=")
	if !ok {
		return "", "", "", fmt.Errorf("service override %q must be service=value or repo:service=value", value)
	}
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return "", "", "", fmt.Errorf("service override %q must be service=value or repo:service=value", value)
	}
	if repo, svc, ok := strings.Cut(left, ":"); ok {
		repo = strings.TrimSpace(repo)
		svc = strings.TrimSpace(svc)
		if repo == "" || svc == "" {
			return "", "", "", fmt.Errorf("service override %q must be service=value or repo:service=value", value)
		}
		return svc, repo, right, nil
	}
	return left, "", right, nil
}

func mergeServiceOverrideFlags(instanceFlags serviceInstanceFlags, environmentFlags serviceEnvironmentFlags) []model.ServiceSizingOverride {
	merged := map[string]model.ServiceSizingOverride{}
	order := []string{}
	merge := func(override model.ServiceSizingOverride) {
		key := override.Repository + "\x00" + override.Service
		existing, ok := merged[key]
		if !ok {
			existing = model.ServiceSizingOverride{Repository: override.Repository, Service: override.Service}
			order = append(order, key)
		}
		if override.InstancesPerService > 0 {
			existing.InstancesPerService = override.InstancesPerService
		}
		if len(override.EnvironmentNames) > 0 {
			existing.EnvironmentNames = append([]string(nil), override.EnvironmentNames...)
			existing.Environments = 0
		} else if override.Environments > 0 {
			existing.Environments = override.Environments
		}
		merged[key] = existing
	}
	for _, override := range instanceFlags {
		merge(override)
	}
	for _, override := range environmentFlags {
		merge(override)
	}
	out := make([]model.ServiceSizingOverride, 0, len(order))
	for _, key := range order {
		out = append(out, merged[key])
	}
	return out
}

func repositoryInputsFromFlags(repos []string) []model.RepositoryInput {
	out := make([]model.RepositoryInput, 0, len(repos))
	for _, repo := range repos {
		out = append(out, model.RepositoryInput{Repo: repo})
	}
	return out
}

func runSingleScan(repoInput model.RepositoryInput, defaultOpts model.Options, htmlPath string, jsonPath string) error {
	generatedAt := time.Now().UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	calibration := config.QueryGrafanaCloud(ctx, defaultOpts.GrafanaQuery)
	repoReport, err := scanRepository(repoInput, defaultOpts, generatedAt, false, calibration)
	if err != nil {
		return err
	}
	finalReport := repoReport.Report
	if jsonPath != "" {
		if err := report.WriteJSON(jsonPath, finalReport); err != nil {
			return err
		}
	}
	if htmlPath != "" {
		if err := report.WriteHTML(htmlPath, finalReport); err != nil {
			return err
		}
	}
	printSummary(finalReport, htmlPath, jsonPath)
	return nil
}

func runWorkspaceScan(repoInputs []model.RepositoryInput, defaultOpts model.Options, htmlPath string, jsonPath string) error {
	generatedAt := time.Now().UTC()
	repositories := make([]model.RepositoryReport, 0, len(repoInputs))
	nameCounts := map[string]int{}
	for _, repoInput := range repoInputs {
		repoReport, err := scanRepository(repoInput, defaultOpts, generatedAt, true, nil)
		if err != nil {
			return err
		}
		uniqueName := uniqueRepositoryName(repoReport.Name, nameCounts)
		if uniqueName != repoReport.Name {
			renameRepositoryReport(&repoReport, uniqueName)
		}
		repositories = append(repositories, repoReport)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	calibration := config.QueryGrafanaCloud(ctx, defaultOpts.GrafanaQuery)
	workspace := buildWorkspaceReport(generatedAt, defaultOpts, repositories, calibration)
	if jsonPath != "" {
		if err := report.WriteWorkspaceJSON(jsonPath, workspace); err != nil {
			return err
		}
	}
	if htmlPath != "" {
		if err := report.WriteWorkspaceHTML(htmlPath, workspace); err != nil {
			return err
		}
	}
	printWorkspaceSummary(workspace, htmlPath, jsonPath)
	return nil
}

func scanRepository(repoInput model.RepositoryInput, defaultOpts model.Options, generatedAt time.Time, annotateRepository bool, calibration *model.CloudCalibration) (model.RepositoryReport, error) {
	opts := optionsForRepository(defaultOpts, repoInput)
	if err := config.ValidateOptions(opts); err != nil {
		return model.RepositoryReport{}, err
	}
	resolvedSource, err := source.Resolve(context.Background(), source.Options{
		Repo:         opts.Repo,
		Ref:          opts.Ref,
		Workdir:      opts.Workdir,
		KeepWorktree: opts.KeepWorktree,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
	})
	if err != nil {
		return model.RepositoryReport{}, err
	}
	if resolvedSource.Cleanup != nil {
		defer func() {
			if err := resolvedSource.Cleanup(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not clean temporary worktree %s: %v\n", resolvedSource.Source.ResolvedPath, err)
			}
		}()
	}
	opts.Repo = resolvedSource.Source.Original
	analysis, err := analyzer.Analyze(resolvedSource.Path, opts.OtelConfig)
	if err != nil {
		return model.RepositoryReport{}, err
	}
	name := repositoryDisplayName(repoInput, resolvedSource.Source)
	if annotateRepository {
		annotateAnalysisRepository(&analysis, name)
	}
	estimate := estimator.Estimate(analysis, opts, calibration)
	return model.RepositoryReport{
		Name: name,
		Report: model.Report{
			Version:     model.Version,
			GeneratedAt: generatedAt,
			Options:     opts,
			Source:      resolvedSource.Source,
			Analysis:    analysis,
			Estimate:    estimate,
		},
	}, nil
}

func optionsForRepository(defaultOpts model.Options, repoInput model.RepositoryInput) model.Options {
	opts := defaultOpts
	opts.Repo = repoInput.Repo
	if repoInput.Ref != "" {
		opts.Ref = repoInput.Ref
	}
	if repoInput.OtelConfig != "" {
		opts.OtelConfig = repoInput.OtelConfig
	}
	if repoInput.Workdir != "" {
		opts.Workdir = repoInput.Workdir
	}
	if repoInput.KeepWorktree {
		opts.KeepWorktree = true
	}
	if len(repoInput.EnvironmentNames) > 0 {
		opts.EnvironmentNames = append([]string(nil), repoInput.EnvironmentNames...)
		opts.Environments = len(repoInput.EnvironmentNames)
	} else if repoInput.Environments > 0 {
		opts.EnvironmentNames = nil
		opts.Environments = repoInput.Environments
	}
	if repoInput.InstancesPerService > 0 {
		opts.InstancesPerService = repoInput.InstancesPerService
	}
	opts.CustomDimensions = append([]model.Dimension(nil), defaultOpts.CustomDimensions...)
	opts.ServiceOverrides = append([]model.ServiceSizingOverride(nil), defaultOpts.ServiceOverrides...)
	return opts
}

func repositoryDisplayName(repoInput model.RepositoryInput, metadata model.SourceMetadata) string {
	if strings.TrimSpace(repoInput.Name) != "" {
		return strings.TrimSpace(repoInput.Name)
	}
	path := metadata.ResolvedPath
	if path == "" {
		path = metadata.Original
	}
	name := strings.TrimSuffix(filepath.Base(strings.TrimRight(path, "/")), ".git")
	if name == "" || name == "." {
		return "repository"
	}
	return name
}

func uniqueRepositoryName(name string, counts map[string]int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "repository"
	}
	counts[name]++
	if counts[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, counts[name])
}

func renameRepositoryReport(repoReport *model.RepositoryReport, name string) {
	repoReport.Name = name
	annotateAnalysisRepository(&repoReport.Report.Analysis, name)
}

func annotateAnalysisRepository(analysis *model.Analysis, repository string) {
	for i := range analysis.Services {
		analysis.Services[i].Repository = repository
	}
	for i := range analysis.Operations {
		analysis.Operations[i].Repository = repository
	}
	for i := range analysis.Edges {
		analysis.Edges[i].Repository = repository
	}
	for i := range analysis.ConfigFindings {
		analysis.ConfigFindings[i].Repository = repository
	}
	for i := range analysis.Risks {
		analysis.Risks[i].Repository = repository
	}
}

func buildWorkspaceReport(generatedAt time.Time, defaultOpts model.Options, repositories []model.RepositoryReport, calibration *model.CloudCalibration) model.WorkspaceReport {
	aggregateAnalysis := mergeRepositoryAnalyses(repositories)
	aggregateOpts := defaultOpts
	aggregateOpts.Repo = "workspace"
	aggregateOpts.ServiceOverrides = aggregateServiceOverrides(defaultOpts, repositories)
	aggregateEstimate := estimator.Estimate(aggregateAnalysis, aggregateOpts, calibration)
	aggregateReport := model.Report{
		Version:     model.Version,
		GeneratedAt: generatedAt,
		Options:     aggregateOpts,
		Source: model.SourceMetadata{
			Original:          "workspace",
			Type:              "workspace",
			ResolvedPath:      "workspace",
			WorktreeTemporary: false,
			WorktreeRetained:  true,
		},
		Analysis: aggregateAnalysis,
		Estimate: aggregateEstimate,
	}
	return model.WorkspaceReport{
		Version:      model.Version,
		GeneratedAt:  generatedAt,
		Options:      aggregateOpts,
		Aggregate:    aggregateReport,
		Repositories: repositories,
	}
}

func aggregateServiceOverrides(defaultOpts model.Options, repositories []model.RepositoryReport) []model.ServiceSizingOverride {
	overrides := append([]model.ServiceSizingOverride(nil), defaultOpts.ServiceOverrides...)
	for _, repo := range repositories {
		for _, service := range repo.Report.Analysis.Services {
			if explicitServiceOverrideExists(overrides, repo.Name, service.Name) {
				continue
			}
			overrides = append(overrides, model.ServiceSizingOverride{
				Repository:          repo.Name,
				Service:             service.Name,
				EnvironmentNames:    append([]string(nil), repo.Report.Options.EnvironmentNames...),
				Environments:        repo.Report.Options.Environments,
				InstancesPerService: repo.Report.Options.InstancesPerService,
			})
		}
	}
	return overrides
}

func explicitServiceOverrideExists(overrides []model.ServiceSizingOverride, repository string, service string) bool {
	for _, override := range overrides {
		if override.Service != service {
			continue
		}
		if override.Repository == "" || override.Repository == repository {
			return true
		}
	}
	return false
}

func mergeRepositoryAnalyses(repositories []model.RepositoryReport) model.Analysis {
	analysis := model.Analysis{Repository: "workspace", GeneratedAt: time.Now().UTC()}
	services := map[string]model.Service{}
	operations := map[string]model.Operation{}
	edges := map[string]model.Edge{}
	languages := []string{}
	for _, repo := range repositories {
		repoAnalysis := repo.Report.Analysis
		for _, service := range repoAnalysis.Services {
			existing := services[service.Name]
			if existing.Name == "" {
				services[service.Name] = service
				continue
			}
			existing.OperationCount += service.OperationCount
			existing.EdgeCount += service.EdgeCount
			if existing.Repository == "" {
				existing.Repository = service.Repository
			}
			services[service.Name] = existing
		}
		for _, op := range repoAnalysis.Operations {
			key := strings.Join([]string{op.Service, op.Kind, op.Protocol, op.Method, op.Route}, "\x00")
			if existing, ok := operations[key]; ok {
				existing.Detectors = appendUnique(existing.Detectors, op.Detectors...)
				existing.Risks = appendUnique(existing.Risks, op.Risks...)
				if existing.Confidence == "" {
					existing.Confidence = op.Confidence
				}
				operations[key] = existing
				continue
			}
			operations[key] = op
		}
		for _, edge := range repoAnalysis.Edges {
			key := strings.Join([]string{edge.SourceService, edge.TargetService, edge.Protocol}, "\x00")
			if _, ok := edges[key]; !ok {
				edges[key] = edge
			}
		}
		analysis.ConfigFindings = append(analysis.ConfigFindings, repoAnalysis.ConfigFindings...)
		analysis.Risks = append(analysis.Risks, repoAnalysis.Risks...)
		analysis.Warnings = append(analysis.Warnings, prefixWarnings(repo.Name, repoAnalysis.Warnings)...)
		languages = appendUnique(languages, splitList(repoAnalysis.DetectedLanguage)...)
	}
	for _, service := range services {
		analysis.Services = append(analysis.Services, service)
	}
	for _, op := range operations {
		analysis.Operations = append(analysis.Operations, op)
	}
	for _, edge := range edges {
		analysis.Edges = append(analysis.Edges, edge)
	}
	sort.Slice(analysis.Services, func(i, j int) bool {
		return analysis.Services[i].Name < analysis.Services[j].Name
	})
	sort.Slice(analysis.Operations, func(i, j int) bool {
		if analysis.Operations[i].Service != analysis.Operations[j].Service {
			return analysis.Operations[i].Service < analysis.Operations[j].Service
		}
		return analysis.Operations[i].Route < analysis.Operations[j].Route
	})
	sort.Slice(analysis.Edges, func(i, j int) bool {
		if analysis.Edges[i].SourceService != analysis.Edges[j].SourceService {
			return analysis.Edges[i].SourceService < analysis.Edges[j].SourceService
		}
		return analysis.Edges[i].TargetService < analysis.Edges[j].TargetService
	})
	analysis.DetectedLanguage = strings.Join(languages, ", ")
	return analysis
}

func prefixWarnings(repository string, warnings []string) []string {
	out := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, repository+": "+warning)
	}
	return out
}

func printWorkspaceSummary(r model.WorkspaceReport, htmlPath string, jsonPath string) {
	fmt.Printf("Scanned %d repos, %d service(s), %d operation(s), %d service-graph edge(s).\n", len(r.Repositories), len(r.Aggregate.Analysis.Services), len(r.Aggregate.Analysis.Operations), len(r.Aggregate.Analysis.Edges))
	fmt.Printf("Estimated active series: %d expected (%d low / %d high).\n", r.Aggregate.Estimate.TotalExpected, r.Aggregate.Estimate.TotalLow, r.Aggregate.Estimate.TotalHigh)
	if htmlPath != "" {
		fmt.Printf("HTML report: %s\n", displayPath(htmlPath))
	}
	if jsonPath != "" {
		fmt.Printf("JSON report: %s\n", displayPath(jsonPath))
	}
}

type workspaceInputFile struct {
	Defaults         workspaceDefaultsInput        `json:"defaults"`
	Repositories     []model.RepositoryInput       `json:"repos"`
	ServiceOverrides []model.ServiceSizingOverride `json:"service_overrides"`
}

type workspaceDefaultsInput struct {
	OtelConfig            string            `json:"otel_config"`
	Ref                   string            `json:"ref"`
	Workdir               string            `json:"workdir"`
	KeepWorktree          *bool             `json:"keep_worktree"`
	GatewayMethodCount    *int              `json:"gateway_method_count"`
	GatewayMethods        []string          `json:"gateway_methods"`
	Profile               string            `json:"profile"`
	Processors            []string          `json:"processors"`
	HistogramBuckets      *int              `json:"histogram_buckets"`
	HistogramType         string            `json:"histogram_type"`
	StatusValues          *int              `json:"status_values"`
	Environments          *environmentInput `json:"environments"`
	EnvironmentCount      *int              `json:"environment_count"`
	InstancesPerService   *int              `json:"instances_per_service"`
	InstanceLabel         string            `json:"instance_label"`
	InstanceLabelEnabled  *bool             `json:"instance_label_enabled"`
	CustomDimensions      []model.Dimension `json:"custom_dimensions"`
	Dimensions            []model.Dimension `json:"dimensions"`
	IncludeClientProducer *bool             `json:"include_client_producer"`
	GrafanaQuery          *bool             `json:"grafana_query"`
}

type environmentInput struct {
	Count int
	Names []string
}

func (e *environmentInput) UnmarshalJSON(data []byte) error {
	var names []string
	if err := json.Unmarshal(data, &names); err == nil {
		e.Names = cleanStringList(names)
		e.Count = len(e.Names)
		return nil
	}
	var count int
	if err := json.Unmarshal(data, &count); err == nil {
		if count < 1 {
			return fmt.Errorf("environment count must be at least 1")
		}
		e.Count = count
		e.Names = nil
		return nil
	}
	return fmt.Errorf("environments must be a positive integer or an array of names")
}

func loadWorkspaceInput(path string, cliDefaults model.Options, visited map[string]bool) (model.Options, []model.RepositoryInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Options{}, nil, fmt.Errorf("could not read --input %s: %v", path, err)
	}
	var input workspaceInputFile
	if err := json.Unmarshal(data, &input); err != nil {
		return model.Options{}, nil, fmt.Errorf("could not parse --input %s: %v", path, err)
	}
	if len(input.Repositories) == 0 {
		return model.Options{}, nil, fmt.Errorf("--input must include at least one repo in repos")
	}
	opts := applyWorkspaceDefaults(cliDefaults, input.Defaults)
	opts.ServiceOverrides = append(opts.ServiceOverrides, input.ServiceOverrides...)
	opts = applyExplicitCLIOverrides(opts, cliDefaults, visited)
	validationOpts := opts
	validationOpts.Repo = "workspace"
	if err := config.ValidateOptions(validationOpts); err != nil {
		return model.Options{}, nil, err
	}
	return opts, input.Repositories, nil
}

func applyWorkspaceDefaults(opts model.Options, defaults workspaceDefaultsInput) model.Options {
	if defaults.OtelConfig != "" {
		opts.OtelConfig = defaults.OtelConfig
	}
	if defaults.Ref != "" {
		opts.Ref = defaults.Ref
	}
	if defaults.Workdir != "" {
		opts.Workdir = defaults.Workdir
	}
	if defaults.KeepWorktree != nil {
		opts.KeepWorktree = *defaults.KeepWorktree
	}
	if defaults.GatewayMethodCount != nil {
		opts.GatewayMethodCount = *defaults.GatewayMethodCount
	}
	if len(defaults.GatewayMethods) > 0 {
		opts.GatewayMethods = cleanStringList(defaults.GatewayMethods)
	}
	if defaults.Profile != "" {
		opts.Profile = defaults.Profile
	}
	if len(defaults.Processors) > 0 {
		opts.Processors = config.SplitProcessors(strings.Join(defaults.Processors, ","))
	}
	if defaults.HistogramBuckets != nil {
		opts.HistogramBuckets = *defaults.HistogramBuckets
	}
	if defaults.HistogramType != "" {
		opts.HistogramType = config.NormalizeHistogramType(defaults.HistogramType)
	}
	if defaults.StatusValues != nil {
		opts.StatusValues = *defaults.StatusValues
	}
	if defaults.Environments != nil {
		if len(defaults.Environments.Names) > 0 {
			opts.EnvironmentNames = append([]string(nil), defaults.Environments.Names...)
			opts.Environments = len(defaults.Environments.Names)
		} else if defaults.Environments.Count > 0 {
			opts.EnvironmentNames = nil
			opts.Environments = defaults.Environments.Count
		}
	}
	if defaults.EnvironmentCount != nil {
		opts.EnvironmentNames = nil
		opts.Environments = *defaults.EnvironmentCount
	}
	if defaults.InstancesPerService != nil {
		opts.InstancesPerService = *defaults.InstancesPerService
	}
	if defaults.InstanceLabel != "" {
		enabled, err := parseEnabledDisabled(defaults.InstanceLabel)
		if err == nil {
			opts.InstanceLabelEnabled = enabled
		}
	}
	if defaults.InstanceLabelEnabled != nil {
		opts.InstanceLabelEnabled = *defaults.InstanceLabelEnabled
	}
	if len(defaults.CustomDimensions) > 0 {
		opts.CustomDimensions = append([]model.Dimension(nil), defaults.CustomDimensions...)
	}
	if len(defaults.Dimensions) > 0 {
		opts.CustomDimensions = append([]model.Dimension(nil), defaults.Dimensions...)
	}
	if defaults.IncludeClientProducer != nil {
		opts.IncludeClientProducer = *defaults.IncludeClientProducer
	}
	if defaults.GrafanaQuery != nil {
		opts.GrafanaQuery = *defaults.GrafanaQuery
	}
	return opts
}

func applyExplicitCLIOverrides(opts model.Options, cli model.Options, visited map[string]bool) model.Options {
	if visited["otel-config"] {
		opts.OtelConfig = cli.OtelConfig
	}
	if visited["ref"] {
		opts.Ref = cli.Ref
	}
	if visited["workdir"] {
		opts.Workdir = cli.Workdir
	}
	if visited["keep-worktree"] {
		opts.KeepWorktree = cli.KeepWorktree
	}
	if visited["gateway-method-count"] {
		opts.GatewayMethodCount = cli.GatewayMethodCount
	}
	if visited["gateway-methods"] {
		opts.GatewayMethods = append([]string(nil), cli.GatewayMethods...)
	}
	if visited["processors"] {
		opts.Processors = append([]string(nil), cli.Processors...)
	}
	if visited["histogram-buckets"] {
		opts.HistogramBuckets = cli.HistogramBuckets
	}
	if visited["histogram-type"] || visited["native-histograms"] {
		opts.HistogramType = cli.HistogramType
	}
	if visited["status-values"] {
		opts.StatusValues = cli.StatusValues
	}
	if visited["environments"] {
		opts.Environments = cli.Environments
		if !visited["environment"] {
			opts.EnvironmentNames = nil
		}
	}
	if visited["environment"] {
		opts.EnvironmentNames = append([]string(nil), cli.EnvironmentNames...)
		opts.Environments = len(cli.EnvironmentNames)
	}
	if visited["instances-per-service"] {
		opts.InstancesPerService = cli.InstancesPerService
	}
	if visited["instance-label"] {
		opts.InstanceLabelEnabled = cli.InstanceLabelEnabled
	}
	if visited["dimension"] {
		opts.CustomDimensions = append([]model.Dimension(nil), cli.CustomDimensions...)
	}
	if visited["service-instances"] || visited["service-environments"] {
		opts.ServiceOverrides = append(opts.ServiceOverrides, cli.ServiceOverrides...)
	}
	if visited["include-client-producer"] {
		opts.IncludeClientProducer = cli.IncludeClientProducer
	}
	if visited["grafana-query"] {
		opts.GrafanaQuery = cli.GrafanaQuery
	}
	return opts
}

func printSummary(r model.Report, htmlPath string, jsonPath string) {
	if r.Source.Type == source.TypeGit {
		ref := r.Source.ResolvedRef
		if len(ref) > 12 {
			ref = ref[:12]
		}
		fmt.Printf("Source: %s %s", r.Source.Provider, r.Source.Original)
		if ref != "" {
			fmt.Printf(" @ %s", ref)
		}
		if r.Source.WorktreeRetained {
			fmt.Printf(" (worktree kept at %s)", r.Source.ResolvedPath)
		}
		fmt.Println()
	}
	fmt.Printf("Scanned %d service(s), %d operation(s), %d service-graph edge(s).\n", len(r.Analysis.Services), len(r.Analysis.Operations), len(r.Analysis.Edges))
	fmt.Printf("Estimated active series: %d expected (%d low / %d high).\n", r.Estimate.TotalExpected, r.Estimate.TotalLow, r.Estimate.TotalHigh)
	if htmlPath != "" {
		fmt.Printf("HTML report: %s\n", displayPath(htmlPath))
	}
	if jsonPath != "" {
		fmt.Printf("JSON report: %s\n", displayPath(jsonPath))
	}
	if len(r.Analysis.Risks) > 0 {
		fmt.Printf("Detected %d cardinality or static-analysis risk(s).\n", len(r.Analysis.Risks))
	}
}

func displayPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func usage() {
	fmt.Fprintln(os.Stderr, "Grafana Cloud App O11y active-series estimator")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gco11y-size scan --repo <path-or-git-url> [--out report.html] [--json report.json]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  gco11y-size scan --repo ./checkout-service --dimension http.route=1")
	fmt.Fprintln(os.Stderr, "  gco11y-size scan --repo https://github.com/acme/checkout-service.git --ref main")
}

func splitGatewayMethods(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := map[string]bool{}
	var methods []string
	for _, part := range strings.Split(raw, ",") {
		method := strings.ToUpper(strings.TrimSpace(part))
		if method == "" || seen[method] {
			continue
		}
		seen[method] = true
		methods = append(methods, method)
	}
	return methods
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := map[string]bool{}
	var values []string
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

func cleanStringList(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
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

func parseEnabledDisabled(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "enabled", "enable", "true", "yes", "on", "1":
		return true, nil
	case "disabled", "disable", "false", "no", "off", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid enabled/disabled value %q", raw)
	}
}
