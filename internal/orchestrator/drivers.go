package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// WorkerDriver 抹平不同开源 Coding Agent 底座在无头模式（Headless CLI）下的启动参数差异。
// 新增一个底座只需要实现这 1 个接口方法（约 25 行代码）。
type WorkerDriver interface {
	Name() string
	Execute(ctx context.Context, worktreeDir string, prompt string, onPID func(pid int)) (output string, err error)
}

// NewDriver 根据底座名称返回对应的无头 CLI 驱动器
func NewDriver(harness string, customShellCmd string) WorkerDriver {
	if customShellCmd != "" {
		return &ShellDriver{name: harness, shellTemplate: customShellCmd}
	}
	switch harness {
	case "codex":
		return &CLIArgsDriver{
			name:   "codex",
			binary: "codex",
			buildArgs: func(prompt string) []string {
				return []string{"exec", "--full-auto", prompt}
			},
		}
	case "opencode":
		return &CLIArgsDriver{
			name:   "opencode",
			binary: "opencode",
			buildArgs: func(prompt string) []string {
				return []string{"run", prompt}
			},
		}
	case "pi":
		return &CLIArgsDriver{
			name:   "pi",
			binary: "pi",
			buildArgs: func(prompt string) []string {
				return []string{"-p", prompt}
			},
		}
	case "dsh":
		return &CLIArgsDriver{
			name:   "dsh",
			binary: "dsh",
			buildArgs: func(prompt string) []string {
				return []string{"headless", prompt}
			},
		}
	default:
		return &ShellDriver{name: harness, shellTemplate: "echo 'no driver configured'"}
	}
}

type CLIArgsDriver struct {
	name      string
	binary    string
	buildArgs func(prompt string) []string
}

func (d *CLIArgsDriver) Name() string { return d.name }

func (d *CLIArgsDriver) Execute(ctx context.Context, worktreeDir, prompt string, onPID func(pid int)) (string, error) {
	cmd := exec.CommandContext(ctx, d.binary, d.buildArgs(prompt)...)
	cmd.Dir = worktreeDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start %s: %w", d.name, err)
	}
	if onPID != nil && cmd.Process != nil {
		onPID(cmd.Process.Pid)
	}
	err := cmd.Wait()
	return out.String(), err
}

// ShellDriver 支持通过自定义命令模板（注入 ETREE_PROMPT 环境变量）驱动自定义脚本或本地测试桩
type ShellDriver struct {
	name          string
	shellTemplate string
}

func (d *ShellDriver) Name() string { return d.name }

func (d *ShellDriver) Execute(ctx context.Context, worktreeDir, prompt string, onPID func(pid int)) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", d.shellTemplate)
	cmd.Dir = worktreeDir
	cmd.Env = append(os.Environ(),
		"ETREE_PROMPT="+prompt,
		"ETREE_WORKTREE="+worktreeDir,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		return "", err
	}
	if onPID != nil && cmd.Process != nil {
		onPID(cmd.Process.Pid)
	}
	err := cmd.Wait()
	return out.String(), err
}
