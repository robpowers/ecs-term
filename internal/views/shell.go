package views

import (
	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/robpowers/ecs-term/internal/aws"
	"github.com/robpowers/ecs-term/internal/config"
	"github.com/robpowers/ecs-term/internal/domain"
	"github.com/robpowers/ecs-term/internal/model"
)

// shellForTask returns a tea.Cmd that shells into a task's container. If the
// task has exactly one container, it launches immediately. Otherwise it pushes
// the ContainerPickerView so the user can choose.
func shellForTask(ctx config.Context, task domain.ECSTask) tea.Cmd {
	switch len(task.Containers) {
	case 0:
		return nil
	case 1:
		return execShellCmd(ctx, task.TaskARN, task.Containers[0].Name)
	default:
		names := make([]string, 0, len(task.Containers))
		for _, c := range task.Containers {
			names = append(names, c.Name)
		}
		picker := NewContainerPickerView(task.TaskARN, names)
		return func() tea.Msg { return model.NavigatePushMsg{View: &picker} }
	}
}

// execShellCmd wraps the `aws ecs execute-command` invocation so bubbletea
// can suspend the TUI, run the shell, and resume when it exits.
func execShellCmd(ctx config.Context, taskARN, container string) tea.Cmd {
	cmd := awsclient.BuildExecCommand(ctx.AWSProfile, ctx.Region, ctx.Cluster, taskARN, container, "")
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return model.ExecFinishedMsg{Err: err}
	})
}
