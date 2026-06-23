package report

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Grafana Cloud App O11y Series Estimate</title>
  <style>
    :root {
      color-scheme: dark;
      --grafana-orange: #ff5a1f;
      --grafana-orange-dark: #ff8a5b;
      --grafana-blue: #6ea2ff;
      --ink: #f4f7fb;
      --muted: #aab4c6;
      --soft-muted: #7f8ba3;
      --line: rgba(255, 255, 255, 0.14);
      --line-soft: rgba(255, 255, 255, 0.08);
      --panel: #151a27;
      --panel-tint: #1b2232;
      --canvas: #111827;
      --code: #080d16;
      --green: #40d39b;
      --red: #ff7b7b;
      --yellow: #ffca70;
      --shadow: 0 18px 48px rgba(0, 0, 0, 0.3);
      --shadow-soft: 0 10px 28px rgba(0, 0, 0, 0.24);
      --shadow-card: 0 14px 38px rgba(0, 0, 0, 0.26);
      --hero-shadow: 0 24px 70px rgba(19, 23, 35, 0.2);
      --summary-shadow: 0 18px 50px rgba(0, 0, 0, 0.16);
      --bar-hover-shadow: 0 10px 22px rgba(0, 0, 0, 0.24);
      --note-shadow: 0 6px 18px rgba(0, 0, 0, 0.14);
      --page-bg: linear-gradient(135deg, #111827 0%, #1c263f 48%, #123e83 140%), #111827;
      --rail-bg: linear-gradient(180deg, #111827 0%, #172137 58%, #101827 100%);
      --rail-border: rgba(255, 255, 255, 0.1);
      --rail-shadow: 14px 0 40px rgba(19, 23, 35, 0.14);
      --rail-text: #d9deea;
      --rail-strong: #ffffff;
      --rail-muted: #aab2c5;
      --rail-panel-bg: rgba(255, 255, 255, 0.06);
      --rail-panel-border: rgba(255, 255, 255, 0.1);
      --rail-link: #c4cada;
      --rail-link-hover: #ffffff;
      --rail-link-hover-bg: rgba(255, 90, 31, 0.14);
      --rail-link-hover-border: rgba(255, 90, 31, 0.26);
      --hero-bg: linear-gradient(135deg, rgba(17, 24, 39, 0.98), rgba(28, 38, 63, 0.94) 46%, rgba(18, 62, 131, 0.9)), #111827;
      --hero-border: rgba(255, 255, 255, 0.12);
      --hero-grid-line: rgba(255, 255, 255, 0.07);
      --hero-title: linear-gradient(90deg, #ffffff, #9cc4ff 58%, #ffb28f);
      --hero-text: #c6cedd;
      --eyebrow-bg: rgba(255, 90, 31, 0.18);
      --eyebrow-text: #ffd2c2;
      --eyebrow-border: rgba(255, 90, 31, 0.28);
      --summary-panel-bg: linear-gradient(180deg, rgba(255, 255, 255, 0.12), rgba(255, 255, 255, 0.07));
      --summary-panel-border: rgba(255, 255, 255, 0.16);
      --summary-card-bg: rgba(255, 255, 255, 0.08);
      --summary-card-border: rgba(255, 255, 255, 0.12);
      --summary-card-inset: rgba(255, 255, 255, 0.08);
      --card-bg: linear-gradient(180deg, rgba(23, 29, 43, 0.98), rgba(16, 22, 35, 0.98));
      --stat-bg: linear-gradient(180deg, rgba(27, 34, 50, 0.98), rgba(19, 25, 38, 0.98));
      --subtle-row-bg: rgba(255, 255, 255, 0.025);
      --row-hover-bg: rgba(255, 255, 255, 0.035);
      --table-text: #dbe3f2;
      --code-text: #a9c8ff;
      --code-bg: rgba(110, 162, 255, 0.13);
      --code-border: rgba(110, 162, 255, 0.18);
      --bar-row-bg: linear-gradient(180deg, rgba(255, 255, 255, 0.045), rgba(255, 255, 255, 0.022));
      --bar-hover-border: rgba(110, 162, 255, 0.26);
      --bar-track-bg: rgba(255, 255, 255, 0.08);
      --chip-text: #dce5f5;
      --chip-bg: rgba(255, 255, 255, 0.08);
      --chip-border: rgba(255, 255, 255, 0.1);
      --note-bg: rgba(255, 255, 255, 0.045);
      --theme-toggle-bg: rgba(255, 255, 255, 0.08);
      --theme-toggle-border: rgba(255, 255, 255, 0.14);
      --theme-toggle-text: #ffffff;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    body[data-theme="light"] {
      color-scheme: light;
      --grafana-orange-dark: #c23a0d;
      --grafana-blue: #246bfe;
      --ink: #10131f;
      --muted: #6e7387;
      --soft-muted: #8d92a7;
      --line: #dfe2eb;
      --line-soft: #edf0f6;
      --panel: #ffffff;
      --panel-tint: #fbfcff;
      --canvas: #f7f7fb;
      --green: #0a8f63;
      --red: #b33a3a;
      --yellow: #a86600;
      --shadow: 0 18px 48px rgba(19, 23, 35, 0.1);
      --shadow-soft: 0 10px 28px rgba(19, 23, 35, 0.07);
      --shadow-card: 0 14px 38px rgba(19, 23, 35, 0.08);
      --hero-shadow: 0 24px 70px rgba(19, 23, 35, 0.1);
      --summary-shadow: 0 18px 44px rgba(19, 23, 35, 0.08);
      --bar-hover-shadow: 0 10px 22px rgba(19, 23, 35, 0.08);
      --note-shadow: 0 6px 18px rgba(19, 23, 35, 0.06);
      --page-bg: linear-gradient(180deg, #f7f7fb 0%, #ffffff 44%, #f5f6fb 100%), #f7f7fb;
      --rail-bg: linear-gradient(180deg, #ffffff 0%, #fbfcff 64%, #f3f6fb 100%);
      --rail-border: #dfe2eb;
      --rail-shadow: 12px 0 34px rgba(19, 23, 35, 0.08);
      --rail-text: #343b4f;
      --rail-strong: #10131f;
      --rail-muted: #6e7387;
      --rail-panel-bg: #f7f8fc;
      --rail-panel-border: #edf0f6;
      --rail-link: #596174;
      --rail-link-hover: #c23a0d;
      --rail-link-hover-bg: rgba(255, 90, 31, 0.1);
      --rail-link-hover-border: rgba(255, 90, 31, 0.24);
      --hero-bg: linear-gradient(115deg, rgba(255, 115, 56, 0.24), rgba(255, 255, 255, 0.86) 44%, rgba(79, 209, 220, 0.3)), linear-gradient(180deg, #ffffff, #f5f6fb);
      --hero-border: #dfe2eb;
      --hero-grid-line: rgba(16, 19, 31, 0.045);
      --hero-title: linear-gradient(90deg, #10131f, #246bfe 58%, #ff5a1f);
      --hero-text: #6e7387;
      --eyebrow-bg: rgba(255, 90, 31, 0.09);
      --eyebrow-text: #c23a0d;
      --eyebrow-border: rgba(255, 90, 31, 0.18);
      --summary-panel-bg: linear-gradient(180deg, rgba(255, 255, 255, 0.95), rgba(251, 252, 255, 0.92));
      --summary-panel-border: rgba(223, 226, 235, 0.9);
      --summary-card-bg: #fbfcff;
      --summary-card-border: #edf0f6;
      --summary-card-inset: rgba(255, 255, 255, 0.75);
      --card-bg: linear-gradient(180deg, rgba(255, 255, 255, 0.99), rgba(251, 252, 255, 0.99));
      --stat-bg: #ffffff;
      --subtle-row-bg: #fbfcff;
      --row-hover-bg: #f7f9ff;
      --table-text: #32384a;
      --code-text: #123e83;
      --code-bg: #edf3ff;
      --code-border: rgba(36, 107, 254, 0.16);
      --bar-row-bg: linear-gradient(180deg, #ffffff, #fbfcff);
      --bar-hover-border: rgba(36, 107, 254, 0.24);
      --bar-track-bg: #eef1f7;
      --chip-text: #3b4255;
      --chip-bg: #eef1f7;
      --chip-border: #edf0f6;
      --note-bg: rgba(255, 255, 255, 0.96);
      --theme-toggle-bg: #ffffff;
      --theme-toggle-border: #dfe2eb;
      --theme-toggle-text: #10131f;
    }
    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body {
      margin: 0;
      color: var(--ink);
      background: var(--page-bg);
    }
    ::selection {
      color: #ffffff;
      background: var(--grafana-blue);
    }
    a { color: inherit; text-decoration: none; }
    .report-shell {
      display: grid;
      grid-template-columns: minmax(220px, 17vw) minmax(0, 1fr);
      min-height: 100vh;
    }
    .side-rail {
      position: sticky;
      top: 0;
      display: flex;
      flex-direction: column;
      gap: 26px;
      height: 100vh;
      padding: 24px 20px;
      color: var(--rail-text);
      background: var(--rail-bg);
      border-right: 1px solid var(--rail-border);
      box-shadow: var(--rail-shadow);
    }
    .theme-toggle {
      display: inline-flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      width: 100%;
      min-height: 40px;
      padding: 0 12px;
      color: var(--theme-toggle-text);
      background: var(--theme-toggle-bg);
      border: 1px solid var(--theme-toggle-border);
      border-radius: 8px;
      cursor: pointer;
      font: inherit;
      font-size: 13px;
      font-weight: 650;
      transition:
        border-color 160ms ease,
        box-shadow 160ms ease,
        transform 160ms ease;
    }
    .theme-toggle:hover {
      border-color: var(--rail-link-hover-border);
      box-shadow: var(--shadow-soft);
      transform: translateY(-1px);
    }
    .theme-toggle:focus-visible {
      outline: 2px solid var(--grafana-blue);
      outline-offset: 2px;
    }
    .theme-toggle-indicator {
      display: inline-flex;
      align-items: center;
      justify-content: flex-start;
      width: 36px;
      height: 20px;
      padding: 3px;
      background: var(--bar-track-bg);
      border: 1px solid var(--line-soft);
      border-radius: 999px;
    }
    .theme-toggle-dot {
      width: 12px;
      height: 12px;
      background: linear-gradient(135deg, var(--grafana-orange), var(--grafana-blue));
      border-radius: 50%;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.22);
      transition: transform 160ms ease;
    }
    body[data-theme="light"] .theme-toggle-indicator {
      justify-content: flex-end;
    }
    body[data-theme="light"] .theme-toggle-dot {
      transform: translateX(0);
    }
    .rail-brand {
      display: flex;
      align-items: center;
      gap: 12px;
      min-width: 0;
      color: var(--rail-strong);
      font-size: 15px;
      font-weight: 720;
      line-height: 1.25;
    }
    .rail-brand span:last-child {
      min-width: 0;
      overflow-wrap: anywhere;
    }
    .rail-meta {
      display: grid;
      gap: 8px;
      padding: 14px;
      color: var(--rail-muted);
      background: var(--rail-panel-bg);
      border: 1px solid var(--rail-panel-border);
      border-radius: 8px;
      font-size: 12px;
      line-height: 1.45;
    }
    .rail-meta strong {
      color: var(--rail-strong);
      font-size: 13px;
      font-weight: 650;
    }
    .rail-nav {
      display: grid;
      gap: 6px;
    }
    .rail-nav a {
      display: flex;
      align-items: center;
      min-height: 38px;
      padding: 0 12px;
      color: var(--rail-link);
      border: 1px solid transparent;
      border-radius: 8px;
      font-size: 13px;
      font-weight: 560;
      transition:
        background-color 160ms ease,
        border-color 160ms ease,
        color 160ms ease,
        transform 160ms ease;
    }
    .rail-nav a:hover {
      color: var(--rail-link-hover);
      background: var(--rail-link-hover-bg);
      border-color: var(--rail-link-hover-border);
      transform: translateX(2px);
    }
    .rail-foot {
      margin-top: auto;
      color: var(--rail-muted);
      font-size: 12px;
      line-height: 1.45;
    }
    .report-content {
      min-width: 0;
    }
    .brand-mark {
      width: 22px;
      height: 22px;
      border: 5px solid var(--grafana-orange);
      border-right-color: transparent;
      border-radius: 50%;
      position: relative;
      flex: 0 0 auto;
    }
    .brand-mark::after {
      content: "";
      position: absolute;
      right: -8px;
      top: -5px;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: var(--grafana-orange);
    }
    main {
      min-height: 100vh;
      padding: 24px clamp(16px, 2vw, 32px) 56px;
    }
    .hero-band {
      position: relative;
      overflow: hidden;
      max-width: min(100%, 1320px);
      margin: 0 auto;
      padding: 28px;
      background: var(--hero-bg);
      border: 1px solid var(--hero-border);
      border-radius: 8px;
      box-shadow: var(--hero-shadow);
    }
    .hero-band::before {
      content: "";
      position: absolute;
      inset: 0;
      pointer-events: none;
      background-image:
        linear-gradient(var(--hero-grid-line) 1px, transparent 1px),
        linear-gradient(90deg, var(--hero-grid-line) 1px, transparent 1px);
      background-size: 42px 42px;
      mask-image: linear-gradient(180deg, rgba(0, 0, 0, 0.62), transparent 78%);
    }
    .hero-grid {
      position: relative;
      z-index: 1;
      display: grid;
      grid-template-columns: minmax(220px, 0.74fr) minmax(320px, 1.26fr);
      gap: 24px;
      align-items: end;
      max-width: none;
      margin: 0;
    }
    .hero-copy h1 {
      margin: 8px 0 8px;
      font-size: 68px;
      line-height: 0.96;
      font-weight: 760;
      letter-spacing: 0;
      color: transparent;
      background: var(--hero-title);
      -webkit-background-clip: text;
      background-clip: text;
    }
    .hero-copy p {
      margin: 0;
      color: var(--hero-text);
      font-size: 18px;
      font-weight: 400;
      line-height: 1.5;
    }
    .eyebrow {
      display: inline-flex;
      align-items: center;
      min-height: 24px;
      padding: 0 9px;
      color: var(--eyebrow-text);
      background: var(--eyebrow-bg);
      border: 1px solid var(--eyebrow-border);
      border-radius: 999px;
      font-size: 13px;
      font-weight: 650;
      letter-spacing: 0;
      text-transform: uppercase;
    }
    .summary-panel {
      min-width: 0;
      overflow: hidden;
      padding: 20px;
      background: var(--summary-panel-bg);
      border: 1px solid var(--summary-panel-border);
      border-radius: 8px;
      box-shadow: var(--summary-shadow);
      backdrop-filter: blur(18px);
    }
    .summary-panel p {
      margin: 0 0 14px;
      color: var(--hero-text);
      font-size: 14px;
      line-height: 1.55;
    }
    .summary-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: 12px;
    }
    .summary-card {
      display: grid;
      gap: 8px;
      min-width: 0;
      padding: 14px;
      background: var(--summary-card-bg);
      border: 1px solid var(--summary-card-border);
      border-radius: 8px;
      box-shadow: inset 0 1px 0 var(--summary-card-inset);
    }
    .summary-card strong {
      overflow-wrap: anywhere;
      color: var(--ink);
      font-size: 18px;
      font-weight: 650;
    }
    .summary-label,
    .stat-label,
    .section-count {
      color: var(--muted);
      font-size: 13px;
      font-weight: 500;
    }
    .stat-band {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 12px;
      max-width: min(100%, 1320px);
      margin: 16px auto 0;
      padding: 0;
      position: relative;
      z-index: 2;
    }
    .stat-item {
      display: grid;
      gap: 8px;
      padding: 22px;
      background: var(--stat-bg);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow-soft);
    }
    .stat-item strong {
      font-size: 28px;
      font-weight: 650;
    }
    .content-section {
      max-width: min(100%, 1320px);
      margin: 0 auto;
      padding: 32px 0 0;
    }
    .content-section:last-child { padding-bottom: 56px; }
    .section-heading {
      display: flex;
      align-items: end;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 16px;
    }
    .section-heading h2 {
      margin: 4px 0 0;
      font-size: 28px;
      font-weight: 680;
      letter-spacing: 0;
      color: var(--ink);
    }
    .table-card,
    .detail-panel,
    .chart-card {
      min-width: 0;
      overflow: hidden;
      background: var(--card-bg);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow-card);
    }
    .detail-panel,
    .chart-card { padding: 18px; }
    .uncertainty-block {
      display: grid;
      gap: 14px;
    }
    .table-scroll {
      max-width: 100%;
      overflow-x: auto;
    }
    table {
      width: 100%;
      min-width: 0;
      border-collapse: collapse;
      table-layout: fixed;
      font-size: 13px;
    }
    th, td {
      text-align: left;
      border-bottom: 1px solid var(--line-soft);
      padding: 11px 12px;
      vertical-align: top;
      color: var(--table-text);
      overflow-wrap: anywhere;
      word-break: normal;
    }
    tbody tr {
      transition: background-color 140ms ease;
    }
    tbody tr:hover {
      background: var(--row-hover-bg);
    }
    tr:last-child td,
    tr:last-child th { border-bottom: 0; }
    th {
      font-weight: 650;
      white-space: normal;
    }
    thead th {
      color: var(--muted);
      background: var(--subtle-row-bg);
    }
    tbody th {
      width: 180px;
      color: var(--muted);
      background: var(--subtle-row-bg);
      border-right: 1px solid var(--line-soft);
    }
    td.numeric,
    th.numeric {
      width: 88px;
      text-align: right;
      white-space: nowrap;
    }
    code {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 12px;
      color: var(--code-text);
      background: var(--code-bg);
      border: 1px solid var(--code-border);
      padding: 2px 5px;
      border-radius: 4px;
      overflow-wrap: anywhere;
    }
    .chart-card-header {
      display: flex;
      align-items: start;
      justify-content: space-between;
      gap: 14px;
      margin-bottom: 16px;
    }
    .chart-card-header h3 {
      margin: 0 0 4px;
      font-size: 16px;
      font-weight: 620;
      letter-spacing: 0;
      color: var(--ink);
    }
    .chart-card-header span,
    .bar-footnote {
      color: var(--muted);
      font-size: 12px;
      font-weight: 450;
    }
    .bar-list {
      display: grid;
      gap: 10px;
    }
    .usage-bar {
      display: grid;
      gap: 7px;
      padding: 12px;
      background: var(--bar-row-bg);
      border: 1px solid var(--line-soft);
      border-radius: 8px;
      transition:
        border-color 160ms ease,
        box-shadow 160ms ease,
        transform 160ms ease;
    }
    .usage-bar:hover {
      border-color: var(--bar-hover-border);
      box-shadow: var(--bar-hover-shadow);
      transform: translateY(-1px);
    }
    .bar-meta {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      color: var(--table-text);
      font-size: 13px;
      font-weight: 560;
    }
    .bar-name {
      min-width: 0;
      overflow-wrap: anywhere;
    }
    .bar-value {
      flex: 0 0 auto;
      color: var(--ink);
      font-weight: 650;
    }
    .bar-track {
      display: block;
      width: 100%;
      height: 11px;
      overflow: hidden;
      background: var(--bar-track-bg);
      border-radius: 999px;
    }
    .bar-fill {
      display: block;
      height: 100%;
      background: linear-gradient(90deg, var(--grafana-orange), var(--grafana-blue));
      border-radius: inherit;
    }
    .chip,
    .pill {
      display: inline-flex;
      align-items: center;
      width: max-content;
      max-width: 100%;
      min-height: 24px;
      padding: 3px 8px;
      border-radius: 999px;
      color: var(--chip-text);
      background: var(--chip-bg);
      border: 1px solid var(--chip-border);
      font-size: 12px;
      font-weight: 550;
      margin: 2px;
    }
    .action-chip {
      color: var(--grafana-orange-dark);
      background: rgba(255, 90, 31, 0.15);
      border-color: rgba(255, 90, 31, 0.28);
    }
    .assumption-list,
    .warning-list {
      display: grid;
      gap: 10px;
      margin: 0;
      padding: 0;
      list-style: none;
    }
    .assumption-list li,
    .warning-list li,
    .empty-state {
      padding: 14px;
      color: var(--muted);
      background: var(--note-bg);
      border: 1px solid var(--line-soft);
      border-radius: 8px;
      line-height: 1.45;
      box-shadow: var(--note-shadow);
    }
    .risk-high { color: var(--red); font-weight: 700; }
    .risk-medium { color: var(--yellow); font-weight: 700; }
    .risk-low { color: var(--grafana-blue); font-weight: 700; }
    .muted { color: var(--muted); }
    @media (max-width: 1440px) {
      .side-rail {
        gap: 18px;
        padding: 18px 16px;
      }
      .rail-nav a {
        min-height: 34px;
        padding: 0 10px;
      }
      .hero-copy h1 { font-size: 60px; }
      .hero-copy p { font-size: 16px; }
      .hero-band { padding: 24px; }
      .summary-panel { padding: 16px; }
      .summary-card,
      .stat-item { padding: 14px; }
      .stat-item strong { font-size: 24px; }
      .section-heading h2 { font-size: 24px; }
      .content-section { padding-top: 26px; }
      th, td { padding: 8px 9px; }
      table { font-size: 12px; }
      code { font-size: 11px; }
    }
    @media (max-width: 1280px) {
      .report-shell {
        grid-template-columns: 1fr;
      }
      .side-rail {
        position: static;
        height: auto;
        padding: 14px 20px;
      }
      .theme-toggle {
        max-width: 190px;
      }
      .rail-brand,
      .rail-meta,
      .rail-foot {
        display: none;
      }
      .rail-nav {
        grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
      }
      .rail-nav a {
        justify-content: center;
      }
      main {
        padding: 20px;
      }
      .hero-band,
      .stat-band,
      .content-section {
        max-width: none;
      }
      .hero-grid { grid-template-columns: 1fr; }
    }
    @media (max-width: 760px) {
      main {
        padding: 16px;
      }
      .side-rail {
        padding: 16px;
      }
      .theme-toggle {
        max-width: none;
      }
      .rail-nav {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
      .hero-copy h1 { font-size: 46px; }
      .hero-copy p { font-size: 15px; }
      .summary-grid,
      .stat-band { grid-template-columns: 1fr; }
      .responsive-table {
        display: block;
        min-width: 0;
      }
      .responsive-table thead {
        display: none;
      }
      .responsive-table tbody,
      .responsive-table tr,
      .responsive-table td {
        display: block;
        width: 100%;
      }
      .responsive-table tbody tr {
        padding: 8px 10px;
        border-bottom: 1px solid var(--line-soft);
      }
      .responsive-table tbody tr:last-child {
        border-bottom: 0;
      }
      .responsive-table td {
        display: grid;
        grid-template-columns: minmax(96px, 34%) minmax(0, 1fr);
        gap: 10px;
        padding: 7px 0;
        border-bottom: 0;
      }
      .responsive-table td::before {
        content: attr(data-label);
        color: var(--muted);
        font-weight: 650;
      }
      .responsive-table td.numeric {
        width: 100%;
        text-align: left;
        white-space: normal;
      }
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
        <span>Grafana Cloud App O11y Series Estimate</span>
      </div>
      <div class="rail-meta">
        <strong>Static Java sizing report</strong>
        <span>Generated {{ .GeneratedAt.Format "2006-01-02 15:04:05 UTC" }}</span>
      </div>
      <nav class="rail-nav">
        <a href="#components">Components</a>
        <a href="#processors">Processors</a>
        <a href="#uncertainty">Uncertainty</a>
        <a href="#operations">Contributors</a>
        <a href="#services">Services</a>
        <a href="#operation-table">Operations</a>
        <a href="#service-graph">Service Graph</a>
        <a href="#risks">Risks</a>
      </nav>
      <div class="rail-foot">Offline estimator. Exact active series still depend on runtime trace label values.</div>
    </aside>

    <div class="report-content">
      <main>
    <section class="hero-band">
      <div class="hero-grid">
        <div class="hero-copy">
          <span class="eyebrow">Expected active series</span>
          <h1>{{ .Estimate.TotalExpected }}</h1>
          <p>{{ .Estimate.TotalLow }} low / {{ .Estimate.TotalHigh }} high. Source code can forecast metric cardinality, while exact active series still depend on runtime traces, label values, enabled dimensions, and metrics-generator configuration.</p>
        </div>

        <aside class="summary-panel" aria-label="Scan summary">
          <p>Scanned <strong>{{ len .Analysis.Services }}</strong> service(s), <strong>{{ len .Analysis.Operations }}</strong> operation(s), and <strong>{{ len .Analysis.Edges }}</strong> service-graph edge(s) using the <strong>{{ .Options.Profile }}</strong> profile.</p>
          <div class="summary-grid">
            <div class="summary-card">
              <span class="summary-label">Repository</span>
              <strong>{{ .Analysis.Repository }}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label">Source</span>
              <strong>{{ if .Source.Type }}{{ .Source.Type }}{{ else }}local{{ end }}{{ if .Source.Provider }} - {{ .Source.Provider }}{{ end }}</strong>
            </div>
			<div class="summary-card">
				<span class="summary-label">Histogram type</span>
				<strong>{{ if .Options.HistogramType }}{{ .Options.HistogramType }}{{ else }}native{{ end }}</strong>
			</div>
			<div class="summary-card">
				<span class="summary-label">Classic bucket series</span>
				<strong>{{ .Options.HistogramBuckets }}</strong>
			</div>
			<div class="summary-card">
				<span class="summary-label">Status values</span>
				<strong>{{ .Options.StatusValues }}</strong>
			</div>
			<div class="summary-card">
				<span class="summary-label">Instance label</span>
				<strong>{{ if .Options.InstanceLabelEnabled }}enabled{{ else }}disabled{{ end }}</strong>
			</div>
			<div class="summary-card">
				<span class="summary-label">Instances / service</span>
				<strong>{{ .Options.InstancesPerService }}</strong>
			</div>
          </div>
        </aside>
      </div>
    </section>

    <section class="stat-band" aria-label="Series sizing summary">
      <div class="stat-item">
        <span class="stat-label">Expected active series</span>
        <strong>{{ .Estimate.TotalExpected }}</strong>
      </div>
      <div class="stat-item">
        <span class="stat-label">Detected operations</span>
        <strong>{{ len .Analysis.Operations }}</strong>
      </div>
      <div class="stat-item">
        <span class="stat-label">Detected service edges</span>
        <strong>{{ len .Analysis.Edges }}</strong>
      </div>
      <div class="stat-item">
        <span class="stat-label">Detected services</span>
        <strong>{{ len .Analysis.Services }}</strong>
      </div>
    </section>

    {{ if .Source.Type }}
    <section class="content-section" id="source">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Input provenance</span>
          <h2>Source</h2>
        </div>
        <span class="section-count">{{ if .Source.WorktreeTemporary }}temporary worktree{{ else }}local worktree{{ end }}</span>
      </div>
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <tbody>
              <tr><th>Input</th><td><code>{{ .Source.Original }}</code></td></tr>
              <tr><th>Type</th><td>{{ .Source.Type }}</td></tr>
              {{ if .Source.Provider }}<tr><th>Provider</th><td>{{ .Source.Provider }}</td></tr>{{ end }}
              {{ if .Source.CloneURL }}<tr><th>Clone URL</th><td><code>{{ .Source.CloneURL }}</code></td></tr>{{ end }}
              {{ if .Source.RequestedRef }}<tr><th>Requested ref</th><td><code>{{ .Source.RequestedRef }}</code></td></tr>{{ end }}
              {{ if .Source.ResolvedRef }}<tr><th>Resolved ref</th><td><code>{{ .Source.ResolvedRef }}</code></td></tr>{{ end }}
              <tr><th>Resolved path</th><td><code>{{ .Source.ResolvedPath }}</code></td></tr>
              <tr><th>Worktree</th><td>{{ if .Source.WorktreeTemporary }}temporary{{ else }}local{{ end }}{{ if .Source.WorktreeRetained }}, retained{{ end }}</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>
    {{ end }}

    <section class="content-section" id="components">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Generated metric model</span>
          <h2>Component Breakdown</h2>
        </div>
        <span class="section-count">{{ len .Estimate.ComponentBreakdown }} components</span>
      </div>
      {{ if .Estimate.ComponentBreakdown }}
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <thead><tr><th>Component</th><th class="numeric">Expected</th><th>Formula</th></tr></thead>
            <tbody>
              {{ range .Estimate.ComponentBreakdown }}
              <tr><td><code>{{ .Component }}</code></td><td class="numeric">{{ .Expected }}</td><td>{{ .Formula }}</td></tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
      {{ else }}
      <div class="empty-state">No component-level estimate was generated.</div>
      {{ end }}
    </section>

    <section class="content-section" id="processors">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Metrics-generator processors</span>
          <h2>Processor Breakdown</h2>
        </div>
        <span class="section-count">{{ len .Estimate.ProcessorBreakdown }} processors</span>
      </div>
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <thead><tr><th>Processor</th><th class="numeric">Low</th><th class="numeric">Expected</th><th class="numeric">High</th><th>Formula</th></tr></thead>
            <tbody>
              {{ range .Estimate.ProcessorBreakdown }}
              <tr><td><code>{{ .Processor }}</code></td><td class="numeric">{{ .Low }}</td><td class="numeric">{{ .Expected }}</td><td class="numeric">{{ .High }}</td><td>{{ .Formula }}</td></tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
    </section>

    {{ if .Estimate.UncertaintyModel }}
    <section class="content-section" id="uncertainty">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Range assumptions</span>
          <h2>Uncertainty Model</h2>
        </div>
        <span class="section-count">{{ len .Estimate.UncertaintyModel }} model(s)</span>
      </div>
      {{ range .Estimate.UncertaintyModel }}
      <div class="uncertainty-block">
        <div class="chart-card-header">
          <div>
            <h3>{{ .Scope }}</h3>
            <span>{{ .Formula }}</span>
          </div>
          <span class="chip action-chip">auditable bounds</span>
        </div>
        <div class="table-card">
          <div class="table-scroll">
            <table>
              <thead><tr><th>Bound</th><th>Status values</th><th>Dimension multiplier</th><th>Buffer</th></tr></thead>
              <tbody>
                {{ range .Bounds }}
                <tr><td>{{ .Bound }}</td><td>{{ .StatusRule }} = {{ .StatusValues }}</td><td>{{ .DimensionRule }} = {{ .DimensionMultiplier }}</td><td>{{ .Buffer }}</td></tr>
                {{ end }}
              </tbody>
            </table>
          </div>
        </div>
        {{ if .HistogramFactors }}
        <div class="table-card">
          <div class="table-scroll">
            <table>
              <thead><tr><th>Metric</th><th>Histogram factor</th><th class="numeric">Value</th></tr></thead>
              <tbody>
                {{ range .HistogramFactors }}
                <tr><td><code>{{ .Metric }}</code></td><td>{{ .Factor }}</td><td class="numeric">{{ .Value }}</td></tr>
                {{ end }}
              </tbody>
            </table>
          </div>
        </div>
        {{ end }}
        {{ if .Notes }}
        <ul class="assumption-list" aria-label="Uncertainty notes">
          {{ range .Notes }}<li>{{ . }}</li>{{ end }}
        </ul>
        {{ end }}
      </div>
      {{ end }}
    </section>
    {{ end }}

    <section class="content-section" id="operations">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Largest span metric routes</span>
          <h2>Top Span Metric Operation Contributors</h2>
        </div>
        <span class="section-count">{{ len .Estimate.OperationContributors }} rows</span>
      </div>
      {{ if .Estimate.OperationContributors }}
      <article class="chart-card">
        <div class="chart-card-header">
          <div>
            <h3>Operation contribution</h3>
            <span>Expected span metric series per detected route or consumer</span>
          </div>
          <span class="chip action-chip">span metrics</span>
        </div>
        <div class="bar-list">
          {{ $total := .Estimate.TotalExpected }}
          {{ range .Estimate.OperationContributors }}
          <div class="usage-bar">
            <div class="bar-meta">
              <span class="bar-name">{{ if .Protocol }}<span class="pill">{{ .Protocol }}</span>{{ end }} <code>{{ .Method }}</code> {{ .Route }}{{ if .Origin }} <span class="pill">{{ .Origin }}</span>{{ end }}</span>
              <span class="bar-value">{{ .Expected }}</span>
            </div>
            <span class="bar-track">
              <span class="bar-fill" style="width: {{ bar .Expected $total }}%"></span>
            </span>
            <span class="bar-footnote">{{ .Service }} - {{ .Kind }}</span>
          </div>
          {{ end }}
        </div>
      </article>
      {{ else }}
      <div class="empty-state">No operation-level contributors were detected.</div>
      {{ end }}
    </section>

    {{ if .Estimate.ServiceBreakdown }}
    <section class="content-section" id="service-contributors">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Service contribution</span>
          <h2>Top Service Contributors</h2>
        </div>
        <span class="section-count">{{ len .Estimate.ServiceBreakdown }} services</span>
      </div>
      <article class="chart-card">
        <div class="bar-list">
          {{ $total := .Estimate.TotalExpected }}
          {{ range .Estimate.ServiceBreakdown }}
          <div class="usage-bar">
            <div class="bar-meta">
              <span class="bar-name"><code>{{ .Service }}</code></span>
              <span class="bar-value">{{ .Expected }}</span>
            </div>
            <span class="bar-track">
              <span class="bar-fill" style="width: {{ bar .Expected $total }}%"></span>
            </span>
          </div>
          {{ end }}
        </div>
      </article>
    </section>
    {{ end }}

    <section class="content-section" id="services">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Detected applications</span>
          <h2>Services</h2>
        </div>
        <span class="section-count">{{ len .Analysis.Services }} services</span>
      </div>
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <thead><tr><th>Service</th><th>Root</th><th class="numeric">Operations</th><th class="numeric">Edges</th><th>Source</th></tr></thead>
            <tbody>
              {{ range .Analysis.Services }}
              <tr><td><code>{{ .Name }}</code></td><td>{{ .Root }}</td><td class="numeric">{{ .OperationCount }}</td><td class="numeric">{{ .EdgeCount }}</td><td>{{ .Source }}</td></tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="content-section" id="operation-table">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Route inventory</span>
          <h2>Operations</h2>
        </div>
        <span class="section-count">{{ len .Analysis.Operations }} operations</span>
      </div>
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <thead><tr><th>Service</th><th>Kind</th><th>Protocol</th><th>Method</th><th>Route</th><th>Confidence</th><th>Detectors</th><th>Handler</th><th>Source</th></tr></thead>
            <tbody>
              {{ range .Analysis.Operations }}
              <tr><td><code>{{ .Service }}</code></td><td>{{ .Kind }}</td><td>{{ .Protocol }}</td><td>{{ .Method }}</td><td><code>{{ .Route }}</code></td><td>{{ .Confidence }}</td><td>{{ range .Detectors }}<span class="pill">{{ . }}</span>{{ end }}</td><td>{{ .Handler }}</td><td>{{ .Source }}</td></tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <section class="content-section" id="service-graph">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Static dependency hints</span>
          <h2>Service Graph Edges</h2>
        </div>
        <span class="section-count">{{ len .Analysis.Edges }} edges</span>
      </div>
      {{ if .Analysis.Edges }}
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <thead><tr><th>Source</th><th>Target</th><th>Protocol</th><th>Confidence</th><th>Source file</th></tr></thead>
            <tbody>
              {{ range .Analysis.Edges }}
              <tr><td><code>{{ .SourceService }}</code></td><td><code>{{ .TargetService }}</code></td><td>{{ .Protocol }}</td><td>{{ .Confidence }}</td><td>{{ .Source }}</td></tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
      {{ else }}
      <div class="empty-state">No static service graph edges were detected.</div>
      {{ end }}
    </section>

    <section class="content-section" id="risks">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Model notes</span>
          <h2>Risks And Assumptions</h2>
        </div>
        <span class="section-count">{{ len .Analysis.Risks }} risks</span>
      </div>
      {{ if .Analysis.Risks }}
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <thead><tr><th>Severity</th><th>Area</th><th>Message</th><th>Source</th></tr></thead>
            <tbody>
              {{ range .Analysis.Risks }}
              <tr><td class="risk-{{ .Severity }}">{{ .Severity }}</td><td>{{ .Area }}</td><td>{{ .Message }}</td><td>{{ .Source }}</td></tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
      {{ else }}
      <div class="empty-state">No high-cardinality risks were detected by static analysis.</div>
      {{ end }}
      {{ if .Estimate.Assumptions }}
      <ul class="assumption-list" aria-label="Assumptions">
        {{ range .Estimate.Assumptions }}<li>{{ . }}</li>{{ end }}
      </ul>
      {{ end }}
    </section>

    <section class="content-section" id="config">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Configuration scan</span>
          <h2>Configuration Findings</h2>
        </div>
        <span class="section-count">{{ len .Analysis.ConfigFindings }} findings</span>
      </div>
      {{ if .Analysis.ConfigFindings }}
      <div class="table-card">
        <div class="table-scroll">
          <table>
            <thead><tr><th>Kind</th><th>Name</th><th>Value</th><th>Service</th><th>Source</th></tr></thead>
            <tbody>
              {{ range .Analysis.ConfigFindings }}
              <tr><td>{{ .Kind }}</td><td><code>{{ .Name }}</code></td><td>{{ .Value }}</td><td>{{ .Service }}</td><td>{{ .Source }}</td></tr>
              {{ end }}
            </tbody>
          </table>
        </div>
      </div>
      {{ else }}
      <div class="empty-state">No App O11y or service-name configuration files were detected.</div>
      {{ end }}
    </section>

    {{ if .Estimate.CloudCalibration }}
    <section class="content-section" id="calibration">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Observed telemetry</span>
          <h2>Grafana Cloud Calibration</h2>
        </div>
      </div>
      <div class="detail-panel">
        <div class="chart-card-header">
          <div>
            <h3>{{ .Estimate.CloudCalibration.Message }}</h3>
            {{ if .Estimate.CloudCalibration.ObservedSeries }}
            <span>Observed active series: <strong>{{ .Estimate.CloudCalibration.ObservedSeries }}</strong></span>
            {{ end }}
          </div>
          <span class="chip action-chip">read-only</span>
        </div>
        {{ if .Estimate.CloudCalibration.Query }}<p class="muted">Query <code>{{ .Estimate.CloudCalibration.Query }}</code></p>{{ end }}
      </div>
    </section>
    {{ end }}

    {{ if .Analysis.Warnings }}
    <section class="content-section" id="warnings">
      <div class="section-heading">
        <div>
          <span class="eyebrow">Scanner output</span>
          <h2>Warnings</h2>
        </div>
        <span class="section-count">{{ len .Analysis.Warnings }} warnings</span>
      </div>
      <ul class="warning-list">
        {{ range .Analysis.Warnings }}<li>{{ . }}</li>{{ end }}
      </ul>
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

      var storedTheme = "";
      try {
        storedTheme = localStorage.getItem(storageKey) || "";
      } catch (error) {}

      prepareResponsiveTables();
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
