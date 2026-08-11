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
