import { ExpTreeClient } from "../shared/client";

export const name = "dsh-plugin-exp-tree";
export const inject = ["tools"];

export function apply(ctx: {
  tools: { register: (def: unknown) => void };
  on: (
    evt: string,
    fn: (c: { tool: string; arguments: string; abort: (r: string) => void }) => Promise<void> | void
  ) => void;
}) {
  const client = new ExpTreeClient(
    process.env.ETREE_AGENT_ID ?? `dsh-${process.pid}`
  );

  // 无感采集历史操作步骤，量化当前科研节点执行进度
  ctx.on("tools/post-execute", async (execCtx) => {
    await client.recordToolStep(execCtx.tool, `executed ${execCtx.tool}`, true);
  });
}
