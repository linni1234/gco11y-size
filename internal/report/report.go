package report

import (
	"bytes"
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func WriteJSON(path string, report model.Report) error {
	if path == "" {
		return nil
	}
	if err := ensureParent(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func WriteWorkspaceJSON(path string, report model.WorkspaceReport) error {
	if path == "" {
		return nil
	}
	if err := ensureParent(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func WriteHTML(path string, report model.Report) error {
	if path == "" {
		return nil
	}
	if err := ensureParent(path); err != nil {
		return err
	}
	html, err := RenderHTML(report)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(html), 0o644)
}

func WriteWorkspaceHTML(path string, report model.WorkspaceReport) error {
	if path == "" {
		return nil
	}
	if err := ensureParent(path); err != nil {
		return err
	}
	html, err := RenderWorkspaceHTML(report)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(html), 0o644)
}

func RenderHTML(report model.Report) (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"bar": func(value int, total int) int {
			if total <= 0 || value <= 0 {
				return 0
			}
			width := int(float64(value) / float64(total) * 100)
			if width < 3 {
				return 3
			}
			if width > 100 {
				return 100
			}
			return width
		},
	}).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func ensureParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
