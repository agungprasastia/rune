package runecommands

import (
	"encoding/json"
	"strings"
	"testing"

	"rune/internal/config"
	"rune/internal/hooks"
	"rune/internal/plugins"
)

func TestNewBackendDoctorReportSurfacesDiagnosticsAndActions(t *testing.T) {
	secret := "sk-proj-" + strings.Repeat("a", 24)
	report := NewBackendDoctorReport(BackendDoctorInput{
		MCP: config.MCPConfig{Servers: map[string]config.MCPServerConfig{
			"remote": {
				Type: "http",
				URL:  "https://api.example.com/mcp?token=" + secret,
			},
			"broken": {
				Type: "http",
			},
			"disabled": {
				Type:     "stdio",
				Command:  "docs-mcp",
				Disabled: true,
			},
		}},
		Hooks: hooks.LoadResult{
			Config: hooks.Config{Enabled: false, Hooks: []hooks.Definition{{
				ID:      "rune.preflight",
				Event:   hooks.EventBeforeTool,
				Command: "sh",
				Enabled: true,
			}}},
			Diagnostics: []hooks.Diagnostic{{
				Kind:      hooks.DiagnosticSchema,
				Message:   "bad arg " + secret,
				Path:      "/tmp/" + secret + "/hooks.json",
				HookID:    "rune.preflight-" + secret,
				FieldPath: "hooks.0.command." + secret,
			}},
		},
		Plugins: plugins.LoadResult{
			Plugins: []plugins.LoadedPlugin{{
				ID:      "rune.docs",
				Name:    "Docs",
				Enabled: false,
				Source:  plugins.SourceProject,
			}},
			Diagnostics: []plugins.Diagnostic{{
				Kind:         plugins.DiagnosticDuplicate,
				Message:      "duplicate " + secret,
				Root:         "/tmp/" + secret + "/plugins",
				PluginPath:   "/tmp/" + secret + "/plugins/docs",
				ManifestPath: "/tmp/plugin.json?token=" + secret,
				FieldPath:    "tools.0.command." + secret,
				PluginID:     "rune.docs-" + secret,
			}},
		},
	})

	if report.OK {
		t.Fatalf("report.OK = true, want false because broken MCP and hook schema diagnostics exist: %#v", report.Checks)
	}
	if report.Status != BackendDoctorStatusFail {
		t.Fatalf("report.Status = %q, want %q", report.Status, BackendDoctorStatusFail)
	}
	assertBackendDoctorCheck(t, report, "backend.mcp.server", "remote", BackendDoctorStatusPass, "rune mcp check remote")
	assertBackendDoctorCheck(t, report, "backend.mcp.invalid", "broken", BackendDoctorStatusFail, "rune mcp add broken")
	assertBackendDoctorCheck(t, report, "backend.mcp.disabled", "disabled", BackendDoctorStatusWarn, "rune mcp enable disabled")
	assertBackendDoctorCheck(t, report, "backend.hooks.disabled", "hooks", BackendDoctorStatusWarn, "rune hooks list")
	assertBackendDoctorCheck(t, report, "backend.hooks.diagnostic", "rune.preflight-[REDACTED]", BackendDoctorStatusFail, "rune hooks list")
	assertBackendDoctorCheck(t, report, "backend.plugins.disabled", "rune.docs", BackendDoctorStatusWarn, "rune plugins list")
	assertBackendDoctorCheck(t, report, "backend.plugins.diagnostic", "rune.docs-[REDACTED]", BackendDoctorStatusWarn, "rune plugins list")

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "sk-proj-") {
		t.Fatalf("backend doctor report leaked secret material: %s", string(encoded))
	}
}

func TestNewBackendDoctorReportPassesEmptySetup(t *testing.T) {
	report := NewBackendDoctorReport(BackendDoctorInput{})
	if !report.OK {
		t.Fatalf("empty setup should be a passing report, got %#v", report.Checks)
	}
	if report.Status != BackendDoctorStatusPass {
		t.Fatalf("report.Status = %q, want %q", report.Status, BackendDoctorStatusPass)
	}
	assertBackendDoctorCheck(t, report, "backend.mcp.configured", "mcp", BackendDoctorStatusPass, "rune mcp add")
	assertBackendDoctorCheck(t, report, "backend.hooks.configured", "hooks", BackendDoctorStatusPass, "rune hooks list")
	assertBackendDoctorCheck(t, report, "backend.plugins.configured", "plugins", BackendDoctorStatusPass, "rune plugins list")
}

func TestNewBackendDoctorReportWarnsWithoutFailing(t *testing.T) {
	report := NewBackendDoctorReport(BackendDoctorInput{
		MCP: config.MCPConfig{Servers: map[string]config.MCPServerConfig{
			"disabled": {Type: "stdio", Command: "docs-mcp", Disabled: true},
		}},
	})
	if !report.OK {
		t.Fatalf("warning-only report should keep OK=true, got %#v", report.Checks)
	}
	if report.Status != BackendDoctorStatusWarn {
		t.Fatalf("report.Status = %q, want %q", report.Status, BackendDoctorStatusWarn)
	}
}

func assertBackendDoctorCheck(t *testing.T, report BackendDoctorReport, id string, target string, status BackendDoctorStatus, actionContains string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID == id && check.Target == target {
			if check.Status != status {
				t.Fatalf("%s/%s status = %q, want %q (check %#v)", id, target, check.Status, status, check)
			}
			if actionContains != "" && !strings.Contains(check.Action, actionContains) {
				t.Fatalf("%s/%s action = %q, want to contain %q", id, target, check.Action, actionContains)
			}
			return
		}
	}
	t.Fatalf("check %s/%s not found in %#v", id, target, report.Checks)
}
