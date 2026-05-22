# ecs-term

k9s-style read-only terminal monitor for AWS ECS. Built in Go with the charmbracelet TUI stack.

## Build & Run

Go is installed at `$HOME/go` (not system-wide). Always prefix go commands:

```bash
export PATH="$HOME/go/bin:$PATH"
go build ./...
go vet ./...
go build -o ecs-term .
```

Run (requires `~/.ecs-term.yaml` or `./ecs-term.yaml`):
```bash
aws sso login --profile <profile>   # must be done first
./ecs-term
./ecs-term --config /path/to/config.yaml
```

## Architecture

The Elm Architecture (bubbletea). One root `AppModel` owns a stack-based navigator.

```
main.go                         entry point — loads config, builds clients, starts tea.Program
internal/config/config.go       YAML config: Config{}, Context{}, Load(), FindConfigFile()
internal/domain/                pure display structs (no AWS types): ECSService, ECSTask, ContainerDetail, LogEvent
internal/aws/                   AWS SDK wrappers returning domain types
  client.go                     ClientSet — NewClientSet(ctx) loads SSO profile via aws-sdk-go-v2/config
  identity.go                   FetchAccountID() via STS
  ecs.go                        ListServices(), ListTasks(), DescribeTaskDefinition(), GetTaskLogConfig()
  logs.go                       GetRecentLogs(), GetLogStreams(), BuildLogStreamName()
internal/model/
  navigator.go                  View interface + Navigator (stack Push/Pop/Current/SetSizeAll)
  messages.go                   all custom tea.Msg types (centralized to avoid import cycles)
  keys.go                       GlobalKeyMap with both vim and arrow key bindings
  app.go                        AppModel — handles WindowSizeMsg, navigation msgs, error overlay, status bar
internal/ui/
  styles.go                     all lipgloss.Style values and the color palette
  statusbar.go                  StatusBar — Header() and Footer() renderers
  error_overlay.go              dismissable centered error overlay
internal/views/                 one file per view, all methods on pointer receivers
  util.go                       shared helpers (tickEvery)
  context_selector.go           View 1: pick a context from config
  services.go                   View 2: ECS services table (bubbles/table) with health status
  tasks.go                      View 3: tasks within a service
  logs.go                       View 4: CloudWatch logs viewport with follow-tail mode
  container_detail.go           View 5: task definition containers — env vars, ports, health check
```

## Critical Design Rules

**Pointer receivers everywhere on views.** The `model.View` interface includes `SetSize` which mutates the struct. Only `*T` satisfies the interface. All five view types must have pointer receivers on ALL methods (Init, Update, View, ViewID, KeyHints, SetSize). Breaking this causes silent state loss — the navigator stack type assertion `updated.(View)` fails and the view never updates.

**AWS types never leave `internal/aws/`.** Functions in `internal/aws/` accept AWS SDK inputs and return `internal/domain` types. Views import `domain`, never `aws-sdk-go-v2` types directly.

**All AWS I/O in `tea.Cmd` closures.** Never call AWS APIs in `Update()`. Return a `func() tea.Msg` that does the blocking call and wraps the result in a message type from `internal/model/messages.go`.

**Centralize messages.** All custom `tea.Msg` types live in `internal/model/messages.go`. This prevents import cycles between `views` and `model`.

**Auto-refresh pattern.** Each view manages its own tick in `Init()` via `tickEvery()`. On `RefreshTickMsg`, re-arm the ticker AND fire a new fetch in `tea.Batch`. When `NavigatePopMsg` fires, `AppModel` calls `navigator.Current().Init()` to restart the dead ticker.

**WindowSizeMsg propagation.** `AppModel` calls `navigator.SetSizeAll(w, contentH)` where `contentH = height - 2` (header + footer rows). This updates all views in the stack so returning to a parent after resize works correctly.

## Navigation Flow

```
ContextSelector → Enter → ServicesView  (also emits ContextSelectedMsg → updates status bar + fetches account ID)
ServicesView    → Enter → TasksView
TasksView       → Enter → LogsView      (first container's logs)
TasksView       → d     → ContainerDetailView
Any             → Esc   → NavigatePopMsg (depth 1 = quit)
Any             → r     → manual refresh
Any             → q     → quit
```

## Config File Format

```yaml
current_context: prod          # optional — auto-selects if only one context

contexts:
  - name: prod
    cluster: my-prod-cluster   # short name or ARN
    region: us-east-1
    aws_profile: prod-sso      # must match a profile in ~/.aws/config
    refresh_interval: 30       # seconds, default 30
```

Config search order: `./ecs-term.yaml`, then `~/.ecs-term.yaml`.

## Key Dependencies

| Import path | Purpose |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI event loop |
| `github.com/charmbracelet/lipgloss` | Styling and layout |
| `github.com/charmbracelet/bubbles` | list, viewport, spinner components |
| `github.com/aws/aws-sdk-go-v2/config` | `LoadDefaultConfig` + `WithSharedConfigProfile` |
| `github.com/aws/aws-sdk-go-v2/service/ecs` | ECS API |
| `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs` | CloudWatch Logs |
| `github.com/aws/aws-sdk-go-v2/service/sts` | GetCallerIdentity |
| `gopkg.in/yaml.v3` | Config parsing |

## Adding a New View

1. Create `internal/views/newview.go` — struct with pointer receivers on all methods
2. Add a `ViewNewThing ViewID = iota` constant to `internal/model/navigator.go`
3. Add any new message types to `internal/model/messages.go`
4. Emit `NavigatePushMsg{View: &newView}` from the parent view's `Update`
5. Handle the new message type in `AppModel.Update` if it carries an error or needs status bar update
