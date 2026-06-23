package framework

import "github.com/nilslindholm/metricgenerationsizer/internal/model"

type Analyzer interface {
	ID() string
	Analyze(Context) (Result, error)
}

type Context struct {
	Repo         string
	OtelConfig   string
	ServiceNames map[string]string
}

type Result struct {
	ServiceNames      map[string]string
	Services          []model.Service
	Operations        []model.Operation
	Edges             []model.Edge
	ConfigFindings    []model.ConfigFinding
	Risks             []model.Risk
	Warnings          []string
	DetectedLanguages []string
}
