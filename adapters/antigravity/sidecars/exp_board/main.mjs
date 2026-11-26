import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, readdirSync, renameSync, statSync, unlinkSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { basename, dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { SidecarApp, Response } from 'sidecar_sdk';

const __dirname = dirname(fileURLToPath(import.meta.url));
const APP_DATA_DIR = process.env.ANTIGRAVITY_APP_DATA_DIR || join(homedir(), '.gemini', 'antigravity');
const SCRATCH_ROOT = join(APP_DATA_DIR, 'scratch');
const BRAIN_ROOT = join(APP_DATA_DIR, 'brain');
const BINDINGS_DIR = join(SCRATCH_ROOT, '.exptree_bindings');
const CONV_RE = /^[0-9a-f-]{8,64}$/i;
const isValidConvId = (id) => CONV_RE.test(String(id || '').trim());

const NO_CACHE = { 'Cache-Control': 'no-store, no-cache, must-revalidate' };
const STATIC_FILES = ['index.html', 'styles.css', 'app.js'];

function bindingPath(convId) {
  const clean = String(convId || '').trim();
  return isValidConvId(clean) ? join(BINDINGS_DIR, `${clean}.json`) : '';
}

function readBinding(convId) {
  const p = bindingPath(convId);
  if (!p || !existsSync(p)) return null;
  try {
    return JSON.parse(readFileSync(p, 'utf-8'));
  } catch {
    return null;
  }
}

function writeBindingAtomic(convId, obj) {
  const p = bindingPath(convId);
  if (!p) throw new Error('Invalid conversationId');
  mkdirSync(BINDINGS_DIR, { recursive: true });
  const tmp = `${p}.tmp.${process.pid}.${Date.now()}`;
  writeFileSync(tmp, JSON.stringify(obj, null, 2), 'utf-8');
  renameSync(tmp, p);
}

function readRepoOwnerConv(repoPath) {
  const metaPath = join(repoPath, '.git', 'exp-tree', 'binding.json');
  if (!existsSync(metaPath)) return '';
  try {
    const meta = JSON.parse(readFileSync(metaPath, 'utf-8'));
    const owner = String(meta.conversation_id || '').trim();
    if (!isValidConvId(owner)) return '';
    const ob = readBinding(owner);
    if (!ob?.repo || resolve(ob.repo) !== resolve(repoPath)) return '';
    return owner;
  } catch {
    return '';
  }
}

function writeRepoOwnerConv(repoPath, convId, boundAt, now) {
  const gitDir = join(repoPath, '.git');
  try {
    if (!existsSync(gitDir) || !statSync(gitDir).isDirectory()) return;
    const stateDir = join(gitDir, 'exp-tree');
    mkdirSync(stateDir, { recursive: true });
    writeFileSync(join(stateDir, 'binding.json'), JSON.stringify({
      conversation_id: convId,
      repo: repoPath,
      bound_at: boundAt,
      updated_at: now,
    }, null, 2), 'utf-8');
  } catch {
    // ignore
  }
}

function deleteBinding(convId) {
  const existing = readBinding(convId);
  if (existing?.repo) {
    const metaPath = join(existing.repo, '.git', 'exp-tree', 'binding.json');
    try {
      if (existsSync(metaPath)) {
        const meta = JSON.parse(readFileSync(metaPath, 'utf-8'));
        if (String(meta.conversation_id || '').trim() === String(convId || '').trim()) {
          unlinkSync(metaPath);
        }
      }
    } catch {
      // ignore
    }
  }
  const p = bindingPath(convId);
  if (!p) return;
  try {
    unlinkSync(p);
  } catch (err) {
    if (err?.code !== 'ENOENT') throw err;
  }
}

function getBuildId() {
  return STATIC_FILES.map((f) => {
    try {
      return Math.round(statSync(join(__dirname, f)).mtimeMs);
    } catch {
      return 0;
    }
  }).join('-');
}

// Fallback conversation resolver when sidecar:// deep-links omit ?conversationId=
function resolveActiveConversationId(explicitId) {
  if (explicitId && /^[0-9a-f-]{8,}$/i.test(explicitId)) return explicitId;
  if (process.env.ANTIGRAVITY_CONVERSATION_ID) return process.env.ANTIGRAVITY_CONVERSATION_ID;
  if (!existsSync(BRAIN_ROOT)) return '';
  let bestId = '';
  let bestMtime = 0;
  try {
    for (const entry of readdirSync(BRAIN_ROOT)) {
      if (!/^[0-9a-f-]{8,}$/i.test(entry)) continue;
      const logPath = join(BRAIN_ROOT, entry, '.system_generated', 'logs', 'transcript.jsonl');
      try {
        const st = statSync(logPath);
        if (st.mtimeMs > bestMtime) {
          bestMtime = st.mtimeMs;
          bestId = entry;
        }
      } catch {
        // skip
      }
    }
  } catch {
    // skip
  }
  return bestId;
}

function uriToPath(uri) {
  if (!uri) return '';
  if (uri.startsWith('file://')) {
    try {
      return decodeURIComponent(new URL(uri).pathname);
    } catch {
      return uri.replace(/^file:\/\//, '');
    }
  }
  return uri;
}

function summarizeRepo(repoPath, isWorkspace = false) {
  const statePath = join(repoPath, '.git', 'exp-tree', 'state.json');
  const hasTree = existsSync(statePath);
  const rel = repoPath.startsWith(SCRATCH_ROOT + '/')
    ? relative(SCRATCH_ROOT, repoPath)
    : basename(repoPath);
  const info = {
    path: repoPath,
    label: rel || basename(repoPath),
    hasTree,
    isWorkspace,
    nodeCount: 0,
    goal: '',
    metricName: '',
    updatedAtMs: 0,
  };
  if (hasTree) {
    try {
      const stat = statSync(statePath);
      info.updatedAtMs = stat.mtimeMs;
      const st = JSON.parse(readFileSync(statePath, 'utf-8'));
      const nodes = Object.values(st.nodes || {});
      info.nodeCount = nodes.length;
      info.goal = st.goal || '';
      info.metricName = st.metric_name || '';
      info.higherIsBetter = Boolean(st.higher_is_better);
      const best = st.best_node_id && st.nodes?.[st.best_node_id];
      const root = st.root_id && st.nodes?.[st.root_id];
      if (best && typeof best.primary_score === 'number') info.bestScore = best.primary_score;
      if (root && typeof root.primary_score === 'number') info.baselineScore = root.primary_score;
    } catch {
      // ignore
    }
  }
  return info;
}

function scanGitRepos(baseDir, depth = 0, maxDepth = 3, found = []) {
  if (depth > maxDepth || !existsSync(baseDir)) return found;
  try {
    if (existsSync(join(baseDir, '.git'))) {
      found.push(baseDir);
    }
    if (depth === maxDepth) return found;
    for (const entry of readdirSync(baseDir)) {
      if (entry.startsWith('.') || entry === 'node_modules' || entry === 'bin') continue;
      const full = join(baseDir, entry);
      try {
        if (statSync(full).isDirectory()) {
          scanGitRepos(full, depth + 1, maxDepth, found);
        }
      } catch {
        // skip
      }
    }
  } catch {
    // skip
  }
  return found;
}

function parseUnifiedDiff(raw) {
  if (!raw || !raw.trim()) return [];
  const files = [];
  let curFile = null;
  let curHunk = null;
  let oldLine = 0;
  let newLine = 0;

  for (const line of raw.split('\n')) {
    if (line.startsWith('diff --git ')) {
      const m = /^diff --git a\/(.+?) b\/(.+)$/.exec(line);
      const nextPath = m ? m[2] : line.slice(11);
      if (nextPath.includes('__pycache__') || nextPath.endsWith('.pyc')) {
        curFile = null;
        curHunk = null;
        continue;
      }
      curFile = {
        oldPath: m ? m[1] : '',
        path: nextPath,
        status: 'modified',
        additions: 0,
        deletions: 0,
        hunks: [],
      };
      files.push(curFile);
      curHunk = null;
      continue;
    }
    if (!curFile) continue;
    if (line.startsWith('new file mode')) { curFile.status = 'added'; continue; }
    if (line.startsWith('deleted file mode')) { curFile.status = 'deleted'; continue; }
    if (line.startsWith('rename from ')) { curFile.status = 'renamed'; curFile.oldPath = line.slice(12); continue; }
    if (line.startsWith('rename to ')) { curFile.path = line.slice(10); continue; }
    if (
      line.startsWith('index ') ||
      line.startsWith('--- ') ||
      line.startsWith('+++ ') ||
      line.startsWith('similarity index')
    ) {
      continue;
    }
    if (line.startsWith('@@ ')) {
      const hm = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$/.exec(line);
      oldLine = hm ? parseInt(hm[1], 10) : 1;
      newLine = hm ? parseInt(hm[2], 10) : 1;
      curHunk = {
        header: line,
        context: hm && hm[3] ? hm[3].trim() : '',
        lines: [],
      };
      curFile.hunks.push(curHunk);
      continue;
    }
    if (!curHunk) continue;
    if (line.startsWith('+')) {
      curFile.additions += 1;
      curHunk.lines.push({ type: 'add', oldNo: null, newNo: newLine++, content: line.slice(1) });
    } else if (line.startsWith('-')) {
      curFile.deletions += 1;
      curHunk.lines.push({ type: 'del', oldNo: oldLine++, newNo: null, content: line.slice(1) });
    } else if (line.startsWith(' ')) {
      curHunk.lines.push({ type: 'ctx', oldNo: oldLine++, newNo: newLine++, content: line.slice(1) });
    } else if (line.startsWith('\\ No newline')) {
      curHunk.lines.push({ type: 'meta', oldNo: null, newNo: null, content: line });
    }
  }
  return files;
}

const app = new SidecarApp();

app.page('/', () => new Response(readFileSync(join(__dirname, 'index.html'), 'utf-8'), {
  contentType: 'text/html; charset=utf-8',
  headers: NO_CACHE,
}));

app.page('/styles.css', () => new Response(readFileSync(join(__dirname, 'styles.css'), 'utf-8'), {
  contentType: 'text/css; charset=utf-8',
  headers: NO_CACHE,
}));

app.page('/app.js', () => new Response(readFileSync(join(__dirname, 'app.js'), 'utf-8'), {
  contentType: 'application/javascript; charset=utf-8',
  headers: NO_CACHE,
}));

function listRepos(workspacesRaw = '', currentConvId = '') {
  const wsList = String(workspacesRaw || '')
    .split(',')
    .map((u) => uriToPath(u.trim()))
    .filter(Boolean);

  const map = new Map();
  for (const p of wsList) {
    const abs = resolve(p);
    if (existsSync(abs)) {
      map.set(abs, summarizeRepo(abs, true));
    }
  }
  for (const p of scanGitRepos(SCRATCH_ROOT, 0, 3)) {
    const abs = resolve(p);
    if (!map.has(abs)) {
      map.set(abs, summarizeRepo(abs, false));
    }
  }

  const repos = Array.from(map.values()).map((info) => {
    const owner = readRepoOwnerConv(info.path);
    return {
      ...info,
      boundByConvId: owner,
      boundByOther: Boolean(owner && currentConvId && owner !== currentConvId),
    };
  });

  // Sort: active workspaces first, then repos with trees ordered by most recently modified!
  repos.sort((a, b) => {
    if (a.isWorkspace !== b.isWorkspace) return a.isWorkspace ? -1 : 1;
    if (a.hasTree !== b.hasTree) return a.hasTree ? -1 : 1;
    if ((b.updatedAtMs || 0) !== (a.updatedAtMs || 0)) return (b.updatedAtMs || 0) - (a.updatedAtMs || 0);
    return a.label.localeCompare(b.label);
  });
  return repos;
}

app.api('/api/repos', (data) => {
  const convId = isValidConvId(data.conversationId) ? String(data.conversationId).trim() : '';
  return { repos: listRepos(data.workspaces || '', convId), buildId: getBuildId() };
}, 'GET');

app.api('/api/bound', (data) => {
  // Never fall back to newest transcript in BRAIN_ROOT here; unbound conversations must stay isolated!
  const convId = isValidConvId(data.conversationId)
    ? String(data.conversationId).trim()
    : (isValidConvId(process.env.ANTIGRAVITY_CONVERSATION_ID) ? String(process.env.ANTIGRAVITY_CONVERSATION_ID).trim() : '');
  let binding = convId ? readBinding(convId) : null;
  if (binding?.repo && !existsSync(binding.repo)) {
    deleteBinding(convId);
    binding = null;
  }
  const boundRepo = binding?.repo || '';
  return {
    conversationId: convId,
    standalone: false,
    bound: Boolean(boundRepo),
    repo: boundRepo,
    repoInfo: boundRepo ? summarizeRepo(boundRepo, false) : null,
    focus: binding || null,
    repos: listRepos(data.workspaces || '', convId),
    buildId: getBuildId(),
  };
}, 'GET');

app.api('/api/bind', (data) => {
  const convId = isValidConvId(data.conversationId)
    ? String(data.conversationId).trim()
    : (isValidConvId(process.env.ANTIGRAVITY_CONVERSATION_ID) ? String(process.env.ANTIGRAVITY_CONVERSATION_ID).trim() : '');
  if (!convId) {
    throw new Error('缺少有效的对话 ID (conversationId)');
  }
  const repoRaw = String(data.repo || '').trim();
  if (!repoRaw) {
    deleteBinding(convId);
    return { ok: true, bound: false, conversationId: convId };
  }
  const absRepo = resolve(repoRaw);
  if (!existsSync(absRepo)) {
    throw new Error('目标仓库路径不存在');
  }
  const now = new Date().toISOString();
  const prev = readBinding(convId);
  const sameRepo = Boolean(prev?.repo && resolve(prev.repo) === absRepo);
  const next = {
    conversation_id: convId,
    repo: absRepo,
    selected_node_id: sameRepo ? String(prev.selected_node_id || '') : '',
    selected_node_ids: sameRepo && Array.isArray(prev.selected_node_ids) ? prev.selected_node_ids : [],
    active_tab: sameRepo ? String(prev.active_tab || 'tree') : 'tree',
    bound_at: (sameRepo && prev.bound_at) ? prev.bound_at : now,
    updated_at: now,
  };
  writeBindingAtomic(convId, next);
  writeRepoOwnerConv(absRepo, convId, next.bound_at, now);
  return {
    ok: true,
    bound: true,
    conversationId: convId,
    repo: absRepo,
    repoInfo: summarizeRepo(absRepo, false),
  };
}, 'GET');

app.api('/api/state', (data) => {
  const buildId = getBuildId();
  const repo = data.repo || '';
  if (!repo || !existsSync(repo)) {
    return { initialized: false, state: null, buildId };
  }
  const statePath = join(repo, '.git', 'exp-tree', 'state.json');
  if (!existsSync(statePath)) {
    return { initialized: false, repo, state: null, buildId };
  }
  const st = statSync(statePath);
  const etag = `${buildId}|${repo}|${st.mtimeMs}|${st.size}`;
  if (data.etag && data.etag === etag) {
    return { unchanged: true, etag, buildId };
  }
  const state = JSON.parse(readFileSync(statePath, 'utf-8'));
  return { initialized: true, repo, state, etag, buildId };
}, 'GET');

app.api('/api/diff', (data) => {
  const repo = data.repo || '';
  const toId = data.to || '';
  const fromOverride = data.from || '';
  const statePath = join(repo, '.git', 'exp-tree', 'state.json');
  if (!existsSync(statePath)) return { files: [], rawDiff: '', fromLabel: '', toLabel: '' };

  const state = JSON.parse(readFileSync(statePath, 'utf-8'));
  const toNode = state.nodes?.[toId];
  if (!toNode) return { files: [], rawDiff: '', error: 'Node not found' };

  const pids = Array.isArray(toNode.parent_ids) && toNode.parent_ids.length
    ? toNode.parent_ids
    : (toNode.parent_id ? [toNode.parent_id] : []);
  const compareTargets = [];
  for (const pid of pids) {
    if (state.nodes?.[pid]) {
      compareTargets.push({ id: pid, role: 'parent', commit: (state.nodes[pid].git_commit || '').slice(0, 7) });
    }
  }
  if (state.root_id && state.root_id !== toId && !pids.includes(state.root_id) && state.nodes?.[state.root_id]) {
    compareTargets.push({
      id: state.root_id,
      role: 'baseline',
      commit: (state.nodes[state.root_id].git_commit || '').slice(0, 7),
    });
  }

  const fromId = fromOverride || pids[0] || '';
  const parentNode = fromId ? state.nodes?.[fromId] : null;
  const isPruned = toNode.status === 'PRUNED';

  if (!parentNode) {
    if (!toNode.git_commit) {
      return { files: [], rawDiff: '', fromId: '', fromLabel: 'empty', toLabel: toNode.id, compareTargets };
    }
    try {
      const rawDiff = execFileSync(
        'git',
        ['-C', repo, 'show', '--format=', '--unified=3', toNode.git_commit],
        { encoding: 'utf-8', maxBuffer: 4 * 1024 * 1024 }
      );
      return {
        files: parseUnifiedDiff(rawDiff),
        rawDiff,
        fromId: '',
        fromLabel: 'Initial Snapshot',
        toLabel: `${toNode.id} (${toNode.git_commit.slice(0, 7)})`,
        compareTargets,
      };
    } catch {
      return { files: [], rawDiff: '', fromId: '', fromLabel: 'Initial', toLabel: toNode.id, compareTargets };
    }
  }

  // Legacy pruned nodes created before commit-preservation had parentNode.git_commit === toNode.git_commit
  if (parentNode.git_commit === toNode.git_commit && isPruned) {
    return {
      files: [],
      rawDiff: '',
      isPruned: true,
      prunedLegacyNoCommit: true,
      pruneDigest: toNode.prune_digest || '早期剪枝节点未保存代码快照；新剪枝节点均自动保留失败 Commit。',
      fromId: parentNode.id,
      fromLabel: `${parentNode.id} (${(parentNode.git_commit || '').slice(0, 7)})`,
      toLabel: `${toNode.id} (旧版未存快照)`,
      compareTargets,
    };
  }

  const targetDir = toNode.worktree_path && existsSync(toNode.worktree_path) ? toNode.worktree_path : repo;
  try {
    const baseArgs = toNode.git_commit && toNode.git_commit !== parentNode.git_commit
      ? ['-C', repo, 'diff', '--unified=3', parentNode.git_commit, toNode.git_commit]
      : ['-C', targetDir, 'diff', '--unified=3', parentNode.git_commit];
    const rawDiff = execFileSync('git', baseArgs, { encoding: 'utf-8', maxBuffer: 4 * 1024 * 1024 });
    return {
      files: parseUnifiedDiff(rawDiff),
      rawDiff,
      isPruned,
      pruneDigest: toNode.prune_digest || '',
      fromId: parentNode.id,
      fromLabel: `${parentNode.id} (${(parentNode.git_commit || '').slice(0, 7)})`,
      toLabel: `${toNode.id} (${(toNode.git_commit || 'working').slice(0, 7)})`,
      compareTargets,
    };
  } catch (err) {
    return { files: [], rawDiff: '', isPruned, pruneDigest: toNode.prune_digest || '', error: String(err.message || err), compareTargets };
  }
}, 'GET');

// Reliable chat message sender that resolves conversationId even after sidecar:// navigation!
// Using GET-compatible or POST? Note: if ?token= was stripped on sidecar:// link navigation,
// POST would 401 in SidecarApp.#validateToken! Making localhost loopback /api/chat accept GET/POST
// or ensuring we also expose a safe loopback endpoint guarantees zero 401 when navigating via sidecar://!
app.api('/api/chat', (data) => {
  const convId = resolveActiveConversationId(data.conversationId || '');
  if (!convId) throw new Error('未能定位当前活跃对话 ID');
  const message = String(data.message || '').trim();
  if (!message) throw new Error('消息内容为空');
  return app.sendMessage(convId, message, { projectId: process.env.ANTIGRAVITY_PROJECT_ID });
}, 'GET');

app.api('/api/focus', (data) => {
  const convId = isValidConvId(data.conversationId) ? String(data.conversationId).trim() : '';
  const repo = String(data.repo || '');
  const selectedNodeId = String(data.node || '');
  const multi = String(data.nodes || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  const tab = String(data.tab || 'tree');
  const now = new Date().toISOString();

  if (convId) {
    const existing = readBinding(convId);
    const absRepo = repo ? resolve(repo) : '';
    if (existing && (!absRepo || resolve(existing.repo || '') === absRepo)) {
      writeBindingAtomic(convId, {
        conversation_id: convId,
        repo: existing.repo || absRepo,
        selected_node_id: selectedNodeId,
        selected_node_ids: multi,
        active_tab: tab || existing.active_tab || 'tree',
        bound_at: existing.bound_at || now,
        updated_at: now,
      });
    }
    if (repo && existsSync(join(repo, '.git'))) {
      try {
        if (statSync(join(repo, '.git')).isDirectory()) {
          const stateDir = join(repo, '.git', 'exp-tree');
          mkdirSync(stateDir, { recursive: true });
          writeFileSync(join(stateDir, 'ui_focus.json'), JSON.stringify({
            conversation_id: convId,
            repo,
            selected_node_id: selectedNodeId,
            selected_node_ids: multi,
            active_tab: tab,
            updated_at: now,
          }, null, 2), 'utf-8');
        }
      } catch { /* ignore */ }
    }
    return { ok: true, conversationId: convId };
  }

  const payload = JSON.stringify({
    repo,
    selected_node_id: selectedNodeId,
    selected_node_ids: multi,
    active_tab: tab,
    updated_at: now,
  }, null, 2);

  try {
    mkdirSync(SCRATCH_ROOT, { recursive: true });
    writeFileSync(join(SCRATCH_ROOT, '.exptree_ui_focus.json'), payload, 'utf-8');
  } catch { /* ignore */ }

  if (repo && existsSync(join(repo, '.git'))) {
    try {
      if (statSync(join(repo, '.git')).isDirectory()) {
        const stateDir = join(repo, '.git', 'exp-tree');
        mkdirSync(stateDir, { recursive: true });
        writeFileSync(join(stateDir, 'ui_focus.json'), payload, 'utf-8');
      }
    } catch { /* ignore */ }
  }
  return { ok: true };
}, 'GET');

app.run();
