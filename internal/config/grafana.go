package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

const defaultActiveSeriesQuery = `count({__name__=~"traces_spanmetrics.*|traces_service_graph.*|traces_target_info|target_info"})`

var grafanaHTTPClient = &http.Client{Timeout: 12 * time.Second}

func QueryGrafanaCloud(ctx context.Context, enabled bool) *model.CloudCalibration {
	promURL := strings.TrimRight(os.Getenv("GRAFANA_CLOUD_PROM_URL"), "/")
	user := os.Getenv("GRAFANA_CLOUD_PROM_USER")
	token := os.Getenv("GRAFANA_CLOUD_PROM_TOKEN")

	calibration := &model.CloudCalibration{
		Configured: promURL != "" && token != "",
		Queried:    false,
		Query:      defaultActiveSeriesQuery,
		Source:     promURL,
	}
	if !calibration.Configured {
		calibration.Message = "Grafana Cloud Prometheus environment variables were not fully configured."
		return calibration
	}
	if !enabled {
		calibration.Message = "Grafana Cloud credentials were detected, but --grafana-query was not enabled."
		return calibration
	}

	endpoint, err := url.Parse(promURL + "/api/v1/query")
	if err != nil {
		calibration.Message = fmt.Sprintf("invalid GRAFANA_CLOUD_PROM_URL: %v", err)
		return calibration
	}
	values := endpoint.Query()
	values.Set("query", defaultActiveSeriesQuery)
	endpoint.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		calibration.Message = fmt.Sprintf("failed to build Grafana query: %v", err)
		return calibration
	}
	if user != "" {
		req.SetBasicAuth(user, token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := grafanaHTTPClient.Do(req)
	if err != nil {
		calibration.Message = fmt.Sprintf("Grafana Cloud query failed: %v", err)
		return calibration
	}
	defer resp.Body.Close()

	calibration.Queried = true
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		calibration.Message = fmt.Sprintf("Grafana Cloud query returned HTTP %d", resp.StatusCode)
		return calibration
	}

	var body prometheusQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		calibration.Message = fmt.Sprintf("Grafana Cloud query response was not valid JSON: %v", err)
		return calibration
	}
	if body.Status != "success" || len(body.Data.Result) == 0 || len(body.Data.Result[0].Value) < 2 {
		calibration.Message = "Grafana Cloud query succeeded but did not return an active-series value."
		return calibration
	}
	raw, ok := body.Data.Result[0].Value[1].(string)
	if !ok {
		calibration.Message = "Grafana Cloud query returned an unexpected value format."
		return calibration
	}
	observed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		calibration.Message = fmt.Sprintf("Grafana Cloud active-series value %q was not numeric", raw)
		return calibration
	}
	calibration.ObservedSeries = int(observed)
	calibration.ObservedAt = time.Now().UTC()
	calibration.Message = "Grafana Cloud active-series calibration query completed."
	return calibration
}

type prometheusQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value []any `json:"value"`
		} `json:"result"`
	} `json:"data"`
}
