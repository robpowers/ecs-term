package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/model"
	"github.com/robpowers/ecs-term/internal/views"
)

func main() {
	cfgPath := flag.String("config", "", "path to config file (default: ecs-term.yaml or ~/.ecs-term.yaml)")
	flag.Parse()

	// Locate and load config
	path := *cfgPath
	if path == "" {
		var err error
		path, err = config.FindConfigFile()
		if err != nil {
			fmt.Fprintln(os.Stderr, "ecs-term:", err)
			fmt.Fprintln(os.Stderr, "Create a config file at ~/.ecs-term.yaml:")
			fmt.Fprint(os.Stderr, exampleConfig)
			os.Exit(1)
		}
	}

	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ecs-term: config error:", err)
		os.Exit(1)
	}

	// Build AWS clients for each context
	clients := make(map[string]*awsclient.ClientSet)
	var firstErr error
	for _, ctx := range cfg.Contexts {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cs, err := awsclient.NewClientSet(bgCtx, ctx)
		cancel()
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("context %q: %w", ctx.Name, err)
			}
			continue
		}
		clients[ctx.Name] = cs
	}

	// Require at least one working client
	if len(clients) == 0 {
		fmt.Fprintln(os.Stderr, "ecs-term: could not connect to any AWS context")
		if firstErr != nil {
			fmt.Fprintln(os.Stderr, "  first error:", firstErr)
		}
		fmt.Fprintln(os.Stderr, "\nRun: aws sso login --profile <profile-name>")
		os.Exit(1)
	}

	// Build initial view
	contextSelector := views.NewContextSelector(cfg, clients)

	// Build app model
	appModel := model.NewAppModel(cfg, &contextSelector, clients)

	// If only one context and current_context is set, auto-jump into it
	if cfg.CurrentContext != "" && len(cfg.Contexts) == 1 {
		ctx, ok := cfg.FindContext(cfg.CurrentContext)
		if ok {
			if cs, ok := clients[ctx.Name]; ok {
				svcView := views.NewServicesView(*ctx, cs)
				appModel = model.NewAppModelWithInitialPush(cfg, &contextSelector, &svcView, clients, *ctx)
			}
		}
	}

	p := tea.NewProgram(appModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ecs-term:", err)
		os.Exit(1)
	}
}

const exampleConfig = `
current_context: my-cluster
contexts:
  - name: my-cluster
    cluster: my-ecs-cluster
    region: us-east-1
    aws_profile: my-sso-profile
    refresh_interval: 30
`
