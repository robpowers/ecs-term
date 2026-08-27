# ecs-term

A terminal UI for monitoring AWS ECS clusters, inspired by [k9s](https://k9scli.io/). Navigate your clusters, services, tasks, logs, and container definitions without leaving the terminal.

> **Note on authorship:** The code in this repository was written by [Claude](https://www.anthropic.com/claude) (Anthropic's AI coding assistant) via [Claude Code](https://claude.com/claude-code), directed by the repository owner. This is called out here so nobody mistakes it for hand-written work.

![Navigation: Context → Services → Tasks → Logs / Container Detail]

## Features

- Browse ECS services with live, color-coded status and desired/running/pending task counts
- Sort any list by column (Services, Tasks) and filter any list or describe page with `/`
- Drill into tasks, or browse all tasks in a cluster independent of any one service
- Stream CloudWatch logs: follow-tail mode, quick lookback windows (1m–24h or all), client-side search with highlighting, and a line-wrap toggle
- Shell into a running container (`aws ecs execute-command`), with an automatic picker when a task has multiple containers
- Full describe views for services and tasks — deployments, events, load balancers, network config, attachments, and tags
- Dedicated Service Events and Deployments pages
- Raw task definition viewer (YAML or JSON)
- Container detail view — environment variables, secrets, port mappings, resource limits, and health check configuration
- Built-in `?` help overlay on every screen listing that screen's keyboard shortcuts
- Auto-refreshes on a configurable interval (default 30 seconds)
- Supports multiple clusters and AWS accounts via named profiles
- Resizes dynamically with the terminal window

## Prerequisites

- Go 1.24+ (to build from source)
- AWS CLI v2 with SSO configured (`~/.aws/config`)
- Active SSO session before launching (`aws sso login --profile <profile>`)

## Installation

```bash
git clone https://github.com/robpowers/ecs-term
cd ecs-term
go build -o ecs-term .
```

Move the binary somewhere on your `$PATH` if you want to run it from anywhere:

```bash
mv ecs-term ~/.local/bin/
```

## Configuration

ecs-term looks for a config file in this order:

1. Path passed via `--config /path/to/file.yaml`
2. `./ecs-term.yaml` in the current directory
3. `~/.ecs-term.yaml` in your home directory

### Config file format

```yaml
current_context: prod          # optional — auto-selects if only one context exists

contexts:
  - name: prod
    cluster: my-prod-cluster   # ECS cluster name or full ARN
    region: us-east-1
    aws_profile: prod-sso      # profile name from ~/.aws/config
    refresh_interval: 30       # auto-refresh interval in seconds (default: 30)

  - name: staging
    cluster: my-staging-cluster
    region: us-west-2
    aws_profile: staging-sso
```

Copy the included example to get started:

```bash
cp ecs-term.yaml.example ~/.ecs-term.yaml
# edit with your cluster names, regions, and profile names
```

### Setting up AWS SSO profiles

Each context references an AWS SSO profile defined in `~/.aws/config`. A typical profile looks like:

```ini
[profile prod-sso]
sso_start_url  = https://my-org.awsapps.com/start
sso_account_id = 123456789012
sso_role_name  = MyRole
sso_region     = us-east-1
region         = us-east-1
```

Before running ecs-term, log into each profile you intend to use:

```bash
aws sso login --profile prod-sso
```

Sessions typically last 8–12 hours. If ecs-term shows an auth error, re-run the login command.

## Usage

```bash
ecs-term                          # uses ~/.ecs-term.yaml or ./ecs-term.yaml
ecs-term --config ~/my-config.yaml
```

### Key bindings

Every screen shows its own shortcuts in the footer, and pressing `?` opens a popup with the full list for whatever you're currently looking at — the tables below are a reference, not the source of truth.

**Everywhere**

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Select / drill in |
| `Esc` | Go back (clears an active filter first, if any) |
| `/` | Filter the current list, or search the current text view |
| `r` | Manual refresh |
| `?` | Toggle the help popup for this screen |
| `q` | Quit |

**Services list**

| Key | Action |
|---|---|
| `Enter` | Open the service's tasks |
| `d` | Describe service (deployments, events, load balancers, network, tags) |
| `e` | Service events |
| `v` | Deployments |
| `t` | Cluster-wide tasks list |
| `Shift+N/S/D/R/P/L` | Sort by name / status / desired / running / pending / last-deployed (press again to reverse) |

**Tasks list / Cluster tasks list**

| Key | Action |
|---|---|
| `Enter` / `l` | Open logs (prompts for a container if there's more than one) |
| `d` | Describe task (network, attachments, container runtime state, tags) |
| `c` | Container detail (env vars, secrets, ports, health check) |
| `s` | Shell into a container (`aws ecs execute-command`) |
| `y` / `J` | Raw task definition as YAML / JSON |
| `Shift+I/A/S/C` | Sort by task ID / age / status / container count |

**Logs view**

| Key | Action |
|---|---|
| `f` | Toggle follow-tail mode |
| `w` | Toggle line wrapping |
| `1`–`8`, `0` | Jump to a lookback window (1m…24h, `0` = all) |

### Navigation

```
Context selector                         (/ filter, sorted by name)
  └── Services list                      (/ filter, sort by column)
        ├── Tasks list                   (/ filter, sort by column)
        │     ├── Logs view              (Enter or l on a task)
        │     ├── Container detail       (c)
        │     ├── Task describe          (d)
        │     ├── Shell                  (s)
        │     └── Raw task definition    (y / J)
        ├── Service describe             (d)
        ├── Service events               (e)
        ├── Deployments                  (v)
        └── Cluster tasks                (t — same sub-actions as the Tasks list above)
```

A container picker pops up automatically in place of Shell/Logs when a task has more than one container.

On startup, ecs-term shows a list of contexts from your config file. Select one to enter the services view for that cluster. If your config has a single context with `current_context` set, it jumps directly to the services view.

The header bar shows the active context name, AWS account ID, and region. The footer shows context-sensitive key hints and the last-refreshed timestamp.

## Troubleshooting

**"no config file found"** — Create `~/.ecs-term.yaml` using the example above or run with `--config`.

**Auth / credentials error** — Your SSO session has expired. Run `aws sso login --profile <profile>` and relaunch.

**No services shown** — Confirm the cluster name in your config matches what appears in the AWS console. The cluster field accepts either the short name or the full ARN.

**No logs shown** — The container must use the `awslogs` log driver with `awslogs-group` and `awslogs-stream-prefix` options set in its task definition. Other log drivers (splunk, fluentd, etc.) are not supported.

**Logs view shows an error for a stopped task** — CloudWatch log streams for short-lived or stopped tasks may have expired or never been created. Check the task's stopped reason in the Tasks view.
