export class ExpTreeClient {
  private readonly endpoint: string;
  public readonly agentId: string;

  constructor(agentId: string, endpoint?: string) {
    this.agentId = agentId;
    this.endpoint = endpoint ?? process.env.ETREE_URL ?? "http://127.0.0.1:7421";
  }

  /**
   * 插件 Hook 无感调用：自动记录 Agent 执行过的关键工具步骤与客观验证状态
   */
  async recordToolStep(toolName: string, summary: string, verified = true) {
    try {
      await fetch(`${this.endpoint}/v1/tree/step`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          node_id: "",
          action: `[${toolName}] ${summary}`,
          verified,
        }),
      });
    } catch {
      // 离线静默降级
    }
  }

  async fetchBrief(nodeId = ""): Promise<string> {
    try {
      const res = await fetch(
        `${this.endpoint}/v1/tree/brief?node_id=${encodeURIComponent(nodeId)}`
      );
      if (!res.ok) return "";
      const data = (await res.json()) as { brief?: string };
      return data.brief ?? "";
    } catch {
      return "";
    }
  }
}
