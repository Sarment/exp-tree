import type { Plugin } from "@opencode-ai/plugin";
import { ExpTreeClient } from "../shared/client";

/**
 * OpenCode 中间件：无感记录每次有效开发步骤，驱动当前探索树节点的进度百分比。
 */
export const ExpTreeOpenCodePlugin: Plugin = async () => {
  const client = new ExpTreeClient(
    process.env.ETREE_AGENT_ID ?? `opencode-${process.pid}`
  );

  return {
    "tool.execute.after": async ({ tool }) => {
      if (["write", "edit", "patch", "bash"].includes(tool)) {
        await client.recordToolStep(tool, `completed ${tool}`, true);
      }
    },
  };
};

export default ExpTreeOpenCodePlugin;
