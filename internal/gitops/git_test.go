package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sarment/exp-tree/internal/tree"
)

// TestGitRollbackPreservesExpTreeState 验证剪枝执行 `git reset --hard` 回退失败代码时，
// 工作区代码干净恢复到父节点，而 `.git/exp-tree/state.json` 中的剪枝记忆毫发无损。
func TestGitRollbackPreservesExpTreeState(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	if _, err := runGit(ctx, repo, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	_, _ = runGit(ctx, repo, "config", "user.email", "agent@exptree.local")
	_, _ = runGit(ctx, repo, "config", "user.name", "ExpTree Agent")

	trainFile := filepath.Join(repo, "train.py")
	baselineCode := "lr = 1e-3\n"
	_ = os.WriteFile(trainFile, []byte(baselineCode), 0o644)

	rootSHA, err := CommitSnapshot(ctx, repo, "baseline")
	if err != nil {
		t.Fatalf("commit baseline: %v", err)
	}

	eng, err := tree.OpenEngine(DefaultStatePath(repo))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	baseScore := 0.80
	_, _ = eng.InitRoot("Optimize train.py", "val_acc", true, rootSHA, &baseScore)

	// 派生实验分支并修改代码（模拟一次失败的科研尝试）
	node1, _ := eng.Branch(tree.BranchInput{
		AgentID:    "pi-researcher",
		Hypothesis: "Set lr=10.0",
		GitBranch:  "exp/node-1",
	})
	_, _ = CheckoutNewBranch(ctx, repo, "exp/node-1", rootSHA)
	_ = os.WriteFile(trainFile, []byte("lr = 10.0 # diverges!\n"), 0o644)

	// 剪枝并执行 Git 强回退回父节点 Commit
	_, backNode, _ := eng.Prune(node1.ID, "Diverged with NaN loss", nil)
	if err := HardRollbackTo(ctx, repo, backNode.GitCommit); err != nil {
		t.Fatalf("hard rollback: %v", err)
	}

	// 1. 验证工作区 train.py 已彻底还原为基线代码
	gotCode, _ := os.ReadFile(trainFile)
	if string(gotCode) != baselineCode {
		t.Fatalf("expected train.py reverted to %q, got %q", baselineCode, string(gotCode))
	}

	// 2. 验证重新加载引擎时，失败墓志铭依然完整保留在 .git/exp-tree/state.json 中
	reloaded, _ := tree.OpenEngine(DefaultStatePath(repo))
	prunedNode, err := reloaded.GetNode(node1.ID)
	if err != nil || prunedNode.Status != tree.NodePruned || prunedNode.PruneDigest != "Diverged with NaN loss" {
		t.Fatalf("expected pruned node preserved across git reset, got %+v (err=%v)", prunedNode, err)
	}
}
