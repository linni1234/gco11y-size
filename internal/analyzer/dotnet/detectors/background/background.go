package background

import (
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	dotnetcommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Operations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	operations = append(operations, hostedServiceOperations(ctx)...)
	operations = append(operations, jobOperations(ctx)...)
	operations = append(operations, orleansOperations(ctx)...)
	return operations
}

func ConfigFindings(ctx dotnetcommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, framework := range []struct {
		name string
		hint string
	}{
		{name: "hangfire", hint: "Hangfire"},
		{name: "quartz", hint: "Quartz"},
		{name: "orleans", hint: "Orleans"},
	} {
		if strings.Contains(ctx.Source, framework.hint) {
			findings = append(findings, model.ConfigFinding{Kind: "dotnet-background-framework", Name: framework.name, Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
		}
	}
	if strings.Contains(ctx.Source, "BackgroundService") || strings.Contains(ctx.Source, "IHostedService") {
		findings = append(findings, model.ConfigFinding{Kind: "dotnet-background-framework", Name: "hosted-service", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	return findings
}

func hostedServiceOperations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, class := range dotnetcommon.FindClasses(ctx.Source) {
		if !strings.Contains(class.Bases, "BackgroundService") && !strings.Contains(class.Bases, "IHostedService") {
			continue
		}
		route := "background:" + basecommon.SanitizeServiceName(class.Name)
		operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", route, class.Name, ctx.SourcePath, "dotnet-hosted-service", "medium"))
	}
	return operations
}

func jobOperations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, call := range dotnetcommon.Calls(ctx.Source, "AddOrUpdate", "Enqueue", "Schedule", "ScheduleJob", "AddJob") {
		switch {
		case strings.Contains(ctx.Source, "Hangfire") && (call.Name == "AddOrUpdate" || call.Name == "Enqueue" || call.Name == "Schedule"):
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", "hangfire:"+strings.ToLower(call.Name), "background job", ctx.SourcePath, "hangfire", "low"))
		case strings.Contains(ctx.Source, "Quartz") && (call.Name == "ScheduleJob" || call.Name == "AddJob"):
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", "quartz:"+strings.ToLower(call.Name), "background job", ctx.SourcePath, "quartz", "low"))
		}
	}
	return operations
}

func orleansOperations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, class := range dotnetcommon.FindClasses(ctx.Source) {
		if strings.Contains(class.Bases, "Grain") || strings.Contains(class.Bases, "IGrainWith") {
			route := "orleans:" + basecommon.SanitizeServiceName(class.Name)
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", route, class.Name, ctx.SourcePath, "orleans", "low"))
		}
	}
	for _, call := range dotnetcommon.Calls(ctx.Source, "SubscribeAsync", "OnNextAsync") {
		switch call.Name {
		case "SubscribeAsync":
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "CONSUMER", "orleans-stream", "MESSAGE", "orleans-stream:subscription", "Orleans stream subscription", ctx.SourcePath, "orleans", "low"))
		case "OnNextAsync":
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "PRODUCER", "orleans-stream", "MESSAGE", "orleans-stream:publish", "Orleans stream publish", ctx.SourcePath, "orleans", "low"))
		}
	}
	return operations
}
