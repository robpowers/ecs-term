# ecs-term

A terminal UI for monitoring AWS ECS clusters, inspired by [k9s](https://k9scli.io/). Navigate your clusters, services, tasks, logs, and container definitions without leaving the terminal.

![Navigation: Context → Services → Tasks → Logs / Container Detail]

## Features

- Browse ECS services with live status (desired / running / pending task counts)
- Drill into tasks and view per-container health and exit codes
- Stream CloudWatch logs with optional follow-tail mode
- Inspect task definition container details — environment variables, port mappings, resource limits, and health check configuration
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

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Select / drill in |
| `Esc` | Go back |
| `d` | View container details (from Tasks view) |
| `f` | Toggle log follow-tail mode (from Logs view) |
| `r` | Manual refresh |
| `q` | Quit |

### Navigation

```
Context selector
  └── Services list
        └── Tasks list
              ├── Logs view      (Enter on a task)
              └── Container detail  (d on a task)
```

On startup, ecs-term shows a list of contexts from your config file. Select one to enter the services view for that cluster. If your config has a single context with `current_context` set, it jumps directly to the services view.

The header bar shows the active context name, AWS account ID, and region. The footer shows context-sensitive key hints and the last-refreshed timestamp.

## Troubleshooting

**"no config file found"** — Create `~/.ecs-term.yaml` using the example above or run with `--config`.

**Auth / credentials error** — Your SSO session has expired. Run `aws sso login --profile <profile>` and relaunch.

**No services shown** — Confirm the cluster name in your config matches what appears in the AWS console. The cluster field accepts either the short name or the full ARN.

**No logs shown** — The container must use the `awslogs` log driver with `awslogs-group` and `awslogs-stream-prefix` options set in its task definition. Other log drivers (splunk, fluentd, etc.) are not supported.

**Logs view shows an error for a stopped task** — CloudWatch log streams for short-lived or stopped tasks may have expired or never been created. Check the task's stopped reason in the Tasks view.
