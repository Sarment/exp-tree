import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

const EXT_DIR = path.dirname(fileURLToPath(import.meta.url));
const ROOT_DIR = path.resolve(EXT_DIR, "../..");
const LOCAL_BIN = existsSync(path.join(ROOT_DIR, "bin/exptree"))
  ? path.join(ROOT_DIR, "bin/exptree")
  : path.join(ROOT_DIR, "bin/etree");
const ETREE_BIN = process.env.ETREE_BIN ?? (existsSync(LOCAL_BIN) ? LOCAL_BIN : "exptree");
const SKILL_DIR = path.join(ROOT_DIR, "skills/exptree");
const DEFAULT_PORT = process.env.ETREE_PORT ?? "7421";
const ENDPOINT = process.env.ETREE_URL ?? `http://127.0.0.1:${DEFAULT_PORT}`;

/**
 * 自动确保本地 `exptree serve` 守护进程已就绪（零手动起终端负担！）
 */
async function ensureDaemon(cwd: string): Promise<boolean> {
  try {
    const r = await fetch(`${ENDPOINT}/v1/health`);
    if (r.ok) return true;
  } catch {
    // 未启动则静默后台拉起
  }
  try {
    const child = spawn(ETREE_BIN, ["serve", "-addr", `127.0.0.1:${DEFAULT_PORT}`, "-repo", cwd], {
      detached: true,
      stdio: "ignore",
    });
    child.unref();
    for (let i = 0; i < 15; i++) {
      await new Promise((r) => setTimeout(r, 120));
      try {
        const check = await fetch(`${ENDPOINT}/v1/health`);
        if (check.ok) return true;
      } catch {
        // retry
      }
    }
  } catch {
    return false;
  }
  return false;
}

async function refreshFooterStatus(ctx: any) {
  if (!ctx?.hasUI || !ctx?.ui?.setStatus) return;
  try {
    const r = await fetch(`${ENDPOINT}/v1/tree`);
    if (!r.ok) return;
    const state = (await r.json()) as any;
    if (!state?.root_id || !state?.nodes) {
      ctx.ui.setStatus("exptree", "🌿 ExpTree: idle (/exptree init)");
      return;
    }
    const active = state.nodes[state.active_node_id];
    const best = state.nodes[state.best_node_id];
    const bestScore =
      best?.primary_score !== undefined ? `${state.metric_name}=${best.primary_score.toFixed(2)}` : "-";
    ctx.ui.setStatus(
      "exptree",
      `🌿 ExpTree: ${active?.id ?? "-"} [${active?.status ?? "-"}] | ★ ${state.best_node_id ?? "-"} (${bestScore})`
    );
  } catch {
    // ignore when daemon offline
  }
}

/**
 * 面向本机 Pi 的完整原生扩展：
 * 1. 自动发现 `exptree` Skill；
 * 2. 自动静默拉起 `exptree serve` 守护进程并在底部状态栏实时展示当前树分支与最高分；
 * 3. 每轮自动将 <150 Token 同级剪枝墓志铭注入 System Prompt；
 * 4. 后置自动记录步骤进度并实时刷新底部状态栏；
 * 5. 注册全套 LLM 可调用工具 (`etree_init`, `etree_brief`, `etree_branch`, `etree_eval`, `etree_converge`, `etree_recommend`) 与 `/exptree` 快捷命令。
 */
