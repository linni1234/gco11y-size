package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	defaultProcessors := strings.Join(config.DefaultProcessors, ",")
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	repo := flags.String("repo", ".", "application repository path or Git URL to scan")
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
	environments := flags.Int("environments", 1, "number of deployment environments")
	instancesPerService := flags.Int("instances-per-service", 1, "expected service instances per environment for host info")
	instanceLabel := flags.String("instance-label", "enabled", "generated trace metrics instance label: enabled or disabled")
	includeClientProducer := flags.Bool("include-client-producer", false, "include CLIENT and PRODUCER span kinds in span-metric operation sizing")
	grafanaQuery := flags.Bool("grafana-query", false, "query Grafana Cloud Prometheus when GRAFANA_CLOUD_PROM_URL and token env vars are set")
	flags.Var(&dimensions, "dimension", "custom span dimension as name=cardinality; may be repeated")

	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gco11y-size scan --repo <path> [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	histogramTypeSet := false
	nativeHistogramsSet := false
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "histogram-type":
			histogramTypeSet = true
		case "native-histograms":
			nativeHistogramsSet = true
		}
	})
	resolvedHistogramType := config.NormalizeHistogramType(*histogramType)
	if nativeHistogramsSet && *nativeHistograms {
		if histogramTypeSet && resolvedHistogramType != config.HistogramTypeBoth {
			return fmt.Errorf("--native-histograms is deprecated; use --histogram-type=both instead")
		}
		resolvedHistogramType = config.HistogramTypeBoth
	}
	instanceLabelEnabled, err := parseEnabledDisabled(*instanceLabel)
	if err != nil {
		return fmt.Errorf("--instance-label must be enabled or disabled")
	}

	opts := model.Options{
		Repo:                  *repo,
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
		Environments:          *environments,
		InstancesPerService:   *instancesPerService,
		InstanceLabelEnabled:  instanceLabelEnabled,
		CustomDimensions:      append([]model.Dimension(nil), dimensions...),
		IncludeClientProducer: *includeClientProducer,
		GrafanaQuery:          *grafanaQuery,
	}
	if err := config.ValidateOptions(opts); err != nil {
		return err
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
		return err
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
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	calibration := config.QueryGrafanaCloud(ctx, opts.GrafanaQuery)
	estimate := estimator.Estimate(analysis, opts, calibration)
	finalReport := model.Report{
		Version:     model.Version,
		GeneratedAt: time.Now().UTC(),
		Options:     opts,
		Source:      resolvedSource.Source,
		Analysis:    analysis,
		Estimate:    estimate,
	}

	if *outJSON != "" {
		if err := report.WriteJSON(*outJSON, finalReport); err != nil {
			return err
		}
	}
	if *outHTML != "" {
		if err := report.WriteHTML(*outHTML, finalReport); err != nil {
			return err
		}
	}

	printSummary(finalReport, *outHTML, *outJSON)
	return nil
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
