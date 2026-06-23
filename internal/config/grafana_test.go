package config

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestQueryGrafanaCloud(t *testing.T) {
	previousClient := grafanaHTTPClient
	defer func() { grafanaHTTPClient = previousClient }()
	grafanaHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/query" {
			t.Fatalf("path = %s, want /api/v1/query", r.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"status":"success","data":{"result":[{"value":[1782226000,"123"]}]}}`)),
			Request:    r,
		}, nil
	})}

	t.Setenv("GRAFANA_CLOUD_PROM_URL", "http://grafana.example")
	t.Setenv("GRAFANA_CLOUD_PROM_USER", "user")
	t.Setenv("GRAFANA_CLOUD_PROM_TOKEN", "token")

	calibration := QueryGrafanaCloud(context.Background(), true)
	if !calibration.Configured || !calibration.Queried {
		t.Fatalf("calibration = %#v, want configured and queried", calibration)
	}
	if calibration.ObservedSeries != 123 {
		t.Fatalf("observed series = %d, want 123", calibration.ObservedSeries)
	}
}

func TestQueryGrafanaCloudOfflineByDefault(t *testing.T) {
	t.Setenv("GRAFANA_CLOUD_PROM_URL", "http://example.invalid")
	t.Setenv("GRAFANA_CLOUD_PROM_TOKEN", "token")
	calibration := QueryGrafanaCloud(context.Background(), false)
	if !calibration.Configured {
		t.Fatalf("calibration should be configured when env vars are set")
	}
	if calibration.Queried {
		t.Fatalf("calibration should not query unless enabled")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