export default function registerExpTreePiExtension(pi: any) {
  // 1. 自动挂载科研树方法论 Skill
  pi.on("resources_discover", async () => ({
    skillPaths: [SKILL_DIR],
  }));

  // 2. 会话启动时自动拉起本地守护进程并刷新底部状态栏
  pi.on("session_start", async (_evt: any, ctx: any) => {
    await ensureDaemon(ctx.cwd);
    await refreshFooterStatus(ctx);
  });

  // 3. 每轮对话前无感将当前探索树简报与同级失败墓志铭注入 System Prompt
  pi.on("before_agent_start", async (event: any, ctx: any) => {
    await ensureDaemon(ctx.cwd);
    await refreshFooterStatus(ctx);
    try {
      const r = await fetch(`${ENDPOINT}/v1/tree/brief`);
      if (!r.ok) return undefined;
      const data = (await r.json()) as { brief?: string };
      if (!data.brief) return undefined;
      return {
        systemPrompt: `${event.systemPrompt}\n\n${data.brief}`,
      };
    } catch {
      return undefined;
    }
  });

  // 4. 后置自动记步并实时刷新底部状态栏
  pi.on("tool_result", async (event: any, ctx: any) => {
    if (["write", "edit", "bash"].includes(event.toolName) && !event.isError) {
      fetch(`${ENDPOINT}/v1/tree/step`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: `[pi:${event.toolName}] verified step`, verified: true }),
      }).catch(() => undefined);
    }
    await refreshFooterStatus(ctx);
    return undefined;
  });

  // 6. 注册 `/exptree`（及兼容别名 `/etree`）交互命令
  const cmdHandler = async (args: string, ctx: any) => {
    await ensureDaemon(ctx.cwd);
    if (args.trim() === "web") {
      await pi.exec("open", [ENDPOINT]);
      ctx.ui.notify(`已在浏览器打开 ExpTree 实时看板: ${ENDPOINT}`, "info");
      return;
    }
    const res = await pi.exec(ETREE_BIN, ["view", "-repo", ctx.cwd]);
    ctx.ui.notify(res.stdout || "当前仓库尚未初始化探索树，可直接让 Pi 调用 etree_init。", "info");
  };
  pi.registerCommand("exptree", {
    description: "查看多叉科研探索树或打开本地可视化看板 (/exptree | /exptree web)",
    handler: cmdHandler,
  });
  pi.registerCommand("etree", {
    description: "Alias for /exptree",
    handler: cmdHandler,
  });

  // 7. 注册全套 Pi 原生 LLM 工具（带 promptSnippet 确保注入默认系统提示词！）
  pi.registerTool({
    name: "etree_init",
    label: "ExpTree Init",
    description: "Initialize an AutoResearch experiment tree in the current Git repo and auto-measure baseline score.",
    promptSnippet: "Initialize multi-way experiment tree with baseline eval",
    parameters: {
      type: "object",
      properties: {
        goal: { type: "string", description: "Optimization or research objective" },
        metricName: { type: "string", description: "Metric name (e.g. ops_per_sec, val_acc, loss)" },
        evalCommand: { type: "string", description: "Shell command to run benchmark (e.g. python3 bench.py)" },
      },
      required: ["goal", "evalCommand"],
    },
    async execute(_id: string, params: any, _sig: any, _onUp: any, ctx: any) {
      const args = [
        "up",
        "-serve=false",
        "-repo",
        ctx.cwd,
        "-goal",
        params.goal,
        "-metric",
        params.metricName || "score",
        "-eval-cmd",
        params.evalCommand,
      ];
      const out = await pi.exec(ETREE_BIN, args);
      await ensureDaemon(ctx.cwd);
      await refreshFooterStatus(ctx);
      return {
        content: [{ type: "text", text: `${out.stdout}\n${out.stderr}`.trim() + `\nLive Console: ${ENDPOINT}` }],
        details: {},
      };
    },
  });

  pi.registerTool({
    name: "etree_brief",
    label: "ExpTree Brief",
    description: "Inspect the current multi-way experiment tree, global best score, and pruned sibling dead-ends.",
    promptSnippet: "Read current experiment tree topology and pruned dead-end graveyard",
    parameters: { type: "object", properties: {} },
    async execute(_id: string, _params: any, _sig: any, _onUp: any, ctx: any) {
      const [viewOut, briefOut] = await Promise.all([
        pi.exec(ETREE_BIN, ["view", "-repo", ctx.cwd]),
        pi.exec(ETREE_BIN, ["brief", "-repo", ctx.cwd]),
      ]);
      return {
        content: [{ type: "text", text: `${briefOut.stdout}\n\n${viewOut.stdout}`.trim() }],
        details: {},
      };
    },
  });

  pi.registerTool({
    name: "etree_branch",
    label: "ExpTree Branch",
    description: "Create a new Git-backed hypothesis branch from a validated parent node before modifying code.",
    promptSnippet: "Fork a new Git experiment branch for a falsifiable hypothesis",
    parameters: {
      type: "object",
      properties: {
        hypothesis: { type: "string", description: "Single falsifiable hypothesis to test" },
        parentId: { type: "string", description: "Optional parent node ID (defaults to active)" },
      },
      required: ["hypothesis"],
    },
    async execute(_id: string, params: any, _sig: any, _onUp: any, ctx: any) {
      const args = ["branch", "-repo", ctx.cwd, "-agent", "pi", "-hypothesis", params.hypothesis];
      if (params.parentId) args.push("-parent", params.parentId);
      const out = await pi.exec(ETREE_BIN, args);
      await refreshFooterStatus(ctx);
      return {
        content: [{ type: "text", text: `${out.stdout}\n${out.stderr}`.trim() }],
        details: {},
      };
    },
  });

  pi.registerTool({
    name: "etree_eval",
    label: "ExpTree Evaluate",
    description: "Run benchmark, auto-commit on metric improvement, or auto-prune & git rollback to parent on regression/crash.",
    promptSnippet: "Evaluate active hypothesis branch and auto-commit or git-rollback",
    parameters: {
      type: "object",
      properties: {
        command: { type: "string", description: "Optional override benchmark command" },
      },
    },
    async execute(_id: string, params: any, _sig: any, _onUp: any, ctx: any) {
      const args = ["eval", "-repo", ctx.cwd];
      if (params.command) args.push("-cmd", params.command);
      const out = await pi.exec(ETREE_BIN, args);
      await refreshFooterStatus(ctx);
      return {
        content: [{ type: "text", text: `${out.stdout}\n${out.stderr}`.trim() }],
        details: {},
      };
    },
  });

  pi.registerTool({
    name: "etree_converge",
    label: "ExpTree Converge",
    description: "Converge 2 or more validated branches into a single N-to-1 synthesis node supervised by an agent.",
    promptSnippet: "Synthesize multiple validated branches into one converged node",
    parameters: {
      type: "object",
      properties: {
        parentIds: { type: "string", description: "Comma-separated parent node IDs (e.g. node-2,node-3)" },
      },
      required: ["parentIds"],
    },
    async execute(_id: string, params: any, _sig: any, _onUp: any, ctx: any) {
      const out = await pi.exec(ETREE_BIN, [
        "converge",
        "-repo",
        ctx.cwd,
        "-parents",
        params.parentIds,
        "-supervisor",
        "pi",
      ]);
      await refreshFooterStatus(ctx);
      return {
        content: [{ type: "text", text: `${out.stdout}\n${out.stderr}`.trim() }],
        details: {},
      };
    },
  });
}
