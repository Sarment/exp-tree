package gitops

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultStatePath 将树状态存放在 `.git/exp-tree/state.json` 中：
// 既不会污染仓库 git status，也不会在剪枝执行 `git reset --hard` / `git clean -fd` 时被误删！
func DefaultStatePath(repoDir string) string {
	return filepath.Join(repoDir, ".git", "exp-tree", "state.json")
}

func IsGitRepo(ctx context.Context, repoDir string) bool {
	_, err := runGit(ctx, repoDir, "rev-parse", "--git-dir")
	return err == nil
}

func HeadSHA(ctx context.Context, repoDir string) (string, error) {
	return runGit(ctx, repoDir, "rev-parse", "HEAD")
}

// CheckoutNewBranch 从指定的父节点基准 Commit 切出干净的实验分支
func CheckoutNewBranch(ctx context.Context, repoDir, branchName, baseRef string) (string, error) {
	args := []string{"checkout", "-B", branchName}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	if _, err := runGit(ctx, repoDir, args...); err != nil {
		return "", err
	}
	return HeadSHA(ctx, repoDir)
}

// CommitSnapshot 将验证有效的实验代码打成原子 Git Commit 并返回新 SHA
func CommitSnapshot(ctx context.Context, repoDir, message string) (string, error) {
	if _, err := runGit(ctx, repoDir, "add", "-A"); err != nil {
		return "", err
	}
	// 检查是否有文件变更；若无变更直接返回当前 HEAD
	status, err := runGit(ctx, repoDir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status != "" {
		if _, err := runGit(ctx, repoDir, "commit", "-m", message); err != nil {
			return "", err
		}
	}
	return HeadSHA(ctx, repoDir)
}

// HardRollbackTo 剪枝失败分支时调用：彻底丢弃失败实验产生的脏改动并回退到父节点 Commit
func HardRollbackTo(ctx context.Context, repoDir, targetCommit string) error {
	if _, err := runGit(ctx, repoDir, "reset", "--hard"); err != nil {
		return err
	}
	if _, err := runGit(ctx, repoDir, "clean", "-fd"); err != nil {
		return err
	}
	if targetCommit != "" {
		if _, err := runGit(ctx, repoDir, "checkout", targetCommit); err != nil {
			return err
		}
	}
	return nil
}

// DiffCommits 获取任意两个树节点绑定 Commit 之间的精确代码变更 (供前端 Diff 查看器与 Super Agent 归因调用)
func DiffCommits(ctx context.Context, repoDir, fromRef, toRef string) (string, error) {
	if fromRef == "" || toRef == "" {
		return "", nil
	}
	return runGit(ctx, repoDir, "diff", "--unified=3", fromRef, toRef)
}

// CreateWorktree 为多 Agent 并行探索创建物理隔离的 Git Worktree（存放在 .git/exp-tree/worktrees/<nodeID>）
func CreateWorktree(ctx context.Context, repoDir, nodeID, baseCommit string) (string, error) {
	wtPath := filepath.Join(repoDir, ".git", "exp-tree", "worktrees", nodeID)
	branchName := "exp/" + nodeID
	args := []string{"worktree", "add", "-B", branchName, wtPath}
	if baseCommit != "" {
		args = append(args, baseCommit)
	}
	if _, err := runGit(ctx, repoDir, args...); err != nil {
		return "", err
	}
	return wtPath, nil
}

// RemoveWorktree 在并行实验节点验证完毕或剪枝后回收独立 Worktree 目录
func RemoveWorktree(ctx context.Context, repoDir, nodeID string) error {
	wtPath := filepath.Join(repoDir, ".git", "exp-tree", "worktrees", nodeID)
	_, err := runGit(ctx, repoDir, "worktree", "remove", "--force", wtPath)
	return err
}

// ThreeWayMerge 在当前分支/Worktree 上执行 Git 三路合并（用于基因杂交合并两个正交成功分支）
// 若存在代码行冲突，自动中止脏合并状态并返回冲突详情，交由上层 Agent 语义融合。
func ThreeWayMerge(ctx context.Context, workDir, secondaryCommit, message string) (mergedSHA string, conflict bool, detail string, err error) {
	out, mergeErr := runGit(ctx, workDir, "merge", "--no-ff", "-m", message, secondaryCommit)
	if mergeErr != nil {
		statusOut, _ := runGit(ctx, workDir, "diff", "--name-only", "--diff-filter=U")
		_, _ = runGit(ctx, workDir, "merge", "--abort")
		if statusOut != "" || strings.Contains(mergeErr.Error(), "CONFLICT") {
			return "", true, statusOut + "\n" + mergeErr.Error(), nil
		}
		return "", false, out, mergeErr
	}
	sha, shaErr := HeadSHA(ctx, workDir)
	return sha, false, out, shaErr
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
