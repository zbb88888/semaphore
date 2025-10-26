package runnerproxy

// 1. runner.go 的 Runner 对象会调用 这里的 template 对象，把 bin 部署任务转化为 semaphore 的任务模板

import (
	"fmt"
	"strings"
)

// Template represents a Semaphore task template generated from a runner bin deploy task.
type Template struct {
	Name        string
	Description string
	Commands    []string
	Env         map[string]string
}

// NewTemplate creates a new Template from runner-provided parameters.
func NewTemplate(name, desc string, commands []string, env map[string]string) *Template {
	return &Template{
		Name:        name,
		Description: desc,
		Commands:    commands,
		Env:         env,
	}
}

// ToSemaphoreFormat converts the Template to a format compatible with Semaphore's task templates.
func (t *Template) ToSemaphoreFormat() map[string]interface{} {
	return map[string]interface{}{
		"name":        t.Name,
		"description": t.Description,
		"commands":    strings.Join(t.Commands, "\n"),
		"env":         t.Env,
	}
}

// Example usage: Convert runner bin deploy task to Semaphore template
func ConvertRunnerTaskToTemplate(taskName string, desc string, binPath string, args []string, env map[string]string) *Template {
	cmd := fmt.Sprintf("%s %s", binPath, strings.Join(args, " "))
	return NewTemplate(taskName, desc, []string{cmd}, env)
}
