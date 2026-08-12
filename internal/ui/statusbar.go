package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type StatusBar struct {
	ContextName string
	AccountID   string
	Region      string
	LastRefresh time.Time
	KeyHints    []string
	Width       int
}

func (s *StatusBar) SetWidth(w int) { s.Width = w }

func (s *StatusBar) Header() string {
	raw := fmt.Sprintf(" ecs-term  ctx: %s  acct: %s  region: %s",
		s.ContextName, s.AccountID, s.Region)
	return HeaderStyle.Width(s.Width).Render(raw)
}

func (s *StatusBar) Footer() string {
	hints := buildHints(s.KeyHints)
	refreshStr := ""
	if !s.LastRefresh.IsZero() {
		refreshStr = fmt.Sprintf("  │  last: %s", s.LastRefresh.Format("15:04:05"))
	}
	content := hints + refreshStr
	return FooterStyle.Width(s.Width).Render(content)
}

func buildHints(hints []string) string {
	if len(hints) == 0 {
		return ""
	}
	var parts []string
	for _, h := range hints {
		kv := strings.SplitN(h, ":", 2)
		if len(kv) == 2 {
			parts = append(parts, KeyHintStyle.Render(kv[0])+KeyDescStyle.Render(":"+kv[1]))
		} else {
			parts = append(parts, h)
		}
	}
	return strings.Join(parts, "  ")
}

func StatusColor(status string) lipgloss.Style {
	switch strings.ToUpper(status) {
	case "RUNNING", "ACTIVE", "HEALTHY":
		return HealthyStyle
	case "PENDING", "PROVISIONING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING":
		return WarningStyle
	case "STOPPED", "INACTIVE", "DRAINING", "FAILED", "ERROR", "UNHEALTHY":
		return ErrorFgStyle
	default:
		return DimStyle
	}
}

// statusAnsiCode maps a status to a basic (16-color) ANSI SGR foreground code.
func statusAnsiCode(status string) int {
	switch strings.ToUpper(status) {
	case "RUNNING", "ACTIVE", "HEALTHY":
		return 32 // green
	case "PENDING", "PROVISIONING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING":
		return 33 // yellow
	case "STOPPED", "INACTIVE", "DRAINING", "FAILED", "ERROR", "UNHEALTHY":
		return 31 // red
	default:
		return 90 // dim gray
	}
}

// StatusColorSafeText renders status text in a basic ANSI foreground color,
// closed with a foreground-only reset (SGR 39) instead of lipgloss's usual
// full reset (SGR 0). Use this (never StatusColor) for text embedded in a
// bubbles/table cell, for two reasons:
//
//  1. bubbles/table (v1.0.0) truncates cell content with go-runewidth, which
//     isn't ANSI-aware — it counts every escape-sequence byte as a printable
//     character. lipgloss's default renderer auto-detects the terminal's
//     color profile (often TrueColor, 20+ escape bytes per styled cell),
//     and that miscount causes truncation to cut through the escape codes
//     and corrupt the terminal's render state. These codes are always a
//     small, fixed length regardless of terminal capability, so they never
//     approach the column widths in services.go/tasks.go/cluster_tasks.go.
//  2. When a row is selected, bubbles/table wraps the *entire* already
//     -rendered row string in the Selected style, which appends its own
//     full reset (SGR 0) at the end. A full reset embedded mid-row (from a
//     styled status cell) would terminate that outer background early,
//     truncating the highlight bar at the status column. A foreground-only
//     reset leaves the enclosing background untouched.
func StatusColorSafeText(status string) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[39m", statusAnsiCode(status), status)
}
