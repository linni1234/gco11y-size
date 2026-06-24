package report

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func RenderWorkspaceHTML(report model.WorkspaceReport) (string, error) {
	tmpl, err := template.New("workspace-report").Funcs(template.FuncMap{
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
		"envLabel": func(opts model.Options) string {
			if len(opts.EnvironmentNames) > 0 {
				return strings.Join(opts.EnvironmentNames, ", ")
			}
			return fmt.Sprintf("%d", opts.Environments)
		},
		"safeID":       safeID,
		"sharedStyles": sharedReportStyles,
	}).Parse(workspaceHTMLTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func safeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "repo"
	}
	return value
}

func sharedReportStyles() template.CSS {
	start := strings.Index(htmlTemplate, "<style>")
	end := strings.Index(htmlTemplate, "</style>")
	if start < 0 || end <= start {
		return ""
	}
	return template.CSS(htmlTemplate[start+len("<style>") : end])
}

const workspaceHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Grafana Cloud App O11y Workspace Estimate</title>
  <style>
{{ sharedStyles }}
    .workspace-view { display: none; }
    .workspace-view.active { display: block; }
    .workspace-nav-group {
      display: grid;
      gap: 4px;
    }
    .workspace-subnav {
      display: none;
      gap: 3px;
      margin: -2px 0 8px 12px;
      padding-left: 10px;
      border-left: 1px solid var(--rail-panel-border);
    }
    .workspace-nav-group.active .workspace-subnav {
      display: grid;
    }
    .workspace-subnav a {
      min-height: 30px;
      padding-left: 10px;
      font-size: 12px;
      font-weight: 520;
    }
    .rail-nav a.active {
      color: var(--rail-link-hover);
      background: var(--rail-link-hover-bg);
      border-color: var(--rail-link-hover-border);
    }
  </style>
