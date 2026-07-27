package aws

import "os/exec"

// BuildExecCommand constructs an `aws ecs execute-command` invocation targeting
// a specific container in a task. Requires session-manager-plugin on PATH and
// enableExecuteCommand=true on the task.
func BuildExecCommand(profile, region, cluster, taskARN, container, shell string) *exec.Cmd {
	if shell == "" {
		shell = "/bin/sh"
	}
	args := []string{
		"ecs", "execute-command",
		"--profile", profile,
		"--region", region,
		"--cluster", cluster,
		"--task", taskARN,
		"--container", container,
		"--interactive",
		"--command", shell,
	}
	return exec.Command("aws", args...)
}
