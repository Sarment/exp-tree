import { createHash } from 'node:crypto';
import { execFile, execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, readdirSync, renameSync, statSync, unlinkSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { basename, dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import { SidecarApp, Response } from 'sidecar_sdk';

const execFileAsync = promisify(execFile);

const __dirname = dirname(fileURLToPath(import.meta.url));
const APP_DATA_DIR = process.env.ANTIGRAVITY_APP_DATA_DIR || join(homedir(), '.gemini', 'antigravity');
const SCRATCH_ROOT = join(APP_DATA_DIR, 'scratch');
const BRAIN_ROOT = join(APP_DATA_DIR, 'brain');
const BINDINGS_DIR = join(SCRATCH_ROOT, '.exptree_bindings');
const CACHE_DIR = join(SCRATCH_ROOT, '.exptree_cache');
const DIFF_CACHE_DIR = join(CACHE_DIR, 'diffs');
const SSH_SOCK_DIR = join(SCRATCH_ROOT, '.exptree_ssh');
const IMMUTABLE_DIFF_MEM = new Map();
const ACTIVE_DIFF_MEM = new Map();
const ACTIVE_DIFF_TTL_MS = 15000;
const CONV_RE = /^[a-zA-Z0-9_-]{6,80}$/;
const SSH_HOST_RE = /^[a-zA-Z0-9._@:-]{1,128}$/;
const SHA_RE = /^[0-9a-f]{4,40}$/i;
const isValidConvId = (id) => CONV_RE.test(String(id || '').trim());

const NO_CACHE = { 'Cache-Control': 'no-store, no-cache, must-revalidate' };
const STATIC_FILES = ['index.html', 'styles.css', 'app.js'];

function isValidSshHost(raw) {
  const h = String(raw || '').trim();
  return Boolean(h && !h.startsWith('-') && SSH_HOST_RE.test(h));
}

function normalizeHttpEndpoint(raw) {
  const trimmed = String(raw || '').trim();
  if (!trimmed) return '';
  let u;
  try {
    u = new URL(trimmed);
  } catch {
    throw new Error('远端 HTTP 地址格式无效');
  }
  if ((u.protocol !== 'http:' && u.protocol !== 'https:') || !u.host) {
    throw new Error('远端地址仅支持 http:// 或 https://');
  }
  return trimmed.replace(/\/+$/, '');
}

function shellQuote(s) {
  return `'${String(s ?? '').replace(/'/g, "'\\''")}'`;
}

function cachePath(convId) {
  const clean = String(convId || '').trim();
  return isValidConvId(clean) ? join(CACHE_DIR, `${clean}.json`) : '';
}

function setBoundedMap(map, key, val, limit) {
  if (map.has(key)) map.delete(key);
  map.set(key, val);
  if (map.size > limit) {
    const oldest = map.keys().next().value;
    if (oldest !== undefined) map.delete(oldest);
  }
}

function diffDiskPath(key) {
  const h = createHash('sha1').update(String(key)).digest('hex');
  return join(DIFF_CACHE_DIR, `${h}.json`);
}

function readImmutableDiffCache(key) {
  if (IMMUTABLE_DIFF_MEM.has(key)) return IMMUTABLE_DIFF_MEM.get(key);
  const p = diffDiskPath(key);
  if (!existsSync(p)) return null;
  try {
    const parsed = JSON.parse(readFileSync(p, 'utf-8'));
    if (parsed && Array.isArray(parsed.files)) {
      setBoundedMap(IMMUTABLE_DIFF_MEM, key, parsed, 200);
      return parsed;
    }
  } catch {
    // ignore corrupt diff cache
  }
  return null;
}

function writeImmutableDiffCache(key, payload) {
  setBoundedMap(IMMUTABLE_DIFF_MEM, key, payload, 200);
  try {
    mkdirSync(DIFF_CACHE_DIR, { recursive: true });
    const p = diffDiskPath(key);
    const tmp = `${p}.tmp.${process.pid}.${Date.now()}`;
    writeFileSync(tmp, JSON.stringify(payload), 'utf-8');
    renameSync(tmp, p);
  } catch {
    // ignore disk cache write failure
  }
}

function readActiveDiffCache(key) {
  const hit = ACTIVE_DIFF_MEM.get(key);
  if (!hit) return null;
  if (Date.now() - hit.ts >= ACTIVE_DIFF_TTL_MS) {
    ACTIVE_DIFF_MEM.delete(key);
    return null;
  }
  return hit.payload;
}

function writeActiveDiffCache(key, payload) {
  setBoundedMap(ACTIVE_DIFF_MEM, key, { ts: Date.now(), payload }, 100);
}

function readCachedState(convId) {
  const p = cachePath(convId);
  if (!p || !existsSync(p)) return null;
  try {
    const parsed = JSON.parse(readFileSync(p, 'utf-8'));
    if (parsed && parsed.state) return parsed;
    if (parsed && parsed.nodes) return { cached_at: '', state: parsed };
    return null;
  } catch {
    return null;
  }
}

function writeCachedState(convId, state) {
  const p = cachePath(convId);
  if (!p || !state || typeof state !== 'object') return;
  try {
    mkdirSync(CACHE_DIR, { recursive: true });
    const tmp = `${p}.tmp.${process.pid}.${Date.now()}`;
    writeFileSync(tmp, JSON.stringify({
      cached_at: new Date().toISOString(),
      state,
    }, null, 2), 'utf-8');
    renameSync(tmp, p);
  } catch {
    // ignore cache write failure
  }
}

function removeCachedState(convId) {
  const cp = cachePath(convId);
  if (!cp) return;
  try {
    unlinkSync(cp);
  } catch (err) {
    if (err?.code !== 'ENOENT') {
      // ignore
    }
  }
}

async function fetchJsonWithTimeout(url, timeoutMs = 3500) {
  const ac = new AbortController();
  const timer = setTimeout(() => ac.abort(new Error(`Timeout after ${timeoutMs}ms`)), timeoutMs);
  try {
    const res = await fetch(url, { signal: ac.signal, headers: { Accept: 'application/json' } });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return await res.json();
  } finally {
    clearTimeout(timer);
  }
}

async function sshExec(sshHost, remoteCmd, timeoutMs = 4500) {
  if (!isValidSshHost(sshHost)) {
    throw new Error('无效的 SSH 主机名');
  }
  mkdirSync(SSH_SOCK_DIR, { recursive: true, mode: 0o700 });
  const args = [
    '-o', 'BatchMode=yes',
    '-o', 'ConnectTimeout=4',
    '-o', 'ControlMaster=auto',
    '-o', 'ControlPersist=60',
    '-o', `ControlPath=${join(SSH_SOCK_DIR, 'cm-%r@%h:%p')}`,
    sshHost,
    remoteCmd,
  ];
  const { stdout } = await execFileAsync('ssh', args, {
    encoding: 'utf-8',
    maxBuffer: 6 * 1024 * 1024,
    timeout: timeoutMs,
  });
  return stdout;
}

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

function resolveConvFromRequest(data) {
  if (isValidConvId(data?.conversationId)) return String(data.conversationId).trim();
  if (isValidConvId(process.env.ANTIGRAVITY_CONVERSATION_ID)) return String(process.env.ANTIGRAVITY_CONVERSATION_ID).trim();
  return '';
}

function resolveRemoteBinding(convId, repoParam) {
  if (!convId) return null;
  const binding = readBinding(convId);
  if (!binding || binding.mode !== 'remote' || !binding.remote?.repo) return null;
  const reqRepo = String(repoParam || '').trim();
  if (reqRepo && reqRepo !== String(binding.remote.repo).trim()) return null;
  return binding;
}

function readRepoOwnerConv(repoPath) {
  const metaPath = join(repoPath, '.git', 'exp-tree', 'binding.json');
  if (!existsSync(metaPath)) return '';
  try {
    const meta = JSON.parse(readFileSync(metaPath, 'utf-8'));
    const owner = String(meta.conversation_id || '').trim();
    if (!isValidConvId(owner)) return '';
    const ob = readBinding(owner);
    if (!ob || ob.mode === 'remote' || !ob.repo || resolve(ob.repo) !== resolve(repoPath)) return '';
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
  if (existing?.mode !== 'remote' && existing?.repo) {
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
  removeCachedState(convId);
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

function buildCompareTargets(state, toNode, toId) {
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
  return { pids, compareTargets };
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
  const convId = resolveConvFromRequest(data);
  let binding = convId ? readBinding(convId) : null;

  if (binding?.mode === 'remote' && binding?.remote?.repo) {
    const remoteRepo = binding.remote.repo;
    const cached = convId ? readCachedState(convId) : null;
    const st = cached?.state;
    const nodeCount = st?.nodes ? Object.keys(st.nodes).length : 0;
    return {
      conversationId: convId,
      standalone: false,
      bound: Boolean(remoteRepo),
      mode: 'remote',
      remote: binding.remote,
      repo: remoteRepo,
      repoInfo: {
        path: remoteRepo,
        label: basename(remoteRepo) || remoteRepo,
        hasTree: true,
        isWorkspace: false,
        isRemote: true,
        sshHost: binding.remote.ssh_host || '',
        transport: binding.remote.transport || '',
        endpoint: binding.remote.endpoint || '',
        nodeCount,
        goal: st?.goal || '',
        metricName: st?.metric_name || '',
      },
      focus: binding,
      repos: listRepos(data.workspaces || '', convId),
      buildId: getBuildId(),
    };
  }

  if (binding?.repo && !existsSync(binding.repo)) {
    deleteBinding(convId);
    binding = null;
  }
  const boundRepo = binding?.repo || '';
  return {
    conversationId: convId,
    standalone: false,
    bound: Boolean(boundRepo),
    mode: 'local',
    remote: null,
    repo: boundRepo,
    repoInfo: boundRepo ? summarizeRepo(boundRepo, false) : null,
    focus: binding || null,
    repos: listRepos(data.workspaces || '', convId),
    buildId: getBuildId(),
  };
}, 'GET');

app.api('/api/bind', (data) => {
  const convId = resolveConvFromRequest(data);
  if (!convId) {
    throw new Error('缺少有效的对话 ID (conversationId)');
  }
  const mode = String(data.mode || '').trim();
  const repoRaw = String(data.repo || '').trim();

  if (mode === 'remote') {
    if (!repoRaw) {
      throw new Error('请填写远端仓库路径');
    }
    const endpoint = normalizeHttpEndpoint(data.endpoint || '');
    const sshHost = String(data.sshHost || '').trim();
    if (!endpoint && !sshHost) {
      throw new Error('请填写远端 HTTP 隧道地址或 SSH 主机名');
    }
    if (sshHost && !isValidSshHost(sshHost)) {
      throw new Error('SSH 主机名格式无效');
    }
    const transport = endpoint ? 'http' : 'ssh';
    const remote = {
      transport,
      ...(endpoint ? { endpoint } : {}),
      ...(sshHost ? { ssh_host: sshHost } : {}),
      repo: repoRaw,
    };
    const now = new Date().toISOString();
    const prev = readBinding(convId);
    const sameTarget = Boolean(
      prev?.mode === 'remote' &&
      prev?.remote &&
      prev.remote.transport === remote.transport &&
      (prev.remote.endpoint || '') === endpoint &&
      (prev.remote.ssh_host || '') === sshHost &&
      (prev.remote.repo || '') === repoRaw
    );
    if (!sameTarget) {
      removeCachedState(convId);
    }
    const next = {
      conversation_id: convId,
      mode: 'remote',
      remote,
      repo: repoRaw,
      selected_node_id: sameTarget ? String(prev.selected_node_id || '') : '',
      selected_node_ids: sameTarget && Array.isArray(prev.selected_node_ids) ? prev.selected_node_ids : [],
      active_tab: sameTarget ? String(prev.active_tab || 'tree') : 'tree',
      bound_at: (sameTarget && prev.bound_at) ? prev.bound_at : now,
      updated_at: now,
    };
    writeBindingAtomic(convId, next);
    return {
      ok: true,
      bound: true,
      mode: 'remote',
      remote,
      conversationId: convId,
      repo: repoRaw,
    };
  }

  if (!repoRaw) {
    deleteBinding(convId);
    return { ok: true, bound: false, mode: 'local', remote: null, conversationId: convId };
  }
  const absRepo = resolve(repoRaw);
  if (!existsSync(absRepo)) {
    throw new Error('目标仓库路径不存在');
  }
  const now = new Date().toISOString();
  const prev = readBinding(convId);
  const sameRepo = Boolean(prev?.mode !== 'remote' && prev?.repo && resolve(prev.repo) === absRepo);
  removeCachedState(convId);
  const next = {
    conversation_id: convId,
    mode: 'local',
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
    mode: 'local',
    remote: null,
    conversationId: convId,
    repo: absRepo,
    repoInfo: summarizeRepo(absRepo, false),
  };
}, 'GET');

app.api('/api/state', async (data) => {
  const buildId = getBuildId();
  const convId = resolveConvFromRequest(data);
  const remoteBinding = resolveRemoteBinding(convId, data.repo);

  if (remoteBinding) {
    const remote = remoteBinding.remote;
    const remoteRepo = remote.repo;
    try {
      if (remote.transport === 'http' && remote.endpoint) {
        const hasCache = Boolean(readCachedState(convId)?.state?.nodes);
        const prefix = `${buildId}|remote|`;
        const rawReqEtag = String(data.etag || '');
        const passEtag = (hasCache && rawReqEtag.startsWith(prefix)) ? rawReqEtag.slice(prefix.length) : '';
        const res = await fetchJsonWithTimeout(`${remote.endpoint}/api/state?etag=${encodeURIComponent(passEtag)}`, 3500);
        if (res.unchanged) {
          return {
            unchanged: true,
            etag: `${prefix}${res.etag || passEtag}`,
            remote: true,
            offline: false,
            buildId,
          };
        }
        if (res.state?.nodes) {
          writeCachedState(convId, res.state);
        }
        const etag = `${prefix}${res.etag || ''}`;
        return {
          initialized: Boolean(res.initialized && res.state?.nodes),
          repo: remoteRepo,
          state: res.state || null,
          etag,
          remote: true,
          offline: false,
          buildId,
        };
      }

      const stateFile = `${remoteRepo}/.git/exp-tree/state.json`;
      const raw = await sshExec(
        remote.ssh_host,
        `if [ -f ${shellQuote(stateFile)} ]; then cat ${shellQuote(stateFile)}; else printf '__NO_STATE__'; fi`,
        4500
      );
      if (raw.trim() === '__NO_STATE__') {
        return {
          initialized: false,
          repo: remoteRepo,
          state: null,
          etag: `${buildId}|ssh|uninit`,
          remote: true,
          offline: false,
          buildId,
        };
      }
      const state = JSON.parse(raw);
      const nodeCount = Object.keys(state?.nodes || {}).length;
      if (state?.nodes && nodeCount > 0) {
        writeCachedState(convId, state);
      }
      const etag = `${buildId}|ssh|${raw.length}|${state.seq || 0}|${nodeCount}`;
      if (data.etag && data.etag === etag) {
        return { unchanged: true, etag, remote: true, offline: false, buildId };
      }
      return {
        initialized: Boolean(state?.root_id && nodeCount > 0),
        repo: remoteRepo,
        state,
        etag,
        remote: true,
        offline: false,
        buildId,
      };
    } catch (err) {
      const cached = readCachedState(convId);
      return {
        initialized: Boolean(cached?.state?.root_id && cached?.state?.nodes),
        repo: remoteRepo,
        state: cached?.state || null,
        cachedAt: cached?.cached_at || '',
        etag: `${buildId}|offline|${cached?.cached_at || 'none'}`,
        remote: true,
        offline: true,
        offlineReason: String(err?.message || err || 'Remote unreachable'),
        buildId,
      };
    }
  }

  const repo = data.repo || '';
  if (!repo || !existsSync(repo)) {
    return { initialized: false, state: null, remote: false, offline: false, buildId };
  }
  const statePath = join(repo, '.git', 'exp-tree', 'state.json');
  if (!existsSync(statePath)) {
    return { initialized: false, repo, state: null, remote: false, offline: false, buildId };
  }
  const st = statSync(statePath);
  const etag = `${buildId}|${repo}|${st.mtimeMs}|${st.size}`;
  if (data.etag && data.etag === etag) {
    return { unchanged: true, etag, remote: false, offline: false, buildId };
  }
  const state = JSON.parse(readFileSync(statePath, 'utf-8'));
  return { initialized: true, repo, state, etag, remote: false, offline: false, buildId };
}, 'GET');

app.api('/api/diff', async (data) => {
  const convId = resolveConvFromRequest(data);
  const remoteBinding = resolveRemoteBinding(convId, data.repo);
  const toId = data.to || '';
  const fromOverride = data.from || '';
  const forceRefresh = data.refresh === '1' || data.refresh === 'true';

  if (remoteBinding) {
    const remote = remoteBinding.remote;
    if (remote.transport === 'http' && remote.endpoint) {
      try {
        return await fetchJsonWithTimeout(
          `${remote.endpoint}/api/diff?to=${encodeURIComponent(toId)}&from=${encodeURIComponent(fromOverride)}`,
          4500
        );
      } catch (err) {
        return { files: [], rawDiff: '', offline: true, error: String(err?.message || err) };
      }
    }

    try {
      // Reuse cached state polled by /api/state to avoid a redundant SSH cat roundtrip
      let state = !forceRefresh ? readCachedState(convId)?.state : null;
      if (!state?.nodes?.[toId] || (fromOverride && !state.nodes?.[fromOverride])) {
        const stateFile = `${remote.repo}/.git/exp-tree/state.json`;
        const stateRaw = await sshExec(remote.ssh_host, `cat ${shellQuote(stateFile)}`, 4500);
        state = JSON.parse(stateRaw);
        if (state?.nodes && Object.keys(state.nodes).length > 0) {
          writeCachedState(convId, state);
        }
      }
      const toNode = state.nodes?.[toId];
      if (!toNode) return { files: [], rawDiff: '', error: 'Node not found' };

      const { pids, compareTargets } = buildCompareTargets(state, toNode, toId);
      const fromId = fromOverride || pids[0] || '';
      const parentNode = fromId ? state.nodes?.[fromId] : null;
      const isPruned = toNode.status === 'PRUNED';
      const toSHA = String(toNode.git_commit || '').trim();
      const qRepo = shellQuote(remote.repo);
      const toActive = toNode.status === 'EXPLORING' || toNode.status === 'SYNTHESIZING';
      const fromActive = Boolean(parentNode && (parentNode.status === 'EXPLORING' || parentNode.status === 'SYNTHESIZING'));

      if (!parentNode) {
        if (!toSHA || !SHA_RE.test(toSHA)) {
          return { files: [], rawDiff: '', fromId: '', fromLabel: 'empty', toLabel: toNode.id, compareTargets };
        }
        const rootKey = `ssh-root|${remote.ssh_host}|${remote.repo}|${toSHA}|${toNode.id}`;
        if (!toActive && !forceRefresh) {
          const hit = readImmutableDiffCache(rootKey);
          if (hit) return { ...hit, compareTargets };
        }
        const rawDiff = await sshExec(remote.ssh_host, `git -C ${qRepo} show --format= --unified=3 ${toSHA}`, 6000);
        const payload = {
          files: parseUnifiedDiff(rawDiff),
          rawDiff,
          fromId: '',
          fromLabel: 'Initial Snapshot',
          toLabel: `${toNode.id} (${toSHA.slice(0, 7)})`,
        };
        if (!toActive) writeImmutableDiffCache(rootKey, payload);
        return { ...payload, compareTargets };
      }

      const fromSHA = String(parentNode.git_commit || '').trim();
      if (fromSHA === toSHA && isPruned) {
        return {
          files: [],
          rawDiff: '',
          isPruned: true,
          prunedLegacyNoCommit: true,
          pruneDigest: toNode.prune_digest || '早期剪枝节点未保存代码快照；新剪枝节点均自动保留失败 Commit。',
          fromId: parentNode.id,
          fromLabel: `${parentNode.id} (${fromSHA.slice(0, 7)})`,
          toLabel: `${toNode.id} (旧版未存快照)`,
          compareTargets,
        };
      }

      if (!SHA_RE.test(fromSHA) || (toSHA && !SHA_RE.test(toSHA))) {
        return { files: [], rawDiff: '', isPruned, pruneDigest: toNode.prune_digest || '', error: 'Invalid git commit SHA', compareTargets };
      }

      if (!toActive && !fromActive) {
        const immKey = `ssh|${remote.ssh_host}|${remote.repo}|${fromSHA}|${toSHA}|${toNode.id}|${parentNode.id}|${isPruned ? 1 : 0}`;
        if (!forceRefresh) {
          const hit = readImmutableDiffCache(immKey);
          if (hit) return { ...hit, compareTargets };
        }
        const rawDiff = (toSHA && toSHA !== fromSHA)
          ? await sshExec(remote.ssh_host, `git -C ${qRepo} diff --unified=3 ${fromSHA} ${toSHA}`, 6000)
          : '';
        const payload = {
          files: parseUnifiedDiff(rawDiff),
          rawDiff,
          isPruned,
          pruneDigest: toNode.prune_digest || '',
          fromId: parentNode.id,
          fromLabel: `${parentNode.id} (${fromSHA.slice(0, 7)})`,
          toLabel: `${toNode.id} (${toSHA.slice(0, 7)})`,
        };
        writeImmutableDiffCache(immKey, payload);
        return { ...payload, compareTargets };
      }

      const activeKey = `ssh-active|${remote.ssh_host}|${remote.repo}|${parentNode.id}:${fromSHA}|${toNode.id}:${toSHA}`;
      if (!forceRefresh) {
        const hit = readActiveDiffCache(activeKey);
        if (hit) return { ...hit, compareTargets };
      }

      const wtPath = String(toNode.worktree_path || '').trim();
      const fromWt = String(parentNode.worktree_path || '').trim();
      const fromResolve = (fromActive && fromWt)
        ? `FROM_REF=$(if [ -d ${shellQuote(fromWt)} ]; then git -C ${shellQuote(fromWt)} rev-parse HEAD 2>/dev/null || echo ${fromSHA}; else echo ${fromSHA}; fi)`
        : `FROM_REF=${fromSHA}`;

      let diffCmd;
      let toSuffix = (toSHA || 'working').slice(0, 7);
      if (toActive && wtPath) {
        const qWt = shellQuote(wtPath);
        const fallbackCmd = (toSHA && toSHA !== fromSHA)
          ? `git -C ${qRepo} diff --unified=3 "$FROM_REF" ${toSHA}`
          : 'true';
        diffCmd = `${fromResolve}; if [ -d ${qWt} ]; then git -C ${qWt} diff --unified=3 "$FROM_REF"; else ${fallbackCmd}; fi`;
        toSuffix = 'working';
      } else if (toActive) {
        diffCmd = `${fromResolve}; git -C ${qRepo} diff --unified=3 "$FROM_REF"`;
        toSuffix = 'working';
      } else {
        diffCmd = `${fromResolve}; if [ -n ${shellQuote(toSHA)} ] && [ "$FROM_REF" != ${shellQuote(toSHA)} ]; then git -C ${qRepo} diff --unified=3 "$FROM_REF" ${toSHA}; fi`;
      }

      const rawDiff = await sshExec(remote.ssh_host, diffCmd, 6000);
      const payload = {
        files: parseUnifiedDiff(rawDiff),
        rawDiff,
        isPruned,
        pruneDigest: toNode.prune_digest || '',
        fromId: parentNode.id,
        fromLabel: `${parentNode.id} (${fromActive ? 'working' : fromSHA.slice(0, 7)})`,
        toLabel: `${toNode.id} (${toSuffix})`,
      };
      writeActiveDiffCache(activeKey, payload);
      return { ...payload, compareTargets };
    } catch (err) {
      return { files: [], rawDiff: '', offline: true, error: String(err?.message || err) };
    }
  }

  const repo = data.repo || '';
  const statePath = join(repo, '.git', 'exp-tree', 'state.json');
  if (!existsSync(statePath)) return { files: [], rawDiff: '', fromLabel: '', toLabel: '' };

  const state = JSON.parse(readFileSync(statePath, 'utf-8'));
  const toNode = state.nodes?.[toId];
  if (!toNode) return { files: [], rawDiff: '', error: 'Node not found' };

  const { pids, compareTargets } = buildCompareTargets(state, toNode, toId);
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

  const isActive = toNode.status === 'EXPLORING' || toNode.status === 'SYNTHESIZING';
  const fromActive = parentNode.status === 'EXPLORING' || parentNode.status === 'SYNTHESIZING';
  const hasWorktree = Boolean(toNode.worktree_path && existsSync(toNode.worktree_path));
  let fromRef = parentNode.git_commit || '';
  if (fromActive && parentNode.worktree_path && existsSync(parentNode.worktree_path)) {
    try {
      const head = execFileSync('git', ['-C', parentNode.worktree_path, 'rev-parse', 'HEAD'], { encoding: 'utf-8' }).trim();
      if (head) fromRef = head;
    } catch { /* fallback to git_commit */ }
  }

  if (!isActive && !fromActive && !forceRefresh) {
    const localKey = `local|${repo}|${fromRef}|${toNode.git_commit || ''}|${toNode.id}|${parentNode.id}|${isPruned ? 1 : 0}`;
    const hit = readImmutableDiffCache(localKey);
    if (hit) return { ...hit, compareTargets };
  }

  try {
    let rawDiff = '';
    let toSuffix = (toNode.git_commit || 'working').slice(0, 7);
    if (isActive && hasWorktree) {
      rawDiff = execFileSync('git', ['-C', toNode.worktree_path, 'diff', '--unified=3', fromRef], { encoding: 'utf-8', maxBuffer: 4 * 1024 * 1024 });
      toSuffix = 'working';
    } else if (isActive) {
      rawDiff = execFileSync('git', ['-C', repo, 'diff', '--unified=3', fromRef], { encoding: 'utf-8', maxBuffer: 4 * 1024 * 1024 });
      toSuffix = 'working';
    } else if (toNode.git_commit && toNode.git_commit !== fromRef) {
      rawDiff = execFileSync('git', ['-C', repo, 'diff', '--unified=3', fromRef, toNode.git_commit], { encoding: 'utf-8', maxBuffer: 4 * 1024 * 1024 });
    }
    const payload = {
      files: parseUnifiedDiff(rawDiff),
      rawDiff,
      isPruned,
      pruneDigest: toNode.prune_digest || '',
      fromId: parentNode.id,
      fromLabel: `${parentNode.id} (${fromActive ? 'working' : (parentNode.git_commit || '').slice(0, 7)})`,
      toLabel: `${toNode.id} (${toSuffix})`,
    };
    if (!isActive && !fromActive) {
      const localKey = `local|${repo}|${fromRef}|${toNode.git_commit || ''}|${toNode.id}|${parentNode.id}|${isPruned ? 1 : 0}`;
      writeImmutableDiffCache(localKey, payload);
    }
    return { ...payload, compareTargets };
  } catch (err) {
    return { files: [], rawDiff: '', isPruned, pruneDigest: toNode.prune_digest || '', error: String(err.message || err), compareTargets };
  }
}, 'GET');

// Reliable chat message sender that resolves conversationId even after sidecar:// navigation!
app.api('/api/chat', (data) => {
  const convId = resolveActiveConversationId(data.conversationId || '');
  if (!convId) throw new Error('未能定位当前活跃对话 ID');
  const message = String(data.message || '').trim();
  if (!message) throw new Error('消息内容为空');
  return app.sendMessage(convId, message, { projectId: process.env.ANTIGRAVITY_PROJECT_ID });
}, 'GET');

app.api('/api/focus', async (data) => {
  const convId = isValidConvId(data.conversationId) ? String(data.conversationId).trim() : '';
  const repo = String(data.repo || '').trim();
  const selectedNodeId = String(data.node || '');
  const multi = String(data.nodes || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  const tab = String(data.tab || 'tree');
  const now = new Date().toISOString();

  if (convId) {
    const existing = readBinding(convId);
    const isRemote = existing?.mode === 'remote';
    const matchesRepo = isRemote
      ? (!repo || String(existing?.remote?.repo || existing?.repo || '') === repo)
      : (!repo || resolve(existing?.repo || '') === resolve(repo));

    if (existing && matchesRepo) {
      writeBindingAtomic(convId, {
        conversation_id: convId,
        mode: existing.mode || 'local',
        ...(existing.remote ? { remote: existing.remote } : {}),
        repo: existing.repo || repo,
        selected_node_id: selectedNodeId,
        selected_node_ids: multi,
        active_tab: tab || existing.active_tab || 'tree',
        bound_at: existing.bound_at || now,
        updated_at: now,
      });
    }

    if (isRemote) {
      if (existing.remote?.transport === 'http' && existing.remote.endpoint) {
        fetchJsonWithTimeout(
          `${existing.remote.endpoint}/api/focus?node=${encodeURIComponent(selectedNodeId)}&nodes=${encodeURIComponent(multi.join(','))}&tab=${encodeURIComponent(tab)}`,
          1500
        ).catch(() => {});
      }
      return { ok: true, conversationId: convId, mode: 'remote' };
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
    return { ok: true, conversationId: convId, mode: 'local' };
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