</head>
<body data-theme="dark">
  <div class="report-shell">
    <aside class="side-rail" aria-label="Report navigation">
      <button class="theme-toggle" type="button" id="themeToggle" aria-pressed="true">
        <span id="themeToggleText">Light mode</span>
        <span class="theme-toggle-indicator" aria-hidden="true"><span class="theme-toggle-dot"></span></span>
      </button>
      <div class="rail-brand">
        <span class="brand-mark"></span>
        <span>Grafana Cloud App O11y Workspace Estimate</span>
      </div>
      <div class="rail-meta">
        <strong>Static application workspace sizing report</strong>
        <span>Generated {{ .GeneratedAt.Format "2006-01-02 15:04:05 UTC" }}</span>
      </div>
      <nav class="rail-nav">
        <div class="workspace-nav-group active" data-workspace-group="overview">
          <a class="workspace-tab workspace-main-tab active" href="#overview" data-workspace-tab="overview">Overview</a>
          <div class="workspace-subnav">
            <a class="workspace-section-link" href="#repositories" data-workspace-tab="overview" data-workspace-section="repositories">Repositories</a>
            <a class="workspace-section-link" href="#aggregate-processors" data-workspace-tab="overview" data-workspace-section="aggregate-processors">Processors</a>
            <a class="workspace-section-link" href="#aggregate-services" data-workspace-tab="overview" data-workspace-section="aggregate-services">Top Services</a>
            <a class="workspace-section-link" href="#aggregate-operations" data-workspace-tab="overview" data-workspace-section="aggregate-operations">Top Operations</a>
            <a class="workspace-section-link" href="#aggregate-graph" data-workspace-tab="overview" data-workspace-section="aggregate-graph">Service Graph</a>
          </div>
        </div>
        {{ range .Repositories }}
        <div class="workspace-nav-group" data-workspace-group="repo-{{ safeID .Name }}">
          <a class="workspace-tab workspace-main-tab" href="#repo-{{ safeID .Name }}" data-workspace-tab="repo-{{ safeID .Name }}">{{ .Name }}</a>
          <div class="workspace-subnav">
            <a class="workspace-section-link" href="#repo-{{ safeID .Name }}" data-workspace-tab="repo-{{ safeID .Name }}" data-workspace-section="repo-{{ safeID .Name }}">Summary</a>
            <a class="workspace-section-link" href="#repo-{{ safeID .Name }}-source" data-workspace-tab="repo-{{ safeID .Name }}" data-workspace-section="repo-{{ safeID .Name }}-source">Source</a>
            <a class="workspace-section-link" href="#repo-{{ safeID .Name }}-processors" data-workspace-tab="repo-{{ safeID .Name }}" data-workspace-section="repo-{{ safeID .Name }}-processors">Processors</a>
            <a class="workspace-section-link" href="#repo-{{ safeID .Name }}-services" data-workspace-tab="repo-{{ safeID .Name }}" data-workspace-section="repo-{{ safeID .Name }}-services">Services</a>
            <a class="workspace-section-link" href="#repo-{{ safeID .Name }}-operations" data-workspace-tab="repo-{{ safeID .Name }}" data-workspace-section="repo-{{ safeID .Name }}-operations">Top Operations</a>
            <a class="workspace-section-link" href="#repo-{{ safeID .Name }}-graph" data-workspace-tab="repo-{{ safeID .Name }}" data-workspace-section="repo-{{ safeID .Name }}-graph">Service Graph</a>
          </div>
        </div>
        {{ end }}
      </nav>
      <div class="rail-foot">Workspace estimate. Per-repo details are preserved while aggregate totals dedupe repeated services, operations, and edges.</div>
    </aside>

    <div class="report-content">
      <main>
        <div class="workspace-view active" id="overview" data-workspace-view="overview">
          <section class="hero-band">
            <div class="hero-grid">
              <div class="hero-copy">
                <span class="eyebrow">Workspace active series</span>
                <h1>{{ .Aggregate.Estimate.TotalExpected }}</h1>
                <p>{{ .Aggregate.Estimate.TotalLow }} low / {{ .Aggregate.Estimate.TotalHigh }} high across {{ len .Repositories }} repo(s). Source code can forecast metric cardinality, while exact active series still depend on runtime traces, label values, enabled dimensions, and metrics-generator configuration.</p>
              </div>
              <aside class="summary-panel" aria-label="Workspace summary">
                <p>Scanned {{ len .Repositories }} repo(s), {{ len .Aggregate.Analysis.Services }} service(s), {{ len .Aggregate.Analysis.Operations }} operation(s), and {{ len .Aggregate.Analysis.Edges }} service-graph edge(s) using the {{ .Options.Profile }} profile.</p>
                <div class="summary-grid">
                  <div class="summary-card"><span class="summary-label">Histogram type</span><strong>{{ .Options.HistogramType }}</strong></div>
                  <div class="summary-card"><span class="summary-label">Environments</span><strong>{{ envLabel .Options }}</strong></div>
                  <div class="summary-card"><span class="summary-label">Instances / service</span><strong>{{ .Options.InstancesPerService }}</strong></div>
                  <div class="summary-card"><span class="summary-label">Status values</span><strong>{{ .Options.StatusValues }}</strong></div>
                </div>
              </aside>
            </div>
          </section>

          <section class="stat-band" aria-label="Workspace sizing summary">
            <div class="stat-item"><span class="stat-label">Repositories</span><strong>{{ len .Repositories }}</strong></div>
            <div class="stat-item"><span class="stat-label">Services</span><strong>{{ len .Aggregate.Analysis.Services }}</strong></div>
            <div class="stat-item"><span class="stat-label">Operations</span><strong>{{ len .Aggregate.Analysis.Operations }}</strong></div>
            <div class="stat-item"><span class="stat-label">Service graph edges</span><strong>{{ len .Aggregate.Analysis.Edges }}</strong></div>
          </section>

        <section class="content-section" id="repositories">
          <div class="section-heading">
            <div>
              <span class="eyebrow">Workspace overview</span>
              <h2>Repository Overview</h2>
            </div>
            <span class="section-count">{{ len .Repositories }} repos</span>
          </div>
          <div class="table-card">
            <div class="table-scroll">
              <table>
                <thead><tr><th>Repository</th><th>Source</th><th class="numeric">Services</th><th class="numeric">Operations</th><th class="numeric">Edges</th><th class="numeric">Expected</th></tr></thead>
                <tbody>
                  {{ range .Repositories }}
                  <tr>
                    <td><a class="pill workspace-tab" href="#repo-{{ safeID .Name }}" data-workspace-tab="repo-{{ safeID .Name }}">{{ .Name }}</a></td>
                    <td>{{ .Report.Source.Original }}</td>
                    <td class="numeric">{{ len .Report.Analysis.Services }}</td>
                    <td class="numeric">{{ len .Report.Analysis.Operations }}</td>
                    <td class="numeric">{{ len .Report.Analysis.Edges }}</td>
                    <td class="numeric">{{ .Report.Estimate.TotalExpected }}</td>
                  </tr>
                  {{ end }}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <section class="content-section" id="aggregate-processors">
          <div class="section-heading">
            <div>
              <span class="eyebrow">Generated metrics</span>
              <h2>Aggregate Processor Breakdown</h2>
            </div>
            <span class="section-count">workspace total</span>
          </div>
          <div class="table-card">
            <div class="table-scroll">
              <table>
                <thead><tr><th>Processor</th><th class="numeric">Low</th><th class="numeric">Expected</th><th class="numeric">High</th><th>Formula</th></tr></thead>
                <tbody>
                  {{ range .Aggregate.Estimate.ProcessorBreakdown }}
                  <tr><td><code>{{ .Processor }}</code></td><td class="numeric">{{ .Low }}</td><td class="numeric">{{ .Expected }}</td><td class="numeric">{{ .High }}</td><td>{{ .Formula }}</td></tr>
                  {{ end }}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <section class="content-section" id="aggregate-services">
          <div class="section-heading">
            <div>
              <span class="eyebrow">Aggregate contributors</span>
              <h2>Top Service Contributors</h2>
            </div>
            <span class="section-count">{{ len .Aggregate.Estimate.ServiceBreakdown }} services</span>
          </div>
          {{ if .Aggregate.Estimate.ServiceBreakdown }}
          <div class="chart-card">
            <div class="bar-list">
              {{ range .Aggregate.Estimate.ServiceBreakdown }}
              <div class="usage-bar">
                <div class="bar-meta"><span class="bar-name">{{ .Service }}{{ if .Repository }} <span class="chip">{{ .Repository }}</span>{{ end }}</span><span class="bar-value">{{ .Expected }}</span></div>
                <span class="bar-track"><span class="bar-fill" style="width: {{ bar .Expected $.Aggregate.Estimate.TotalExpected }}%"></span></span>
              </div>
              {{ end }}
            </div>
          </div>
          {{ else }}
          <div class="empty-state">No aggregate service contributor data was produced.</div>
          {{ end }}
        </section>

        <section class="content-section" id="aggregate-operations">
          <div class="section-heading">
            <div>
              <span class="eyebrow">Aggregate span metrics</span>
              <h2>Top Operation Contributors</h2>
            </div>
            <span class="section-count">{{ len .Aggregate.Estimate.OperationContributors }} operations</span>
          </div>
          {{ if .Aggregate.Estimate.OperationContributors }}
          <div class="chart-card">
            <div class="bar-list">
              {{ range .Aggregate.Estimate.OperationContributors }}
              <div class="usage-bar">
                <div class="bar-meta"><span class="bar-name">{{ .Service }} <code>{{ .Protocol }}</code> {{ .Method }} {{ .Route }}</span><span class="bar-value">{{ .Expected }}</span></div>
                <span class="bar-track"><span class="bar-fill" style="width: {{ bar .Expected $.Aggregate.Estimate.TotalExpected }}%"></span></span>
              </div>
              {{ end }}
            </div>
          </div>
          {{ else }}
          <div class="empty-state">No operation contributors were produced.</div>
          {{ end }}
        </section>

        <section class="content-section" id="aggregate-graph">
          <div class="section-heading">
            <div>
              <span class="eyebrow">Aggregate dependency hints</span>
              <h2>Workspace Service Graph</h2>
            </div>
            <span class="section-count">{{ len .Aggregate.Analysis.Edges }} edges</span>
          </div>
          {{ if .Aggregate.Analysis.Edges }}
          <div class="table-card">
            <div class="table-scroll">
              <table>
                <thead><tr><th>Source</th><th>Target</th><th>Protocol</th><th>Repository</th><th>Confidence</th></tr></thead>
                <tbody>
                  {{ range .Aggregate.Analysis.Edges }}
                  <tr><td><code>{{ .SourceService }}</code></td><td><code>{{ .TargetService }}</code></td><td>{{ .Protocol }}</td><td>{{ .Repository }}</td><td>{{ .Confidence }}</td></tr>
                  {{ end }}
                </tbody>
              </table>
            </div>
          </div>
          {{ else }}
          <div class="empty-state">No aggregate service graph edges were detected.</div>
          {{ end }}
        </section>
        </div>

        {{ range .Repositories }}
        <section class="workspace-view content-section" id="repo-{{ safeID .Name }}" data-workspace-view="repo-{{ safeID .Name }}">
          <div class="section-heading">
            <div>
              <span class="eyebrow">Repository drilldown</span>
              <h2>{{ .Name }}</h2>
            </div>
            <span class="section-count">{{ .Report.Estimate.TotalExpected }} expected series</span>
          </div>

          <div class="stat-band" aria-label="{{ .Name }} summary">
            <div class="stat-item"><span class="stat-label">Services</span><strong>{{ len .Report.Analysis.Services }}</strong></div>
            <div class="stat-item"><span class="stat-label">Operations</span><strong>{{ len .Report.Analysis.Operations }}</strong></div>
            <div class="stat-item"><span class="stat-label">Service graph edges</span><strong>{{ len .Report.Analysis.Edges }}</strong></div>
            <div class="stat-item"><span class="stat-label">Active series</span><strong>{{ .Report.Estimate.TotalExpected }}</strong></div>
          </div>

          <div class="detail-panel" id="repo-{{ safeID .Name }}-source" style="margin-top: 16px;">
            <div class="table-scroll">
              <table>
                <tbody>
                  <tr><th>Input</th><td>{{ .Report.Source.Original }}</td></tr>
                  <tr><th>Type</th><td>{{ .Report.Source.Type }}</td></tr>
                  <tr><th>Resolved path</th><td>{{ .Report.Source.ResolvedPath }}</td></tr>
                  <tr><th>Histogram type</th><td>{{ .Report.Options.HistogramType }}</td></tr>
                  <tr><th>Environments</th><td>{{ envLabel .Report.Options }}</td></tr>
                  <tr><th>Instances / service</th><td>{{ .Report.Options.InstancesPerService }}</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="section-heading" id="repo-{{ safeID .Name }}-processors" style="margin-top: 28px;">
            <div>
              <span class="eyebrow">Generated metrics</span>
              <h2>{{ .Name }} Processors</h2>
            </div>
          </div>
          <div class="table-card">
            <div class="table-scroll">
              <table>
                <thead><tr><th>Processor</th><th class="numeric">Low</th><th class="numeric">Expected</th><th class="numeric">High</th><th>Formula</th></tr></thead>
                <tbody>
                  {{ range .Report.Estimate.ProcessorBreakdown }}
                  <tr><td><code>{{ .Processor }}</code></td><td class="numeric">{{ .Low }}</td><td class="numeric">{{ .Expected }}</td><td class="numeric">{{ .High }}</td><td>{{ .Formula }}</td></tr>
                  {{ end }}
                </tbody>
              </table>
            </div>
          </div>

          <div class="section-heading" id="repo-{{ safeID .Name }}-services" style="margin-top: 28px;">
            <div>
              <span class="eyebrow">Detected services</span>
              <h2>{{ .Name }} Services</h2>
            </div>
            <span class="section-count">{{ len .Report.Analysis.Services }} services</span>
          </div>
          {{ if .Report.Analysis.Services }}
          <div class="table-card">
            <div class="table-scroll">
              <table>
                <thead><tr><th>Service</th><th>Root</th><th>Source</th><th class="numeric">Operations</th><th class="numeric">Edges</th></tr></thead>
                <tbody>
                  {{ range .Report.Analysis.Services }}
                  <tr><td><code>{{ .Name }}</code></td><td>{{ .Root }}</td><td>{{ .Source }}</td><td class="numeric">{{ .OperationCount }}</td><td class="numeric">{{ .EdgeCount }}</td></tr>
                  {{ end }}
                </tbody>
              </table>
            </div>
          </div>
          {{ else }}
          <div class="empty-state">No services were detected for this repository.</div>
          {{ end }}

          <div class="section-heading" id="repo-{{ safeID .Name }}-operations" style="margin-top: 28px;">
            <div>
              <span class="eyebrow">Span metric contributors</span>
              <h2>{{ .Name }} Top Operations</h2>
            </div>
          </div>
          {{ if .Report.Estimate.OperationContributors }}
          <div class="chart-card">
            <div class="bar-list">
              {{ range .Report.Estimate.OperationContributors }}
              <div class="usage-bar">
                <div class="bar-meta"><span class="bar-name">{{ .Service }} <code>{{ .Protocol }}</code> {{ .Method }} {{ .Route }}</span><span class="bar-value">{{ .Expected }}</span></div>
                <span class="bar-track"><span class="bar-fill" style="width: {{ bar .Expected $.Aggregate.Estimate.TotalExpected }}%"></span></span>
              </div>
              {{ end }}
            </div>
          </div>
          {{ else }}
          <div class="empty-state">No operation contributors were produced for this repository.</div>
          {{ end }}

          <div class="section-heading" id="repo-{{ safeID .Name }}-graph" style="margin-top: 28px;">
            <div>
              <span class="eyebrow">Static dependency hints</span>
              <h2>{{ .Name }} Service Graph</h2>
            </div>
            <span class="section-count">{{ len .Report.Analysis.Edges }} edges</span>
          </div>
          {{ if .Report.Analysis.Edges }}
          <div class="table-card">
            <div class="table-scroll">
              <table>
                <thead><tr><th>Source</th><th>Target</th><th>Protocol</th><th>Confidence</th><th>Source file</th></tr></thead>
                <tbody>
                  {{ range .Report.Analysis.Edges }}
                  <tr><td><code>{{ .SourceService }}</code></td><td><code>{{ .TargetService }}</code></td><td>{{ .Protocol }}</td><td>{{ .Confidence }}</td><td>{{ .Source }}</td></tr>
                  {{ end }}
                </tbody>
              </table>
            </div>
          </div>
          {{ else }}
          <div class="empty-state">No service graph edges were detected for this repository.</div>
          {{ end }}
        </section>
        {{ end }}
      </main>
    </div>
  </div>
  <script>
    (function () {
      var storageKey = "gco11y-size-theme";
      var button = document.getElementById("themeToggle");
      var label = document.getElementById("themeToggleText");

      function prepareResponsiveTables() {
        var tables = document.querySelectorAll("table");
        for (var tableIndex = 0; tableIndex < tables.length; tableIndex += 1) {
          var table = tables[tableIndex];
          var headerCells = table.querySelectorAll("thead th");
          if (!headerCells.length) {
            continue;
          }
          table.classList.add("responsive-table");
          var labels = [];
          for (var headerIndex = 0; headerIndex < headerCells.length; headerIndex += 1) {
            labels.push(headerCells[headerIndex].textContent.trim());
          }
          var rows = table.querySelectorAll("tbody tr");
          for (var rowIndex = 0; rowIndex < rows.length; rowIndex += 1) {
            var cells = rows[rowIndex].children;
            for (var cellIndex = 0; cellIndex < cells.length; cellIndex += 1) {
              cells[cellIndex].setAttribute("data-label", labels[cellIndex] || "Value");
            }
          }
        }
      }

      function applyTheme(theme) {
        document.body.setAttribute("data-theme", theme);
        if (button) {
          button.setAttribute("aria-pressed", theme === "dark" ? "true" : "false");
        }
        if (label) {
          label.textContent = theme === "dark" ? "Light mode" : "Dark mode";
        }
      }

      function scrollToWorkspaceSection(sectionId) {
        var section = document.getElementById(sectionId);
        if (section) {
          section.scrollIntoView({ behavior: "smooth", block: "start" });
        } else {
          window.scrollTo({ top: 0, behavior: "smooth" });
        }
      }

      function showWorkspaceView(viewId, updateHash, sectionId) {
        var target = document.querySelector('[data-workspace-view="' + viewId + '"]');
        if (!target) {
          viewId = "overview";
          target = document.querySelector('[data-workspace-view="overview"]');
        }
        sectionId = sectionId || viewId;
        var views = document.querySelectorAll("[data-workspace-view]");
        for (var viewIndex = 0; viewIndex < views.length; viewIndex += 1) {
          views[viewIndex].classList.toggle("active", views[viewIndex] === target);
        }
        var tabs = document.querySelectorAll(".workspace-main-tab");
        for (var tabIndex = 0; tabIndex < tabs.length; tabIndex += 1) {
          tabs[tabIndex].classList.toggle("active", tabs[tabIndex].getAttribute("data-workspace-tab") === viewId);
        }
        var groups = document.querySelectorAll("[data-workspace-group]");
        for (var groupIndex = 0; groupIndex < groups.length; groupIndex += 1) {
          groups[groupIndex].classList.toggle("active", groups[groupIndex].getAttribute("data-workspace-group") === viewId);
        }
        var sectionLinks = document.querySelectorAll("[data-workspace-section]");
        for (var sectionIndex = 0; sectionIndex < sectionLinks.length; sectionIndex += 1) {
          sectionLinks[sectionIndex].classList.toggle("active", sectionLinks[sectionIndex].getAttribute("data-workspace-section") === sectionId);
        }
        if (updateHash && window.history && window.history.replaceState) {
          window.history.replaceState(null, "", "#" + sectionId);
        }
        if (sectionId && sectionId !== viewId) {
          setTimeout(function () {
            scrollToWorkspaceSection(sectionId);
          }, 0);
        } else {
          window.scrollTo({ top: 0, behavior: "smooth" });
        }
      }

      function prepareWorkspaceTabs() {
        var tabs = document.querySelectorAll("[data-workspace-tab]");
        for (var tabIndex = 0; tabIndex < tabs.length; tabIndex += 1) {
          tabs[tabIndex].addEventListener("click", function (event) {
            event.preventDefault();
            showWorkspaceView(this.getAttribute("data-workspace-tab"), true, this.getAttribute("data-workspace-section") || "");
          });
        }
        var initialHash = (window.location.hash || "#overview").slice(1) || "overview";
        var initialView = initialHash;
        var initialSection = "";
        var hashTarget = document.getElementById(initialHash);
        if (hashTarget) {
          var view = hashTarget.closest("[data-workspace-view]");
          if (view) {
            initialView = view.getAttribute("data-workspace-view") || initialView;
            initialSection = initialHash;
          }
        }
        showWorkspaceView(initialView || "overview", false, initialSection);
      }

      var storedTheme = "";
      try {
        storedTheme = localStorage.getItem(storageKey) || "";
      } catch (error) {}

      prepareResponsiveTables();
      prepareWorkspaceTabs();
      applyTheme(storedTheme === "light" ? "light" : "dark");

      if (button) {
        button.addEventListener("click", function () {
          var nextTheme = document.body.getAttribute("data-theme") === "dark" ? "light" : "dark";
          applyTheme(nextTheme);
          try {
            localStorage.setItem(storageKey, nextTheme);
          } catch (error) {}
        });
      }
    })();
  </script>
</body>
</html>`
