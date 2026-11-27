(() => {
  const $ = (id) => document.getElementById(id);
  // Rails use a quiet neutral palette; saturated hues are reserved for node status only.
  const LANE_COLORS = ['var(--lane-0)', 'var(--lane-1)', 'var(--lane-2)', 'var(--lane-3)', 'var(--lane-4)'];
  const ICON_FILE = '<svg class="ico" viewBox="0 0 16 16" aria-hidden="true"><path d="M4 1.75h5l3 3v9.5H4z"/><path d="M9 1.75v3h3"/></svg>';
  const ICON_CHECK = '<svg class="ico" viewBox="0 0 16 16" aria-hidden="true"><path d="M3.5 8.5l3 3 6-7"/></svg>';
  const ICON_SERVER = '<svg class="ico" viewBox="0 0 16 16" aria-hidden="true"><rect x="2.2" y="2.8" width="11.6" height="4.2" rx="1"/><rect x="2.2" y="9" width="11.6" height="4.2" rx="1"/><circle cx="4.5" cy="4.9" r="0.6" fill="currentColor" stroke="none"/><circle cx="4.5" cy="11.1" r="0.6" fill="currentColor" stroke="none"/></svg>';
  const urlParams = new URLSearchParams(location.search);
  const currentConv = () => window.sidecar?.conversationId || urlParams.get('conversationId') || '';

  const LANG = (() => {
    const q = (urlParams.get('lang') || '').toLowerCase();
    if (q === 'en' || q === 'zh') return q;
    return (navigator.language || '').toLowerCase().startsWith('zh') ? 'zh' : 'en';
  })();
  document.documentElement.lang = LANG === 'en' ? 'en' : 'zh-CN';

  const I18N = {
    en: {
      follow: 'Follow',
      followTitle: 'Auto-follow the latest or active node in this tree',
      refreshTitle: 'Refresh now',
      menuTitle: 'Switch or manage experiment trees',
      tabTree: 'Tree',
      tabChanges: 'Changes',
      tabOverview: 'Overview',
      modeRail: 'Rail',
      modeMap: 'DAG Map',
      filterAll: 'All nodes',
      filterWin: 'Promoted / Leading only',
      filterPruned: 'Pruned only',
      zoomOut: 'Zoom out',
      zoomReset: 'Reset zoom',
      zoomIn: 'Zoom in',
      focusClear: 'Clear selection',
      focusHintSingle: 'Say "this node..." in chat',
      focusHintMulti: 'Say "converge selected branches"',
      focusSingle: (id) => `Focused ${id}`,
      focusMulti: (n, ids) => `Selected ${n} · ${ids}`,
      emptyTitle: 'No experiment tree bound yet',
      emptySub: 'Describe your research or optimization goal in chat to initialize a tree automatically.',
      emptyDraft: 'Send a prompt in chat to initialize or bind a repository',
      emptyOrBind: 'or bind an existing repository',
      bindBtn: 'Bind',
      watchIdle: 'Waiting for chat prompt…',
      watchActive: 'Listening to this conversation…',
      menuRebind: 'Switch repository',
      menuUnbind: 'Unbind conversation',
      menuHint: 'Start a new conversation for a new topic',
      noLocalRepos: 'No local experiment repositories found',
      tagCurrent: 'Current',
      tagOtherConv: 'Other chat',
      tagNoTree: 'Empty',
      tagNodes: (n) => `${n} nodes`,
      previewBind: 'Bind to chat',
      previewBack: 'Back',
      previewBackBound: 'Back to bound tree',
      previewLabel: (name) => `Previewing ${name} (unbound)`,
      retry: 'Retry',
      offlineBanner: 'Remote disconnected · offline read-only cache',
      offlineBannerAt: (t) => `Remote disconnected · offline cache (${t})`,
      offlineUninitTitle: 'Cannot reach remote experiment tree',
      offlineUninitDesc: (ep) => `Unable to reach ${ep} and no local cache exists yet. Verify SSH connectivity and click Retry.`,
      remoteUninitTitle: 'Remote repository has no tree yet',
      remoteUninitDesc: (r) => `Bound to remote ${r}; run exptree init or exptree up there to sync automatically.`,
      localUninitTitle: 'No experiment tree in bound repository',
      localUninitDesc: 'Describe your optimization goal in chat to initialize a tree automatically, or switch repos from the top-right menu.',
      convPrefix: (c) => (c ? `Chat ${c.slice(0, 8)}…` : 'Standalone'),
      noBoundRepo: 'No repository bound',
      badgeMergedLead: 'Merged · Lead',
      badgeMerged: 'Merged',
      badgeLead: 'Leading',
      badgeBest: 'Best',
      badgeBase: 'Baseline',
      badgePass: 'Validated',
      badgeViolate: 'Constraint failed',
      badgePruned: 'Pruned',
      badgePrep: 'Preparing',
      badgeQueue: 'Queued',
      badgeRun: 'Evaluating',
      badgeSynth: 'Converging',
      axisY: 'Y',
      axisX: 'X',
      reqOp: (op, th) => `req ${op} ${th}`,
      revisitTag: (id) => `↻ Revisiting ${id}`,
      rootBase: 'Root baseline',
      mergedPrefix: (ps) => `Converge ${ps}`,
      jumpDiffTitle: (p) => `View changes in ${p}`,
      multiBtnTitle: 'Select multiple nodes to converge (or Cmd/Shift-click row)',
      goalMulti: (specs, cCnt) => `Optimize ${specs}${cCnt ? ` · ${cCnt} hard ${cCnt === 1 ? 'gate' : 'gates'}` : ''}`,
      goalSingle: (m, dir) => `Optimize ${m} ${dir}`,
      paretoTitle: 'Multi-Objective Trade-off',
      paretoNote: 'Top-right is better',
      paretoPlot: 'Plot',
      paretoTable: 'Table',
      tblNode: 'Node',
      beatsBase: 'Beats baseline',
      higherBetter: 'higher is better',
      lowerBetter: 'lower is better',
      sparkTitle: 'Best Score Trajectory',
      sparkNote: 'Higher is better',
      outOfRange: ' (clamped to edge)',
      diffSelectNode: 'Select node',
      diffVs: 'vs',
      diffRoleBase: 'Baseline',
      diffRoleParent: 'Parent',
      diffLegacyPruned: 'Legacy pruned node has no saved snapshot',
      diffOfflineTitle: 'Remote disconnected',
      diffOfflineDesc: 'Offline read-only cache active. Restore SSH connectivity and click Retry to load diffs.',
      diffPrunedSaved: 'Preserved failure snapshot',
      diffPrunedDefault: 'Below baseline',
      diffNoFiles: 'No file differences against selected baseline',
      diffLoading: 'Loading diff…',
      diffChangesOnly: 'Changes only',
      diffCollapseAll: 'Collapse all',
      diffExpandAll: 'Expand all',
      diffRefreshTitle: 'Refresh diff',
      diffMoreBase: 'Other node…',
      diffFilesSummary: (n) => `${n} ${n === 1 ? 'file' : 'files'} changed`,
      diffNoChangesHunk: 'No added/removed lines',
      ovProgress: 'Progress',
      ovLead: 'Leading',
      ovCons: 'Gates',
      ovBest: 'Best',
      ovVsBase: (d) => `${d} vs baseline`,
      ovConf: (r) => `${r.toFixed(1)}× MAD`,
      ovConfTitle: 'Signal-to-noise ratio: improvement / Median Absolute Deviation',
      ovFeed: 'Activity',
      ovNoFeed: 'No activity yet',
      ovExp: (n) => `${n} ${n === 1 ? 'run' : 'runs'}`,
      ovSucc: (n) => `${n} passed`,
      ovPrune: (n, v) => `${n} pruned${v ? ` (${v} violated)` : ''}`,
      ovConv: (n) => `${n} merged`,
      ovRun: (n) => `${n} running`,
      toastFollowOn: 'Auto-follow enabled',
      toastFollowOff: 'Auto-follow paused',
      toastSynced: 'Synced latest state',
      toastStillOffline: 'Remote still unreachable',
      toastReconnected: 'Remote reconnected',
      toastNoConv: 'No conversation ID detected',
      toastBound: 'Bound to this conversation',
      toastUnbound: 'Unbound from conversation',
    },
    zh: {
      follow: '跟随',
      followTitle: '自动跟随本树最新运行或新增的节点',
      refreshTitle: '立即刷新',
      menuTitle: '切换或新建探索树',
      tabTree: '探索树',
      tabChanges: '代码变更',
      tabOverview: '总览',
      modeRail: '列表轨道',
      modeMap: '树形画布',
      filterAll: '全部节点',
      filterWin: '仅看已晋升 / 领先',
      filterPruned: '仅看已剪枝',
      zoomOut: '缩小',
      zoomReset: '重置缩放',
      zoomIn: '放大',
      focusClear: '清除多选',
      focusHintSingle: '在对话里说“这个节点…”即可指代',
      focusHintMulti: '在对话里说“融合选中的分支”',
      focusSingle: (id) => `聚焦 ${id}`,
      focusMulti: (n, ids) => `已选 ${n} 个 · ${ids}`,
      emptyTitle: '当前对话尚未绑定探索树',
      emptySub: '直接在左侧对话框描述科研或优化主题，主 Agent 会立即建树并自动接入此面板。',
      emptyDraft: '在左侧发送首条消息后，可自动建树或关联已有实验树',
      emptyOrBind: '或关联已有实验树',
      bindBtn: '绑定',
      watchIdle: '等待左侧发起对话…',
      watchActive: '监听当前对话中 · 左侧发起探索后将自动接入',
      menuRebind: '关联其他已有实验树',
      menuUnbind: '解除当前对话绑定',
      menuHint: '开启新课题请直接新建对话',
      noLocalRepos: '未检测到本地实验仓库',
      tagCurrent: '当前',
      tagOtherConv: '其他对话',
      tagNoTree: '未建树',
      tagNodes: (n) => `${n} 节点`,
      previewBind: '绑定到此对话',
      previewBack: '返回',
      previewBackBound: '返回已绑定树',
      previewLabel: (name) => `预览 ${name}（未绑定当前对话）`,
      retry: '重试',
      offlineBanner: '远端连接已断开 · 当前展示本地离线缓存（只读）',
      offlineBannerAt: (t) => `远端连接已断开 · 离线只读缓存 (${t})`,
      offlineUninitTitle: '无法连接远端实验树',
      offlineUninitDesc: (ep) => `未能连接到 ${ep}，且本地尚无离线缓存。请确认 SSH 隧道或免密 SSH 已就绪后点击「重试」。`,
      remoteUninitTitle: '远端仓库尚未初始化探索树',
      remoteUninitDesc: (r) => `已在远端关联 ${r}，但尚未检测到探索树状态；在远端运行 exptree init 或 exptree up 后将自动同步。`,
      localUninitTitle: '当前绑定的仓库尚无探索树',
      localUninitDesc: '直接在左侧对话框说明科研或优化目标，主 Agent 会自动初始化探索树并实时接入此面板；也可从右上角「···」解绑或更换仓库。',
      convPrefix: (c) => (c ? `对话 ${c.slice(0, 8)}…` : '独立模式'),
      noBoundRepo: '当前未绑定仓库',
      badgeMergedLead: '融合 · 领先',
      badgeMerged: '已融合',
      badgeLead: '领先',
      badgeBest: '最优',
      badgeBase: '基线',
      badgePass: '已晋升',
      badgeViolate: '违反约束',
      badgePruned: '已剪枝',
      badgePrep: '准备中',
      badgeQueue: '排队中',
      badgeRun: '评测中',
      badgeSynth: '融合中',
      axisY: '纵轴',
      axisX: '横轴',
      reqOp: (op, th) => `要求 ${op} ${th}`,
      revisitTag: (id) => `↻ 重审 ${id}`,
      rootBase: '根基线',
      mergedPrefix: (ps) => `融合 ${ps}`,
      jumpDiffTitle: (p) => `查看 ${p} 的改动`,
      multiBtnTitle: '多选节点用于对话融合 (或按住 Cmd/Shift 点击行)',
      goalMulti: (specs, cCnt) => `优化 ${specs}${cCnt ? ` · ${cCnt} 项硬约束` : ''}`,
      goalSingle: (m, dir) => `优化 ${m} ${dir}`,
      paretoTitle: '多目标权衡',
      paretoNote: '越靠右上越优',
      paretoPlot: '散点图',
      paretoTable: '表格',
      tblNode: '节点',
      beatsBase: '优于基线',
      higherBetter: '越大越好',
      lowerBetter: '越小越好',
      sparkTitle: '最优值演进',
      sparkNote: '越靠上越优',
      outOfRange: '（超出坐标范围，已贴边）',
      diffSelectNode: '选择节点',
      diffVs: '对比',
      diffRoleBase: '基线',
      diffRoleParent: '父节点',
      diffLegacyPruned: '早期剪枝节点未存独立快照',
      diffOfflineTitle: '远端连接已断开',
      diffOfflineDesc: '当前为离线只读缓存模式，恢复远端连接后点击顶部「重试」即可加载代码变更。',
      diffPrunedSaved: '失败快照已保留',
      diffPrunedDefault: '未达标',
      diffNoFiles: '该节点相对基准无文件差异',
      diffLoading: '正在加载代码变更…',
      diffChangesOnly: '只看改动行',
      diffCollapseAll: '全部折叠',
      diffExpandAll: '全部展开',
      diffRefreshTitle: '重新计算差异',
      diffMoreBase: '其他节点…',
      diffFilesSummary: (n) => `共改动 ${n} 个文件`,
      diffNoChangesHunk: '无增删改动行',
      ovProgress: '进展',
      ovLead: '领先',
      ovCons: '约束',
      ovBest: '最优',
      ovVsBase: (d) => `${d} vs 基线`,
      ovConf: (r) => `置信度 ${r.toFixed(1)}× MAD`,
      ovConfTitle: '信噪比：相对基线提升幅度 / 中位数绝对偏差 (MAD)',
      ovFeed: '动态',
      ovNoFeed: '暂无动态',
      ovExp: (n) => `${n} 个实验`,
      ovSucc: (n) => `${n} 成功`,
      ovPrune: (n, v) => `${n} 剪枝${v ? `（${v} 违反约束）` : ''}`,
      ovConv: (n) => `${n} 融合`,
      ovRun: (n) => `${n} 运行中`,
      toastFollowOn: '已开启自动跟随本树最新节点',
      toastFollowOff: '已暂停自动跟随',
      toastSynced: '已同步最新状态',
      toastStillOffline: '远端仍未就绪',
      toastReconnected: '已恢复远端连接',
      toastNoConv: '未检测到当前对话 ID',
      toastBound: '已绑定到此对话',
      toastUnbound: '已解除当前对话绑定',
    },
  };

  function t(key, ...args) {
    const val = (I18N[LANG] && I18N[LANG][key] !== undefined) ? I18N[LANG][key] : I18N.en[key];
    return typeof val === 'function' ? val(...args) : val;
  }

  function applyStaticI18n() {
    const setText = (sel, val) => { const el = document.querySelector(sel); if (el) el.textContent = val; };
    const setLeadText = (sel, val) => {
      const el = document.querySelector(sel);
      if (el && el.firstChild && el.firstChild.nodeType === 3) el.firstChild.textContent = `${val} `;
    };
    setText('#btn-follow span:last-child', t('follow'));
    if ($('btn-follow')) $('btn-follow').title = t('followTitle');
    if ($('btn-refresh')) { $('btn-refresh').title = t('refreshTitle'); $('btn-refresh').setAttribute('aria-label', t('refreshTitle')); }
    if ($('btn-tree-menu')) $('btn-tree-menu').title = t('menuTitle');
    setLeadText('.tab[data-tab="tree"]', t('tabTree'));
    setLeadText('.tab[data-tab="changes"]', t('tabChanges'));
    setLeadText('.tab[data-tab="overview"]', t('tabOverview'));
    setText('#mode-rail', t('modeRail'));
    setText('#mode-map', t('modeMap'));
    setText('#filter-select option[value="all"]', t('filterAll'));
    setText('#filter-select option[value="win"]', t('filterWin'));
    setText('#filter-select option[value="pruned"]', t('filterPruned'));
    if ($('zoom-out')) $('zoom-out').title = t('zoomOut');
    if ($('zoom-reset')) $('zoom-reset').title = t('zoomReset');
    if ($('zoom-in')) $('zoom-in').title = t('zoomIn');
    setText('#focus-clear', t('focusClear'));
    setText('.empty-hero-title', t('emptyTitle'));
    setText('.empty-hero-sub', t('emptySub'));
    setText('#empty-draft-note', t('emptyDraft'));
    setText('.bind-divider span', t('emptyOrBind'));
    setText('#btn-bind-existing', t('bindBtn'));
    setText('#menu-toggle-rebind span:nth-child(2)', t('menuRebind'));
    setText('#menu-unbind span:nth-child(2)', t('menuUnbind'));
    setText('.menu-hint', t('menuHint'));
    setText('#btn-preview-bind', t('previewBind'));
    setText('#btn-offline-retry', t('retry'));
  }

  const S = {
    buildId: '',
    workspaces: [],
    repos: [],
    conv: '',
    bound: false,
    boundRepo: '',
    mode: 'local', // 'local' | 'remote'
    remote: null,
    offline: false,
    offlineReason: '',
    cachedAt: '',
    targetKey: '',
    standalone: false,
    previewRepo: urlParams.get('repo') || '',
    repo: '',
    etag: '',
    initialized: false,
    tree: null,
    tab: ['tree', 'changes', 'overview'].includes(urlParams.get('tab')) ? urlParams.get('tab') : 'tree',
    treeMode: urlParams.get('mode') === 'map' ? 'map' : 'rail', // 'rail' | 'map'
    filter: 'all',    // 'all' | 'win' | 'pruned'
    follow: true,
    knownNodeCount: 0,
    selectedId: urlParams.get('node') || '',
    expandedId: urlParams.get('node') || '',
    deepLinkedNode: urlParams.get('node') || '',
    multiSet: new Set(),
    diffFromId: '',
    diffCache: new Map(), // key -> diff response
    diffInFlight: new Map(), // key -> promise
    diffLoadingKey: '',
    diffChangesOnly: false,
    diffAutoCollapsedKey: '',
    diffSummaryOpen: true,
    pendingScrollFileIdx: null,
    collapsedFiles: new Set(),
    mapZoom: 1,
    mapFitPending: true,
    paretoView: 'plot', // 'plot' | 'table'
    paretoYIdx: 0,
    paretoXIdx: 1,
    paretoSortCol: '',
    paretoSortDir: 'best',
    menuOpen: false,
    rebindListOpen: false,
    lastReposSig: '',
  };

  // ---------- Host Theme Sync ----------
  function syncScheme() {
    const forced = urlParams.get('theme');
    if (forced === 'dark' || forced === 'light') {
      if (document.documentElement.dataset.theme !== forced) {
        document.documentElement.dataset.theme = forced;
      }
      return;
    }
    const probe = document.createElement('span');
    probe.style.cssText = 'position:absolute;visibility:hidden;color:var(--background, transparent)';
    document.documentElement.appendChild(probe);
    const c = getComputedStyle(probe).color;
    probe.remove();
    const nums = (c.match(/[\d.]+/g) || []).map(Number);
    let theme = window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    if (nums.length >= 3 && !(nums.length >= 4 && nums[3] === 0)) {
      const scale = c.startsWith('color(') ? 1 : 255;
      const [r, g, b] = nums.slice(0, 3).map((v) => v / scale);
      const lum = 0.2126 * r + 0.7152 * g + 0.0722 * b;
      theme = lum < 0.5 ? 'dark' : 'light';
    }
    if (document.documentElement.dataset.theme !== theme) {
      document.documentElement.dataset.theme = theme;
    }
  }
  syncScheme();
  new MutationObserver(syncScheme).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['style'],
  });

  // ---------- Helpers ----------
  const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
  ));

  let toastTimer = null;
  function showToast(msg) {
    const el = $('toast');
    if (!el) return;
    el.textContent = msg;
    el.classList.remove('hidden');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.classList.add('hidden'), 2200);
  }

  async function apiGet(path) {
    const fn = window.sidecar?.fetch ? window.sidecar.fetch.bind(window.sidecar) : fetch;
    const res = await fn(path, { method: 'GET' });
    if (!res.ok) throw new Error(`${res.status}`);
    return res.json();
  }

  function checkBuildReload(nextBuildId) {
    if (!nextBuildId) return false;
    if (!S.buildId) {
      S.buildId = nextBuildId;
      return false;
    }
    if (S.buildId !== nextBuildId) {
      location.reload();
      return true;
    }
    return false;
  }

  function fmtNum(v) {
    if (typeof v !== 'number' || !Number.isFinite(v)) return '—';
    if (v === 0) return '0';
    const abs = Math.abs(v);
    if (abs < 0.005 || abs >= 1e6) return v.toExponential(2);
    if (Number.isInteger(v)) return String(v);
    if (abs >= 1000) return String(Math.round(v));
    if (abs >= 100) return v.toFixed(1).replace(/\.0$/, '');
    if (abs >= 1) return v.toFixed(2).replace(/\.?0+$/, '');
    return v.toPrecision(3);
  }

  function fmtTime(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  }

  function repoShortName() {
    if (!S.repo) return '';
    const found = S.repos.find((r) => r.path === S.repo);
    return found ? found.label : S.repo.split('/').pop();
  }

  function parentsOf(n) {
    if (!n) return [];
    if (Array.isArray(n.parent_ids) && n.parent_ids.length) return n.parent_ids;
    return n.parent_id ? [n.parent_id] : [];
  }

  function calcDelta(score, refScore, higherIsBetter) {
    if (typeof score !== 'number' || typeof refScore !== 'number' || refScore === 0) return null;
    const rawPct = ((score - refScore) / Math.abs(refScore)) * 100;
    const improved = higherIsBetter ? score > refScore : score < refScore;
    const sign = rawPct > 0 ? '+' : '';
    return { text: `${sign}${rawPct.toFixed(1)}%`, improved, same: Math.abs(rawPct) < 0.05 };
  }

  const opSym = (op) => ({ '<=': '≤', '>=': '≥', '<': '<', '>': '>' }[op] || op);

  // Red is reserved for genuine hard-constraint violations; ordinary pruning is a normal research outcome.
  function isViolation(n) {
    return Boolean(n) && n.status === 'PRUNED' && /HARD_CONSTRAINT_VIOLATED|CONSTRAINT_GATE_FAILED/.test(n.prune_digest || '');
  }

  // Distinguish preparing (coding/compiling/smoke), queued (waiting eval.lock), and evaluating (running benchmark).
  function activePhase(n) {
    if (!n || n.status !== 'EXPLORING') return '';
    if (n.phase === 'evaluating' || n.phase === 'queued' || n.phase === 'preparing') return n.phase;
    const steps = Array.isArray(n.steps) ? n.steps : [];
    if (!steps.length) return 'preparing';
    const live = [...steps].reverse().find((s) => /^(进度[:：]|PROGRESS)/i.test(s.action || ''));
    const latest = live || steps[steps.length - 1];
    const latestMsg = String(latest?.action || '');
    if (/排队|queued/i.test(latestMsg) && !/已获得评测槽位|running eval/i.test(latestMsg)) {
      return 'queued';
    }
    const allActions = steps.map((s) => String(s.action || ''));
    if (allActions.some((a) => /已提交后台评测作业|已获得评测槽位|正在运行评测|Smoke sanity gate passed/i.test(a))
      || /\d+\s*\/\s*\d+|solved=|benchmark|running eval|已获得评测槽位/i.test(latestMsg)) {
      return 'evaluating';
    }
    return 'preparing';
  }

  function statusBadge(n, bestId) {
    const pFront = S.tree?.pareto_front_ids || [];
    const isMulti = (S.tree?.objectives || []).length >= 2;
    const onFront = isMulti && pFront.includes(n.id);
    switch (n.status) {
      case 'CONVERGED': return `<span class="badge conv">${esc(onFront ? t('badgeMergedLead') : t('badgeMerged'))}</span>`;
      case 'VALIDATED':
        if (onFront) return `<span class="badge pass">${esc(t('badgeLead'))}</span>`;
        if (!isMulti && n.id === bestId) return `<span class="badge pass">${esc(t('badgeBest'))}</span>`;
        if (n.id === S.tree?.root_id) return `<span class="badge muted">${esc(t('badgeBase'))}</span>`;
        return `<span class="badge pass-soft">${esc(t('badgePass'))}</span>`;
      case 'PRUNED': return isViolation(n) ? `<span class="badge prune">${esc(t('badgeViolate'))}</span>` : `<span class="badge muted">${esc(t('badgePruned'))}</span>`;
      case 'EXPLORING': {
        const ph = activePhase(n);
        if (ph === 'queued') return `<span class="badge queue">${esc(t('badgeQueue'))}</span>`;
        if (ph === 'preparing') return `<span class="badge prep">${esc(t('badgePrep'))}</span>`;
        return `<span class="badge run live">${esc(t('badgeRun'))}</span>`;
      }
      case 'SYNTHESIZING': return `<span class="badge conv live">${esc(t('badgeSynth'))}</span>`;
      default: return `<span class="badge muted">${esc(n.status)}</span>`;
    }
  }

  function constraintBreaches(n) {
    const cons = S.tree?.constraints || [];
    if (!n.metrics) return [];
    return cons.filter((c) => {
      const v = n.metrics[c.metric];
      if (typeof v !== 'number') return false;
      return (c.op === '<=' && !(v <= c.threshold)) || (c.op === '<' && !(v < c.threshold))
        || (c.op === '>=' && !(v >= c.threshold)) || (c.op === '>' && !(v > c.threshold));
    }).map((c) => ({ ...c, value: n.metrics[c.metric] }));
  }

  // One quiet line of objective values; constraints only surface when breached.
  function renderMultiMetricChips(n) {
    const objs = S.tree?.objectives || [];
    if (!n.metrics) return '';
    const parts = objs.length >= 2
      ? objs.filter((o) => typeof n.metrics[o.name] === 'number')
        .map((o) => `<span class="m-item"><span class="m-key">${esc(o.name)}</span><span class="m-val">${esc(fmtNum(n.metrics[o.name]))}</span></span>`)
      : [];
    const breaches = constraintBreaches(n)
      .map((c) => `<span class="metric-chip violated">${esc(c.metric)} ${esc(fmtNum(c.value))} · ${esc(t('reqOp', opSym(c.op), fmtNum(c.threshold)))}</span>`);
    if (!parts.length && !breaches.length) return '';
    return `${parts.length ? `<div class="metric-line">${parts.join('')}</div>` : ''}${breaches.length ? `<div class="metric-strip">${breaches.join('')}</div>` : ''}`;
  }

  function prettyKV(str) {
    return String(str || '').split(/,\s*/).map((kv) => {
      const m = /^\s*([A-Za-z_][\w.]*)\s*=\s*([-+0-9.eE]+)\s*$/.exec(kv);
      return m ? `${m[1]} ${fmtNum(Number(m[2]))}` : kv.trim();
    }).join(' · ');
  }

  // Translate engine log strings into short human sentences in LANG.
  function humanizeAction(a) {
    const str = String(a || '');
    const isEn = LANG === 'en';
    let m;
    if ((m = /^Pareto synergy converged.*?\((.*)\)\s*$/.exec(str))) return `${isEn ? 'Merged' : '融合成功'} · ${prettyKV(m[1])}`;
    if ((m = /^Pareto advance \((.*)\)\s*$/.exec(str))) return `${isEn ? 'New frontier' : '刷新领先指标'} · ${prettyKV(m[1])}`;
    if ((m = /^Multi-objective validated \((.*)\)\s*$/.exec(str))) return `${isEn ? 'Validated' : '验证通过'} · ${prettyKV(m[1])}`;
    if ((m = /^Baseline measured(?: \((.*)\)|:\s*(.*))\s*$/.exec(str))) return `${isEn ? 'Baseline' : '测得基线'} · ${prettyKV(m[1] || m[2])}`;
    if ((m = /^Benchmark improved:\s*(.*)$/.exec(str))) return `${isEn ? 'Improved' : '指标提升'} · ${prettyKV(m[1])}`;
    if ((m = /^Regressed on (\S+): got (\S+) vs parent (\S+)/.exec(str))) return isEn ? `Regressed · ${m[1]} ${fmtNum(Number(m[2]))} (parent ${fmtNum(Number(m[3]))})` : `指标退化 · ${m[1]} ${fmtNum(Number(m[2]))}（父节点 ${fmtNum(Number(m[3]))}）`;
    if ((m = /HARD_CONSTRAINT_VIOLATED\]\s*(\S+)=(\S+) violates hard gate (\S+) (\S+)/.exec(str))) return isEn ? `Gate violated · ${m[1]} ${fmtNum(Number(m[2]))} (req ${opSym(m[3])} ${fmtNum(Number(m[4]))})` : `违反约束 · ${m[1]} ${fmtNum(Number(m[2]))}，要求 ${opSym(m[3])} ${fmtNum(Number(m[4]))}`;
    if ((m = /HARD_CONSTRAINT_VIOLATED\]\s*missing or invalid required metric "?([^"\s]+)"?/.exec(str))) return isEn ? `Gate violated · missing ${m[1]}` : `违反约束 · 缺少指标 ${m[1]}`;
    if ((m = /^Pre-merge probe done \(textual_conflict=(true|false)\)/.exec(str))) return isEn ? (m[1] === 'true' ? 'Merge probe · textual conflict, switching to semantic synthesis' : 'Merge probe · clean merge') : (m[1] === 'true' ? '合并预检 · 有文本冲突，转入语义融合' : '合并预检 · 无文本冲突');
    if ((m = /^Pareto-dominated by (\S+): got \((.*?)\)/.exec(str))) return isEn ? `Dominated by ${m[1]} · ${prettyKV(m[2])}` : `全面弱于 ${m[1]} · ${prettyKV(m[2])}`;
    if ((m = /^Convergence dominated by parent (\S+)/.exec(str))) return isEn ? `Merge dominated by parent ${m[1]}` : `融合未达标 · 全面弱于父节点 ${m[1]}`;
    if ((m = /^Convergence failed to reach Pareto frontier: \((.*)\)/.exec(str))) return isEn ? `Merge missed frontier · ${prettyKV(m[1])}` : `融合未刷新领先指标 · ${prettyKV(m[1])}`;
    if ((m = /^Convergence did not improve.*?\((.*)\)/.exec(str))) return isEn ? `No merge synergy · ${prettyKV(m[1])}` : `融合无协同增益 · ${prettyKV(m[1])}`;
    if ((m = /AssertionError:\s*(.*)$/.exec(str))) return isEn ? `Assertion failed · ${m[1]}` : `正确性校验失败 · ${m[1]}`;
    if ((m = /^\[SMOKE_GATE_FAILED\]\s*(.*)$/.exec(str))) return isEn ? `Smoke failed · ${m[1]}` : `冒烟测试失败 · ${m[1]}`;
    if ((m = /^\[CONSTRAINT_GATE_FAILED\]\s*(.*)$/.exec(str))) return isEn ? `Gate violated · ${m[1]}` : `违反约束 · ${m[1]}`;
    if ((m = /^Eval failed:\s*(.*)$/.exec(str))) return isEn ? `Eval failed · ${m[1]}` : `评测失败 · ${m[1]}`;
    if (/^Smoke sanity gate passed/.test(str)) return isEn ? 'Smoke gate passed' : '冒烟测试通过';
    return str;
  }

  function setChangesCount(files) {
    const add = (files || []).reduce((acc, f) => acc + f.additions, 0);
    const del = (files || []).reduce((acc, f) => acc + f.deletions, 0);
    $('cnt-changes').textContent = (files || []).length ? `+${add} −${del}` : '';
  }

  // ---------- Unified Focus Sync (Single Source of Truth across Tabs & Chat) ----------
  function syncFocusBackend() {
    if (!S.repo) {
      $('focus-bar')?.classList.add('hidden');
      return;
    }
    const bar = $('focus-bar');
    if (!S.initialized || !S.selectedId) {
      bar.classList.add('hidden');
      return;
    }
    bar.classList.remove('hidden');

    const multiIds = [...S.multiSet].filter((id) => S.tree?.nodes?.[id]);
    const isMulti = multiIds.length >= 2;
    bar.classList.toggle('multi', isMulti);
    $('focus-clear').classList.toggle('hidden', !isMulti);

    let hintSeen = 0;
    try { hintSeen = Number(localStorage.getItem('exptree_hint_seen') || 0); } catch { hintSeen = 99; }
    if (isMulti) {
      $('focus-text').textContent = t('focusMulti', multiIds.length, multiIds.join(' ⊕ '));
      $('focus-hint').textContent = t('focusHintMulti');
      $('focus-hint').classList.remove('hidden');
    } else {
      $('focus-text').textContent = t('focusSingle', S.selectedId);
      $('focus-hint').textContent = t('focusHintSingle');
      $('focus-hint').classList.toggle('hidden', hintSeen >= 3);
      if (hintSeen < 3 && S.lastHintNode !== S.selectedId) {
        S.lastHintNode = S.selectedId;
        try { localStorage.setItem('exptree_hint_seen', String(hintSeen + 1)); } catch { /* sandboxed */ }
      }
    }

    const isPreview = Boolean(!S.standalone && S.previewRepo && S.previewRepo !== S.boundRepo);
    if ((!S.bound && !S.standalone) || isPreview) return;

    const q = new URLSearchParams({
      repo: S.repo,
      node: S.selectedId,
      nodes: multiIds.join(','),
      tab: S.tab,
    });
    if (S.conv) q.set('conversationId', S.conv);
    apiGet(`/api/focus?${q.toString()}`).catch(() => {});
  }

  async function selectNode(nodeId, { toggleExpand = false, openTab = '', scrollFileIdx = null } = {}) {
    if (!nodeId || !S.tree?.nodes?.[nodeId]) return;
    const prevSelected = S.selectedId;
    S.selectedId = nodeId;
    if (prevSelected !== nodeId) {
      S.diffFromId = '';
      S.collapsedFiles.clear();
    }
    if (toggleExpand) {
      S.expandedId = S.expandedId === nodeId ? '' : nodeId;
    } else {
      S.expandedId = nodeId;
    }
    if (scrollFileIdx !== null) {
      S.pendingScrollFileIdx = scrollFileIdx;
    }

    syncFocusBackend();

    // Pre-fetch diff into cache so inline changed-file chips render instantaneously
    ensureNodeDiffCached(nodeId, S.diffFromId).then(() => {
      if (S.tab === 'tree') renderTreeTab();
      if (S.tab === 'changes') renderChangesTab();
    });

    if (openTab) {
      switchTab(openTab);
    } else {
      if (S.tab === 'tree') renderTreeTab();
      if (S.tab === 'changes') renderChangesTab();
      if (S.tab === 'overview') renderOverviewTab();
    }
  }

  function toggleMultiSelect(nodeId) {
    if (S.multiSet.has(nodeId)) S.multiSet.delete(nodeId);
    else S.multiSet.add(nodeId);
    if (!S.selectedId) S.selectedId = nodeId;
    syncFocusBackend();
    renderTreeTab();
  }

  // ---------- Diff Cache ----------
  function diffCacheKey(nodeId, fromId = '') {
    const n = S.tree?.nodes?.[nodeId];
    const effectiveFrom = fromId || parentsOf(n)[0] || '';
    const fromNode = effectiveFrom ? S.tree?.nodes?.[effectiveFrom] : null;
    return `${S.targetKey || S.repo}|${nodeId}|${n?.status || ''}|${n?.git_commit || ''}|${fromId}|${fromNode?.status || ''}|${fromNode?.git_commit || ''}`;
  }

  async function ensureNodeDiffCached(nodeId, fromId = '', { force = false } = {}) {
    if (!S.repo || !nodeId) return null;
    const cacheKey = diffCacheKey(nodeId, fromId);
    if (!force && S.diffCache.has(cacheKey)) return S.diffCache.get(cacheKey);
    if (!force && S.diffInFlight.has(cacheKey)) return S.diffInFlight.get(cacheKey);

    S.diffLoadingKey = cacheKey;
    if (S.tab === 'changes') renderChangesTab();

    const convParam = S.conv ? `&conversationId=${encodeURIComponent(S.conv)}` : '';
    const refreshParam = force ? '&refresh=1' : '';
    const url = `/api/diff?repo=${encodeURIComponent(S.repo)}&to=${encodeURIComponent(nodeId)}&from=${encodeURIComponent(fromId)}${convParam}${refreshParam}`;
    const p = apiGet(url)
      .then((res) => {
        if (!res?.offline) {
          S.diffCache.set(cacheKey, res);
        }
        return res;
      })
      .catch(() => null)
      .finally(() => {
        S.diffInFlight.delete(cacheKey);
        if (S.diffLoadingKey === cacheKey) S.diffLoadingKey = '';
      });
    S.diffInFlight.set(cacheKey, p);
    return p;
  }

  function getCachedDiff(nodeId, fromId = '') {
    return S.diffCache.get(diffCacheKey(nodeId, fromId)) || null;
  }

  // ---------- Tab Navigation ----------
  function switchTab(tabName) {
    S.tab = tabName;
    document.querySelectorAll('.tab[data-tab]').forEach((btn) => {
      const on = btn.dataset.tab === tabName;
      btn.classList.toggle('active', on);
      btn.setAttribute('aria-selected', String(on));
    });
    ['tree', 'changes', 'overview'].forEach((t) => {
      const sec = $(`tab-${t}`);
      if (sec) sec.classList.toggle('active', S.initialized && t === tabName);
    });
    $('uninit-view').classList.toggle('hidden', S.initialized);
    syncFocusBackend();

    if (tabName === 'changes' && S.initialized && S.selectedId) {
      ensureNodeDiffCached(S.selectedId, S.diffFromId).then(() => renderChangesTab());
    }
  }

  // ---------- Conversation-Scoped Binding & Polling ----------
  function resetTreeState({ preserveDeepLink = false } = {}) {
    S.etag = '';
    S.initialized = false;
    S.tree = null;
    S.offline = false;
    S.offlineReason = '';
    S.cachedAt = '';
    S.knownNodeCount = 0;
    S.selectedId = preserveDeepLink ? (urlParams.get('node') || '') : '';
    S.expandedId = S.selectedId;
    if (!preserveDeepLink) S.deepLinkedNode = '';
    S.multiSet.clear();
    S.diffFromId = '';
    S.diffCache.clear();
    S.diffInFlight.clear();
    S.diffLoadingKey = '';
    S.diffAutoCollapsedKey = '';
    S.pendingScrollFileIdx = null;
    S.collapsedFiles.clear();
    S.mapFitPending = true;
    S.paretoYIdx = 0;
    S.paretoXIdx = 1;
    S.paretoSortCol = '';
    S.paretoSortDir = 'best';
  }

  function setMenuOpen(open) {
    S.menuOpen = Boolean(open);
    if (!S.menuOpen) S.rebindListOpen = false;
    $('tree-menu')?.classList.toggle('hidden', !S.menuOpen);
    $('btn-tree-menu')?.setAttribute('aria-expanded', String(S.menuOpen));
    if (!S.menuOpen) {
      $('menu-repo-list')?.classList.add('hidden');
      $('menu-toggle-rebind')?.setAttribute('aria-expanded', 'false');
    }
  }

  function renderMenuRepoList() {
    const list = $('menu-repo-list');
    if (!list) return;
    list.classList.toggle('hidden', !S.rebindListOpen);
    $('menu-toggle-rebind')?.setAttribute('aria-expanded', String(S.rebindListOpen));
    if (!S.rebindListOpen) return;

    if (!S.repos.length) {
      list.innerHTML = `<div class="menu-repo-item" style="color:var(--et-fg-faint);cursor:default;">${esc(t('noLocalRepos'))}</div>`;
      return;
    }
    list.innerHTML = S.repos.map((r) => {
      const cur = S.mode !== 'remote' && r.path === S.boundRepo;
      const tag = cur ? t('tagCurrent') : (r.boundByOther ? t('tagOtherConv') : (r.hasTree ? t('tagNodes', r.nodeCount) : t('tagNoTree')));
      return `<button type="button" class="menu-repo-item ${cur ? 'current' : ''}" data-bind-repo="${esc(r.path)}" title="${esc(r.path)}">
        <span class="menu-repo-name">${esc(r.label)}</span>
        <span class="menu-repo-tag">${esc(tag)}</span>
      </button>`;
    }).join('');
    list.querySelectorAll('[data-bind-repo]').forEach((btn) => {
      btn.addEventListener('click', () => bindRepo(btn.dataset.bindRepo));
    });
  }

  function updateOfflineBanner() {
    const banner = $('offline-banner');
    if (!banner) return;
    const isPreview = Boolean(!S.standalone && S.previewRepo && S.previewRepo !== S.boundRepo);
    const showOffline = Boolean(S.offline && S.mode === 'remote' && !isPreview);
    banner.classList.toggle('hidden', !showOffline);
    if (showOffline) {
      const tm = S.cachedAt ? fmtTime(S.cachedAt) : '';
      $('offline-label').textContent = tm ? t('offlineBannerAt', tm) : t('offlineBanner');
      banner.title = S.offlineReason || '';
    }

    const uTitle = $('uninit-title');
    const uDesc = $('uninit-desc');
    if (uTitle && uDesc && !S.initialized) {
      if (S.mode === 'remote' && !isPreview && S.offline) {
        uTitle.textContent = t('offlineUninitTitle');
        uDesc.textContent = t('offlineUninitDesc', S.remote?.endpoint || S.remote?.ssh_host || 'remote');
      } else if (S.mode === 'remote' && !isPreview) {
        uTitle.textContent = t('remoteUninitTitle');
        uDesc.textContent = t('remoteUninitDesc', S.repo);
      } else {
        uTitle.textContent = t('localUninitTitle');
        uDesc.textContent = t('localUninitDesc');
      }
    }
  }

  function updateViewShell() {
    const isPreview = Boolean(!S.standalone && S.previewRepo && S.previewRepo !== S.boundRepo);
    const showBoard = S.standalone || Boolean(S.repo);

    $('empty-view').classList.toggle('hidden', showBoard);
    $('board-shell').classList.toggle('hidden', !showBoard);

    if (!showBoard) {
      setMenuOpen(false);
      $('focus-bar')?.classList.add('hidden');
      $('offline-banner')?.classList.add('hidden');

      const draftNote = $('empty-draft-note');
      const bindBlock = $('existing-bind-block');
      const sel = $('existing-repo-select');
      const watchText = $('empty-watch-text');

      if (!S.conv) {
        draftNote?.classList.remove('hidden');
        bindBlock?.classList.add('hidden');
        if (watchText) watchText.textContent = t('watchIdle');
      } else {
        draftNote?.classList.add('hidden');
        if (watchText) watchText.textContent = t('watchActive');
        if (!S.repos.length) {
          bindBlock?.classList.add('hidden');
        } else {
          bindBlock?.classList.remove('hidden');
          const sig = S.repos.map((r) => `${r.path}:${r.nodeCount}:${r.boundByOther ? 1 : 0}`).join('|');
          if (sig !== S.lastReposSig && document.activeElement !== sel) {
            S.lastReposSig = sig;
            sel.innerHTML = S.repos.map((r) => {
              const suffix = r.boundByOther ? t('tagOtherConv') : (r.hasTree ? t('tagNodes', r.nodeCount) : t('tagNoTree'));
              return `<option value="${esc(r.path)}">${esc(r.label)} (${esc(suffix)})</option>`;
            }).join('');
          }
        }
      }
      return;
    }

    $('tree-title').textContent = repoShortName() || 'ExpTree';
    const remoteBadge = $('remote-badge');
    if (remoteBadge) {
      const showRemoteBadge = Boolean(!S.standalone && !isPreview && S.mode === 'remote');
      remoteBadge.classList.toggle('hidden', !showRemoteBadge);
      if (showRemoteBadge) {
        const hostLbl = S.remote?.ssh_host || (S.remote?.transport === 'http' ? 'HTTP' : 'Remote');
        remoteBadge.innerHTML = `${ICON_SERVER}<span>${esc(hostLbl)}</span>`;
        remoteBadge.title = S.remote?.transport === 'http'
          ? `HTTP (${S.remote.endpoint}) · ${S.boundRepo}`
          : `SSH (${hostLbl}) · ${S.boundRepo}`;
      }
    }
    $('btn-tree-menu').classList.toggle('hidden', S.standalone || !S.conv);

    const previewEl = $('preview-banner');
    if (isPreview) {
      previewEl.classList.remove('hidden');
      $('preview-label').textContent = t('previewLabel', repoShortName());
      $('btn-preview-back').textContent = S.bound ? t('previewBackBound') : t('previewBack');
    } else {
      previewEl.classList.add('hidden');
    }

    if (S.mode === 'remote' && S.boundRepo) {
      const prefix = S.remote?.ssh_host ? `${S.remote.ssh_host}:` : '';
      $('menu-meta-path').textContent = `${prefix}${S.boundRepo}`;
      $('menu-meta-path').title = S.remote?.transport === 'http'
        ? `HTTP ${S.remote.endpoint} -> ${S.boundRepo}`
        : `SSH ${S.remote?.ssh_host || ''} -> ${S.boundRepo}`;
    } else {
      $('menu-meta-path').textContent = S.boundRepo || t('noBoundRepo');
      $('menu-meta-path').title = S.boundRepo || '';
    }
    $('menu-meta-conv').textContent = t('convPrefix', S.conv);
    $('menu-unbind').classList.toggle('hidden', !S.bound);
    renderMenuRepoList();
    updateOfflineBanner();
  }

  async function syncBinding(isBackgroundPoll = false) {
    void isBackgroundPoll;
    const conv = currentConv();
    if (conv !== S.conv) {
      S.conv = conv;
      S.previewRepo = '';
      resetTreeState({ preserveDeepLink: true });
    }

    const q = new URLSearchParams({ workspaces: S.workspaces.join(',') });
    if (conv) q.set('conversationId', conv);
    const data = await apiGet(`/api/bound?${q.toString()}`);
    if (checkBuildReload(data.buildId)) return false;

    S.standalone = Boolean(data.standalone);
    S.repos = data.repos || [];
    S.bound = Boolean(data.bound);
    S.boundRepo = data.repo || '';
    S.mode = data.mode || 'local';
    S.remote = data.remote || null;

    if (S.previewRepo && S.previewRepo === S.boundRepo) {
      S.previewRepo = '';
    }

    const isPreviewing = Boolean(!S.standalone && S.previewRepo);
    const nextRepo = isPreviewing ? S.previewRepo : (S.bound ? S.boundRepo : '');
    const nextKey = nextRepo
      ? (isPreviewing ? `local|${nextRepo}` : `${S.mode}|${S.remote?.endpoint || ''}|${S.remote?.ssh_host || ''}|${nextRepo}`)
      : '';

    let repoChanged = false;
    if (nextRepo !== S.repo || nextKey !== S.targetKey) {
      S.repo = nextRepo;
      S.targetKey = nextKey;
      resetTreeState({ preserveDeepLink: Boolean(S.deepLinkedNode) });
      if (!S.selectedId && !S.previewRepo && data.focus?.selected_node_id) {
        S.selectedId = data.focus.selected_node_id;
        S.expandedId = S.selectedId;
      }
      repoChanged = true;
    }

    updateViewShell();
    return repoChanged;
  }

  async function bindRepo(repoPath) {
    if (!S.conv) {
      showToast(t('toastNoConv'));
      return;
    }
    try {
      await apiGet(`/api/bind?conversationId=${encodeURIComponent(S.conv)}&repo=${encodeURIComponent(repoPath || '')}`);
      S.previewRepo = '';
      setMenuOpen(false);
      await syncBinding(false);
      await pollState(true);
      syncFocusBackend();
      showToast(repoPath ? t('toastBound') : t('toastUnbound'));
    } catch (err) {
      showToast(`${err.message || err}`);
    }
  }

  async function pollState(force = false) {
    if (!S.repo) return;
    try {
      const etagParam = force ? '' : encodeURIComponent(S.etag);
      const convParam = S.conv ? `&conversationId=${encodeURIComponent(S.conv)}` : '';
      const data = await apiGet(`/api/state?repo=${encodeURIComponent(S.repo)}&etag=${etagParam}${convParam}`);
      if (checkBuildReload(data.buildId)) return;

      S.offline = Boolean(data.offline);
      S.offlineReason = data.offlineReason || '';
      if (data.cachedAt !== undefined) S.cachedAt = data.cachedAt || '';
      updateOfflineBanner();

      if (data.unchanged && !force) return;

      const prevCount = S.knownNodeCount;
      S.etag = data.etag || '';
      S.initialized = Boolean(data.initialized && data.state?.nodes);
      S.tree = data.state || null;

      if (S.initialized) {
        const nodesMap = S.tree.nodes || {};
        const ordered = Object.values(nodesMap).sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
        S.knownNodeCount = ordered.length;

        // Live Auto-Follow within this bound tree only
        const runningNode = ordered.find((n) => n.status === 'EXPLORING' || n.status === 'SYNTHESIZING');
        if (S.follow && !S.deepLinkedNode && (runningNode || (prevCount > 0 && ordered.length > prevCount))) {
          const target = runningNode || ordered[ordered.length - 1];
          S.selectedId = target.id;
          S.expandedId = target.id;
          S.deepLinkedNode = target.id;
        } else if (!S.selectedId || !nodesMap[S.selectedId]) {
          S.selectedId = S.tree.best_node_id || S.tree.active_node_id || S.tree.root_id || ordered[0]?.id || '';
          S.expandedId = S.selectedId;
        }

        if (S.selectedId) {
          ensureNodeDiffCached(S.selectedId, S.diffFromId).then(() => {
            if (S.tab === 'tree') renderTreeTab();
            if (S.tab === 'changes') renderChangesTab();
          });
        }
      } else {
        S.knownNodeCount = 0;
      }

      renderAll();

      if (S.deepLinkedNode && S.initialized) {
        const targetId = S.deepLinkedNode;
        S.deepLinkedNode = '';
        requestAnimationFrame(() => {
          const el = document.querySelector(`[data-group-id="${CSS.escape(targetId)}"]`)
            || document.querySelector(`[data-map-id="${CSS.escape(targetId)}"]`);
          if (el) {
            el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' });
            el.classList.add('flash-focus');
          }
        });
      }
    } catch {
      // ignore transient poll error
    }
  }

  // ---------- Render All ----------
  function renderAll() {
    const goalBar = $('goal-bar');
    if (!S.initialized || !S.tree) {
      goalBar.classList.add('hidden');
      $('cnt-overview').textContent = '';
      $('cnt-tree').textContent = '';
      $('cnt-changes').textContent = '';
      switchTab(S.tab);
      return;
    }

    goalBar.classList.remove('hidden');
    $('goal-text').textContent = S.tree.goal || '';
    $('goal-bar').title = S.tree.goal || '';
    const objs = S.tree.objectives || [];
    if (objs.length >= 2) {
      const specs = objs.map((o) => `${o.name} ${o.higher_is_better ? '↑' : '↓'}`).join(' · ');
      $('goal-dir').textContent = t('goalMulti', specs, (S.tree.constraints || []).length);
    } else {
      $('goal-dir').textContent = t('goalSingle', S.tree.metric_name || 'score', S.tree.higher_is_better ? '↑' : '↓');
    }

    const allNodes = Object.values(S.tree.nodes || {});
    const validatedCnt = allNodes.filter((n) => n.status === 'VALIDATED' || n.status === 'CONVERGED').length;
    $('cnt-overview').textContent = '';
    void validatedCnt;
    $('cnt-tree').textContent = String(allNodes.length);

    renderTreeTab();
    renderOverviewTab();
    if (S.tab === 'changes') renderChangesTab();
    switchTab(S.tab);
  }

  function computeDepthMap(nodesMap) {
    const depth = {};
    const visit = (id, seen = new Set()) => {
      if (depth[id] !== undefined) return depth[id];
      if (seen.has(id) || !nodesMap[id]) return 0;
      seen.add(id);
      const pids = parentsOf(nodesMap[id]);
      if (!pids.length) { depth[id] = 0; return 0; }
      let d = 0;
      for (const p of pids) {
        if (nodesMap[p]) d = Math.max(d, visit(p, new Set(seen)) + 1);
      }
      depth[id] = d;
      return d;
    };
    Object.keys(nodesMap).forEach((k) => visit(k));
    return depth;
  }

  // ---------- Shared Read-Only Node Summary Fragment (Used in Rail & Map) ----------
  function renderNodeSummaryInner(n) {
    const pids = parentsOf(n);
    const diff = getCachedDiff(n.id, '');
    const files = diff?.files || [];
    if (n.id === S.selectedId) setChangesCount(files);

    const filesHtml = files.length ? `
      <div class="changed-files-row">
        ${files.map((f, idx) => `<button type="button" class="changed-chip" data-jump-diff-node="${esc(n.id)}" data-jump-diff-idx="${idx}" title="${esc(t('jumpDiffTitle', f.path))}">
            ${ICON_FILE}<span>${esc(f.path.split('/').pop())}</span>
            <span class="add">+${f.additions}</span><span class="del">−${f.deletions}</span>
          </button>`).join('')}
      </div>` : '';

    const digest = n.prune_digest ? humanizeAction(n.prune_digest) : '';
    const steps = (n.steps || []).filter((st) => st.action !== n.prune_digest);
    return `
      <div class="detail-meta">
        <span>${esc((n.git_commit || 'working').slice(0, 7))}</span>
        <span>${pids.length ? `← ${esc(pids.join(' ⊕ '))}` : esc(t('rootBase'))}</span>
        ${n.agent_id ? `<span>${esc(n.agent_id)}</span>` : ''}
      </div>
      ${digest ? `<div class="callout ${isViolation(n) ? '' : 'muted'}">${esc(digest)}</div>` : ''}
      ${filesHtml}
      ${steps.length ? `<ol class="step-mini">${steps.map((st) => `<li><span>${esc(humanizeAction(st.action))}</span><time>${esc(fmtTime(st.timestamp))}</time></li>`).join('')}</ol>` : ''}
    `;
  }

  function bindSummaryFileJumps(container) {
    container.querySelectorAll('[data-jump-diff-node]').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        e.stopPropagation();
        const nid = btn.dataset.jumpDiffNode;
        const idx = Number(btn.dataset.jumpDiffIdx);
        selectNode(nid, { openTab: 'changes', scrollFileIdx: idx });
      });
    });
  }

  // ---------- Shared visual helpers ----------
  const RAIL_MID = 19;

  // Collapse verbose "Converge [node-2 + node-3]: A ⊕ B" titles into a short headline + muted subtitle.
  function hypoParts(n) {
    const h = n.hypothesis || (parentsOf(n).length ? '' : t('rootBase'));
    const m = /^Converge \[([^\]]+)\]:\s*(.*)$/.exec(h);
    if (m) {
      const sub = m[2].split(' ⊕ ')
        .map((s) => s.replace(/\(.*?\)/g, '').trim().split(/\s+/).slice(0, 4).join(' '))
        .join(' ⊕ ');
      return { title: t('mergedPrefix', m[1].replace(/\s*\+\s*/g, ' ⊕ ')), sub };
    }
    return { title: h || 'Baseline', sub: '' };
  }

  // Ancestors + descendants of the selected node (empty set disables spotlight on tiny trees).
  function computeLineage(selId, nodesMap) {
    const set = new Set();
    const all = Object.values(nodesMap);
    if (!selId || !nodesMap[selId] || all.length <= 3) return set;
    const up = [selId];
    while (up.length) {
      const id = up.pop();
      if (set.has(id)) continue;
      set.add(id);
      parentsOf(nodesMap[id]).forEach((p) => { if (nodesMap[p]) up.push(p); });
    }
    const down = [selId];
    const seen = new Set();
    while (down.length) {
      const id = down.pop();
      if (seen.has(id)) continue;
      seen.add(id);
      set.add(id);
      all.forEach((c) => { if (parentsOf(c).includes(id)) down.push(c.id); });
    }
    return set;
  }

  // Continue every active lane through the expanded detail panel so rails never break.
  function renderRailContinuationSVG(row, totalCols) {
    const stepX = 14, padX = 14;
    const W = Math.max(42, padX * 2 + (totalCols - 1) * stepX);
    const parts = [];
    row.bottomLanes.forEach((owner, c) => {
      if (!owner) return;
      const x = padX + c * stepX;
      const color = (c === row.lane && row.node.status === 'CONVERGED') ? 'var(--et-conv)' : LANE_COLORS[c % LANE_COLORS.length];
      parts.push(`<line x1="${x}" y1="0" x2="${x}" y2="100%" stroke="${color}" stroke-width="2" stroke-opacity="${c === row.lane ? 1 : 0.75}"/>`);
    });
    return `<span class="rail-spacer" style="width:${W}px"></span><svg class="rail-cell-svg" width="${W}" height="100%">${parts.join('')}</svg>`;
  }

  // GitHub-style intraline emphasis: pair del/add runs and highlight the differing middle span.
  function annotateWordDiff(lines) {
    const out = lines.map((l) => ({ ...l }));
    let i = 0;
    while (i < out.length) {
      if (out[i].type !== 'del') { i++; continue; }
      let j = i;
      while (j < out.length && out[j].type === 'del') j++;
      let k = j;
      while (k < out.length && out[k].type === 'add') k++;
      const dels = out.slice(i, j);
      const adds = out.slice(j, k);
      const pairs = Math.min(dels.length, adds.length);
      for (let p = 0; p < pairs; p++) {
        const a = dels[p].content || '';
        const b = adds[p].content || '';
        const maxPre = Math.min(a.length, b.length);
        let pre = 0;
        while (pre < maxPre && a[pre] === b[pre]) pre++;
        let suf = 0;
        while (suf < maxPre - pre && a[a.length - 1 - suf] === b[b.length - 1 - suf]) suf++;
        const changed = (a.length + b.length - 2 * (pre + suf)) / Math.max(1, a.length + b.length);
        if (changed > 0.7) continue;
        const wrap = (s) => esc(s.slice(0, pre))
          + (s.length - pre - suf > 0 ? `<span class="wd">${esc(s.slice(pre, s.length - suf))}</span>` : '')
          + esc(s.slice(s.length - suf));
        dels[p].html = wrap(a);
        adds[p].html = wrap(b);
      }
      i = Math.max(k, j);
    }
    return out;
  }

  function bindChartTips(container) {
    const tip = $('chart-tip');
    if (!tip) return;
    container.querySelectorAll('[data-tip]').forEach((g) => {
      g.addEventListener('mouseenter', () => {
        tip.textContent = g.dataset.tip;
        tip.classList.remove('hidden');
      });
      g.addEventListener('mousemove', (e) => {
        const r = tip.getBoundingClientRect();
        let x = e.clientX + 12;
        let y = e.clientY + 12;
        if (x + r.width > window.innerWidth - 6) x = e.clientX - r.width - 12;
        if (y + r.height > window.innerHeight - 6) y = e.clientY - r.height - 12;
        tip.style.left = `${Math.max(4, x)}px`;
        tip.style.top = `${Math.max(4, y)}px`;
      });
      g.addEventListener('mouseleave', () => tip.classList.add('hidden'));
    });
  }

  function fmtTick(v) {
    if (v === 0) return '0';
    const abs = Math.abs(v);
    if (abs < 1e-3 || abs >= 1e6) return v.toExponential(0).replace('e+', 'e');
    return String(+v.toPrecision(4));
  }

  // Round, human-friendly tick positions (decades on log axes, 1/2/2.5/5 steps on linear axes).
  function niceTicks(minT, maxT, useLog) {
    if (useLog) {
      const ks = [];
      for (let k = Math.ceil(minT); k <= Math.floor(maxT); k++) ks.push(k);
      let pick = ks;
      while (pick.length > 4) pick = pick.filter((_, i) => i % 2 === 0);
      if (pick.length >= 2) {
        return pick.map((k) => ({ t: k, label: (k >= -2 && k <= 4) ? String(+Math.pow(10, k).toPrecision(1)) : `1e${k}` }));
      }
      return [minT, maxT].map((t) => ({ t, label: fmtTick(Math.pow(10, t)) }));
    }
    const span = Math.max(maxT - minT, 1e-12);
    const raw = span / 3;
    const mag = Math.pow(10, Math.floor(Math.log10(raw)));
    const step = [1, 2, 2.5, 5, 10].map((f) => f * mag).find((st) => st >= raw);
    const out = [];
    for (let k = Math.ceil(minT / step); k * step <= maxT + 1e-12; k++) {
      const v = +(k * step).toPrecision(12);
      out.push({ t: v, label: fmtTick(v) });
    }
    return out;
  }

  function buildParetoTableInner(ordered, objs, pFrontIds) {
    const rows = ordered.filter((n) => n.metrics && objs.some((o) => typeof n.metrics[o.name] === 'number' && Number.isFinite(n.metrics[o.name])));
    if (!rows.length) return '';

    const legalSucc = rows.filter((n) => (n.status === 'VALIDATED' || n.status === 'CONVERGED') && !isViolation(n));
    const bestBasis = legalSucc.length ? legalSucc : rows.filter((n) => !isViolation(n));
    const isEligible = (n) => (legalSucc.length ? ((n.status === 'VALIDATED' || n.status === 'CONVERGED') && !isViolation(n)) : !isViolation(n));

    const colBest = {};
    for (const o of objs) {
      const vals = bestBasis
        .map((n) => n.metrics[o.name])
        .filter((v) => typeof v === 'number' && Number.isFinite(v));
      if (vals.length) {
        colBest[o.name] = o.higher_is_better ? Math.max(...vals) : Math.min(...vals);
      }
    }
    const isColBest = (n, o, v) => {
      if (!isEligible(n) || typeof v !== 'number' || !Number.isFinite(v) || colBest[o.name] === undefined) return false;
      const b = colBest[o.name];
      return Math.abs(v - b) <= 1e-12 * Math.max(1, Math.abs(b));
    };

    const sortedRows = [...rows];
    const sortObj = S.paretoSortCol ? objs.find((o) => o.name === S.paretoSortCol) : null;
    if (sortObj) {
      sortedRows.sort((a, b) => {
        const va = a.metrics[sortObj.name];
        const vb = b.metrics[sortObj.name];
        const ha = typeof va === 'number' && Number.isFinite(va);
        const hb = typeof vb === 'number' && Number.isFinite(vb);
        if (!ha && !hb) return 0;
        if (!ha) return 1;
        if (!hb) return -1;
        let cmp = sortObj.higher_is_better ? (vb - va) : (va - vb); // 'best' first
        if (S.paretoSortDir === 'worst') cmp = -cmp;
        return cmp;
      });
    }

    const thCols = objs.map((o) => {
      const active = S.paretoSortCol === o.name;
      const sortArrow = active ? (S.paretoSortDir === 'best' ? ' ▾' : ' ▴') : '';
      const dirTxt = o.higher_is_better ? t('higherBetter') : t('lowerBetter');
      return `<th class="num sortable ${active ? 'sorted' : ''}" data-pareto-sort="${esc(o.name)}" title="${esc(o.name)} · ${esc(dirTxt)}">${esc(o.name)} ${o.higher_is_better ? '↑' : '↓'}${sortArrow}</th>`;
    }).join('');

    const bodyRows = sortedRows.map((n) => {
      const onFront = pFrontIds.includes(n.id);
      const viol = isViolation(n);
      const isSel = S.selectedId === n.id;
      const dotCls = viol ? 'violation' : (n.status === 'PRUNED' ? 'prune' : (n.status === 'CONVERGED' ? 'conv' : (onFront ? 'lead' : (n.status === 'VALIDATED' ? 'pass' : 'run'))));
      const rowCls = [
        n.status === 'PRUNED' ? 'is-pruned' : '',
        viol ? 'is-violation' : '',
        onFront ? 'is-lead' : '',
        isSel ? 'selected' : '',
      ].filter(Boolean).join(' ');
      const tds = objs.map((o) => {
        const v = n.metrics[o.name];
        if (typeof v !== 'number' || !Number.isFinite(v)) {
          return '<td class="num muted">—</td>';
        }
        const best = isColBest(n, o, v);
        return `<td class="num ${best ? 'col-best' : ''}">${best ? `<strong>${esc(fmtNum(v))}</strong>` : esc(fmtNum(v))}</td>`;
      }).join('');
      return `
        <tr class="pareto-tr ${rowCls}" data-jump-node="${esc(n.id)}" title="${esc(hypoParts(n).title)}">
          <td class="col-node"><span class="pt-dot ${dotCls}"></span><span class="pt-id">${esc(n.id)}</span>${onFront ? '<span class="pt-star" title="Pareto">★</span>' : ''}</td>
          ${tds}
        </tr>`;
    }).join('');

    return `
      <div class="pareto-table-wrap">
        <table class="pareto-table">
          <thead>
            <tr>
              <th class="col-node sortable ${!S.paretoSortCol ? 'sorted' : ''}" data-pareto-sort="">${esc(t('tblNode'))}</th>
              ${thCols}
            </tr>
          </thead>
          <tbody>${bodyRows}</tbody>
        </table>
      </div>`;
  }

  // 2D Pareto projection plot + N-objective comparison table toggle.
  function buildParetoPlot(ordered, objs, pFrontIds, root) {
    const isTable = S.paretoView === 'table';
    const hasTableRows = ordered.some((n) => n.metrics && objs.some((o) => typeof n.metrics[o.name] === 'number' && Number.isFinite(n.metrics[o.name])));

    let yIdx = Math.max(0, Math.min(objs.length - 1, S.paretoYIdx || 0));
    let xIdx = Math.max(0, Math.min(objs.length - 1, S.paretoXIdx ?? 1));
    if (xIdx === yIdx) xIdx = (yIdx + 1) % objs.length;
    const objY = objs[yIdx];
    const objX = objs[xIdx];
    const pts = ordered.filter((n) => n.metrics && typeof n.metrics[objX.name] === 'number' && typeof n.metrics[objY.name] === 'number');

    if (isTable) {
      if (!hasTableRows) return '';
    } else if (!pts.length) {
      return hasTableRows ? buildParetoPlotTableShell(ordered, objs, pFrontIds) : '';
    }

    const optsHtml = (curIdx) => objs.map((o, idx) => `<option value="${idx}" ${idx === curIdx ? 'selected' : ''}>${esc(o.name)}</option>`).join('');
    const axisPicker = (!isTable && objs.length >= 3)
      ? `<div class="pareto-axes">
          <label>${esc(t('axisY'))}<select class="pareto-axis-sel" data-pareto-axis="y">${optsHtml(yIdx)}</select></label>
          <label>${esc(t('axisX'))}<select class="pareto-axis-sel" data-pareto-axis="x">${optsHtml(xIdx)}</select></label>
        </div>`
      : '';
    const viewToggle = `
      <div class="segmented xs">
        <button type="button" class="${!isTable ? 'on' : ''}" data-pareto-view="plot">${esc(t('paretoPlot'))}</button>
        <button type="button" class="${isTable ? 'on' : ''}" data-pareto-view="table">${esc(t('paretoTable'))}</button>
      </div>`;

    if (isTable) {
      return `
        <div class="card">
          <div class="card-head">
            <span class="card-title">${esc(t('paretoTitle'))}</span>
            <div class="pareto-head-ctrl">${viewToggle}</div>
          </div>
          ${buildParetoTableInner(ordered, objs, pFrontIds)}
        </div>`;
    }
    const W = 460, H = 212, padL = 54, padR = 18, padT = 22, padB = 34;
    const plotW = W - padL - padR;
    const plotH = H - padT - padB;

    // Constraint violators never stretch the axes; they are pinned to the frame edge instead.
    const legal = pts.filter((n) => !isViolation(n));
    const basis = legal.length ? legal : pts;

    const makeAxis = (obj, vals) => {
      const lo = Math.min(...vals);
      const hi = Math.max(...vals);
      const useLog = lo > 0 && hi / lo > 20;
      const tf = (v) => (useLog ? Math.log10(Math.max(v, 1e-300)) : v);
      let minT = Math.min(...vals.map(tf));
      let maxT = Math.max(...vals.map(tf));
      const margin = (maxT - minT) * 0.12 || (useLog ? 0.5 : (Math.abs(maxT) * 0.1 || 1));
      minT -= margin;
      maxT += margin;
      const span = maxT - minT;
      const goodOfT = (t) => {
        const raw = (t - minT) / span;
        return obj.higher_is_better ? raw : 1 - raw;
      };
      return { useLog, minT, maxT, goodOfT, goodness: (v) => goodOfT(tf(v)) };
    };
    const axX = makeAxis(objX, basis.map((n) => n.metrics[objX.name]));
    const axY = makeAxis(objY, basis.map((n) => n.metrics[objY.name]));
    const clampG = (g) => Math.min(1, Math.max(0, g));
    const xOfG = (g) => padL + clampG(g) * plotW;
    const yOfG = (g) => padT + (1 - clampG(g)) * plotH;

    const mapped = pts.map((n) => {
      const gx = axX.goodness(n.metrics[objX.name]);
      const gy = axY.goodness(n.metrics[objY.name]);
      return { n, gx, gy, x: xOfG(gx), y: yOfG(gy), pinned: gx < 0 || gx > 1 || gy < 0 || gy > 1 };
    });

    const xTicks = niceTicks(axX.minT, axX.maxT, axX.useLog).map((tk) => {
      const x = xOfG(axX.goodOfT(tk.t));
      return `<line x1="${x}" y1="${padT}" x2="${x}" y2="${padT + plotH}" stroke="var(--et-line)" stroke-width="1"/>
        <line x1="${x}" y1="${padT + plotH}" x2="${x}" y2="${padT + plotH + 3}" stroke="var(--et-line-strong)"/>
        <text x="${x}" y="${padT + plotH + 14}" text-anchor="middle" class="ax-tick">${esc(tk.label)}</text>`;
    }).join('');
    const yTicks = niceTicks(axY.minT, axY.maxT, axY.useLog).map((tk) => {
      const y = yOfG(axY.goodOfT(tk.t));
      return `<line x1="${padL}" y1="${y}" x2="${padL + plotW}" y2="${y}" stroke="var(--et-line)" stroke-width="1"/>
        <line x1="${padL - 3}" y1="${y}" x2="${padL}" y2="${y}" stroke="var(--et-line-strong)"/>
        <text x="${padL - 6}" y="${y + 3}" text-anchor="end" class="ax-tick">${esc(tk.label)}</text>`;
    }).join('');

    const rp = mapped.find((p) => p.n.id === root?.id);
    const baseline = rp && !rp.pinned ? `
      <rect x="${rp.x}" y="${padT}" width="${Math.max(0, padL + plotW - rp.x)}" height="${Math.max(0, rp.y - padT)}" fill="var(--et-pass-bg)" opacity="0.65"/>
      <line x1="${rp.x}" y1="${padT}" x2="${rp.x}" y2="${padT + plotH}" stroke="var(--et-line-strong)" stroke-dasharray="2 3"/>
      <line x1="${padL}" y1="${rp.y}" x2="${padL + plotW}" y2="${rp.y}" stroke="var(--et-line-strong)" stroke-dasharray="2 3"/>
      <text x="${padL + plotW - 6}" y="${padT + 13}" text-anchor="end" class="ax-note">${esc(t('beatsBase'))}</text>
    ` : '';

    const frontPts = mapped.filter((p) => pFrontIds.includes(p.n.id)).sort((a, b) => a.gx - b.gx);
    const frontPoly = frontPts.map((p, idx) => `${idx === 0 ? 'M' : 'L'}${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ');

    const priority = (p) => (p.n.status === 'CONVERGED' ? 0 : (pFrontIds.includes(p.n.id) ? 1 : (p.n.status === 'PRUNED' ? 3 : 2)));
    const byPriority = [...mapped].sort((a, b) => priority(a) - priority(b));
    const placed = [];
    const labelPos = new Map();
    for (const p of byPriority) {
      const isSel = S.selectedId === p.n.id;
      const d = isSel ? 12 : 9;
      const candidates = [
        { x: p.x, y: p.y - d, anchor: 'middle' },
        { x: p.x, y: p.y + d + 6, anchor: 'middle' },
        { x: p.x + d + 1, y: p.y + 3, anchor: 'start' },
        { x: p.x - d - 1, y: p.y + 3, anchor: 'end' },
      ];
      const fitsFrame = (c) => {
        const minX = c.anchor === 'start' ? c.x : (c.anchor === 'end' ? c.x - 24 : c.x - 12);
        const maxX = c.anchor === 'start' ? c.x + 24 : (c.anchor === 'end' ? c.x : c.x + 12);
        return minX >= padL + 2 && maxX <= padL + plotW - 2 && c.y >= padT + 9 && c.y <= padT + plotH - 3;
      };
      const hasConflict = (c) => (
        placed.some((q) => Math.abs(q.x - c.x) < 24 && Math.abs(q.y - c.y) < 11)
        || mapped.some((m) => m.n.id !== p.n.id && Math.abs(m.x - c.x) < 12 && Math.abs(m.y - c.y) < 9)
        || Boolean(rp && !rp.pinned && c.y <= padT + 20 && c.x >= padL + plotW - 92)
      );
      const chosen = candidates.find((c) => fitsFrame(c) && !hasConflict(c))
        || candidates.find((c) => fitsFrame(c))
        || candidates[0];
      placed.push({ x: chosen.x, y: chosen.y });
      labelPos.set(p.n.id, chosen);
    }

    const markers = [...byPriority].reverse().map((p) => {
      const { n, x, y } = p;
      const onFront = pFrontIds.includes(n.id);
      const isSel = S.selectedId === n.id;
      const sx = x.toFixed(1), sy = y.toFixed(1);
      const lp = labelPos.get(n.id);
      const lx = lp.x.toFixed(1), ly = lp.y.toFixed(1);
      const allObjLines = objs
        .filter((o) => typeof n.metrics[o.name] === 'number')
        .map((o) => `${o.name} = ${fmtNum(n.metrics[o.name])}`)
        .join('\n');
      const tip = `${n.id} · ${n.status}${p.pinned ? t('outOfRange') : ''}\n${allObjLines}\n${hypoParts(n).title.slice(0, 60)}`;
      const ring = isSel ? `<circle cx="${sx}" cy="${sy}" r="8.5" fill="none" stroke="var(--et-primary)" stroke-width="1.5"/>` : '';
      const label = (fill, weight) => `<text x="${lx}" y="${ly}" text-anchor="${lp.anchor}" class="ax-label" font-weight="${weight}" fill="${fill}">${esc(n.id.replace('node-', '#'))}</text>`;
      let glyph;
      if (n.status === 'PRUNED') {
        const c = isViolation(n) ? 'var(--et-prune)' : 'var(--et-fg-faint)';
        glyph = `<circle cx="${sx}" cy="${sy}" r="4" fill="var(--et-card)" stroke="${c}" stroke-width="1.5"/>
          <path d="M ${x - 1.8} ${y - 1.8} L ${x + 1.8} ${y + 1.8} M ${x + 1.8} ${y - 1.8} L ${x - 1.8} ${y + 1.8}" stroke="${c}" stroke-width="1.35"/>` + label(c, 400);
      } else if (n.status === 'CONVERGED') {
        glyph = `<polygon points="${x},${y - 5.5} ${x + 5.5},${y} ${x},${y + 5.5} ${x - 5.5},${y}" fill="var(--et-conv)" stroke="var(--et-card)" stroke-width="1.5"/>` + label('var(--et-conv)', 600);
      } else if (onFront) {
        glyph = `<circle cx="${sx}" cy="${sy}" r="4.5" fill="var(--et-pass)" stroke="var(--et-card)" stroke-width="1.5"/>` + label('var(--et-fg)', 600);
      } else {
        glyph = `<circle cx="${sx}" cy="${sy}" r="3.8" fill="var(--et-card)" stroke="var(--et-fg-muted)" stroke-width="1.5"/>` + label('var(--et-fg-muted)', 400);
      }
      return `<g class="chart-pt" data-jump-node="${esc(n.id)}" data-tip="${esc(tip)}"><circle cx="${sx}" cy="${sy}" r="10" fill="transparent"/>${ring}${glyph}</g>`;
    }).join('');

    const dir = (o) => (o.higher_is_better ? t('higherBetter') : t('lowerBetter'));
    const xTitle = `${objX.name} · ${dir(objX)}${axX.useLog ? ' · log' : ''}  →`;
    const yTitle = `↑  ${objY.name} · ${dir(objY)}${axY.useLog ? ' · log' : ''}`;
    return `
      <div class="card">
        <div class="card-head">
          <span class="card-title">${esc(t('paretoTitle'))}</span>
          <div class="pareto-head-ctrl">${axisPicker}${viewToggle}</div>
        </div>
        <svg class="pareto-svg" viewBox="0 0 ${W} ${H}">
          <rect x="${padL}" y="${padT}" width="${plotW}" height="${plotH}" fill="var(--et-field)" rx="3"/>
          ${baseline}
          ${frontPts.length >= 2 ? `<path d="${frontPoly}" fill="none" stroke="var(--et-pass)" stroke-width="1.5" stroke-opacity="0.85"/>` : ''}
          ${xTicks}${yTicks}
          ${markers}
          <text x="${padL + plotW / 2}" y="${H - 5}" text-anchor="middle" class="ax-title">${esc(xTitle)}</text>
          <text x="${padL}" y="${padT - 8}" text-anchor="start" class="ax-title">${esc(yTitle)}</text>
        </svg>
        <div class="pareto-foot-note">${esc(t('paretoNote'))}</div>
      </div>`;
  }

  function buildParetoPlotTableShell(ordered, objs, pFrontIds) {
    const inner = buildParetoTableInner(ordered, objs, pFrontIds);
    if (!inner) return '';
    return `
      <div class="card">
        <div class="card-head">
          <span class="card-title">${esc(t('paretoTitle'))}</span>
          <span class="card-note">${esc(t('paretoTable'))}</span>
        </div>
        ${inner}
      </div>`;
  }

  // Single-metric trend: best-so-far step envelope + per-node scatter (siblings are parallel, not sequential).
  function buildSparkChart(scored, root, best, hib) {
    const W = 460, H = 136, padL = 54, padR = 14, padT = 14, padB = 24;
    const plotW = W - padL - padR;
    const plotH = H - padT - padB;
    const baseVal = typeof root?.primary_score === 'number' ? root.primary_score : scored[0].primary_score;
    const bestVal = typeof best?.primary_score === 'number' ? best.primary_score : baseVal;
    const span = Math.max(Math.abs(bestVal - baseVal), Math.abs(baseVal) * 0.15, 1e-9);
    const clampVal = (v) => (hib ? Math.max(baseVal - span * 0.65, v) : Math.min(baseVal + span * 0.85, v));
    const vals = scored.map((n) => clampVal(n.primary_score));
    const minV = Math.min(...vals);
    const maxV = Math.max(...vals);
    const vRange = Math.max(maxV - minV, 1e-12);
    const yOf = (v) => {
      const norm = (clampVal(v) - minV) / vRange;
      const goodUp = hib ? norm : (1 - norm);
      return padT + 4 + (1 - goodUp) * (plotH - 8);
    };
    const xOf = (i) => padL + 6 + (scored.length === 1 ? 0 : (i / (scored.length - 1)) * (plotW - 12));

    let runBest = null;
    const env = [];
    scored.forEach((n, i) => {
      if (n.status === 'VALIDATED' || n.status === 'CONVERGED') {
        if (runBest === null || (hib ? n.primary_score > runBest : n.primary_score < runBest)) runBest = n.primary_score;
      }
      if (runBest !== null) env.push({ x: xOf(i), y: yOf(runBest) });
    });
    let envPath = '';
    if (env.length) {
      envPath = `M ${env[0].x} ${env[0].y}`;
      for (let k = 1; k < env.length; k++) envPath += ` H ${env[k].x} V ${env[k].y}`;
      envPath += ` H ${padL + plotW}`;
    }

    const baseY = yOf(baseVal);
    const bestY = yOf(bestVal);
    const dots = scored.map((n, i) => {
      const x = xOf(i), y = yOf(n.primary_score);
      const color = n.status === 'PRUNED' ? (isViolation(n) ? 'var(--et-prune)' : 'var(--et-fg-faint)') : (n.status === 'CONVERGED' ? 'var(--et-conv)' : 'var(--et-pass)');
      const isSel = S.selectedId === n.id;
      const tip = `${n.id} · ${n.status}\n${S.tree.metric_name || 'score'} = ${fmtNum(n.primary_score)}\n${hypoParts(n).title.slice(0, 60)}`;
      return `<g class="chart-pt" data-jump-node="${esc(n.id)}" data-tip="${esc(tip)}">
        <circle cx="${x}" cy="${y}" r="10" fill="transparent"/>
        ${isSel ? `<circle cx="${x}" cy="${y}" r="8" fill="none" stroke="var(--et-primary)" stroke-width="1.5"/>` : ''}
        <circle cx="${x}" cy="${y}" r="${n.id === best?.id ? 5 : 3.8}" fill="${n.status === 'PRUNED' ? 'var(--et-card)' : color}" stroke="${color}" stroke-width="2"/>
        <text x="${x}" y="${H - 6}" text-anchor="middle" font-size="9" font-family="var(--et-mono)" fill="var(--et-fg-muted)">${esc(n.id.replace('node-', '#'))}</text>
      </g>`;
    }).join('');

    return `
      <div class="card">
        <div class="card-head">
          <span class="card-title">${esc(t('sparkTitle'))}</span>
          <span class="card-note">${esc(t('sparkNote'))}</span>
        </div>
        <svg class="spark-svg" viewBox="0 0 ${W} ${H}">
          <rect x="${padL}" y="${padT}" width="${plotW}" height="${plotH}" fill="var(--et-field)" rx="3"/>
          <line x1="${padL}" y1="${baseY}" x2="${padL + plotW}" y2="${baseY}" stroke="var(--et-fg-muted)" stroke-dasharray="3 3"/>
          <text x="${padL - 5}" y="${baseY + 3}" text-anchor="end" font-size="9" font-family="var(--et-mono)" fill="var(--et-fg-faint)">${esc(fmtNum(baseVal))}</text>
          ${Math.abs(bestY - baseY) > 10 ? `<text x="${padL - 5}" y="${bestY + 3}" text-anchor="end" font-size="9" font-family="var(--et-mono)" fill="var(--et-pass)">${esc(fmtNum(bestVal))}</text>` : ''}
          ${envPath ? `<path d="${envPath}" fill="none" stroke="var(--et-pass)" stroke-width="2" stroke-linejoin="round"/>` : ''}
          ${dots}
        </svg>
      </div>`;
  }

  // ---------- Tab 1A: Metro-Rail Vertical List ----------
  function computeMetroLayout(nodes) {
    const remChildren = {};
    for (const n of nodes) remChildren[n.id] = 0;
    for (const n of nodes) {
      for (const p of parentsOf(n)) {
        if (remChildren[p] !== undefined) remChildren[p]++;
      }
    }

    const activeLanes = [];
    const nodeLane = {};
    const rows = [];

    const allocLane = (preferred = 0) => {
      for (let i = preferred; i < activeLanes.length; i++) {
        if (activeLanes[i] === null) return i;
      }
      activeLanes.push(null);
      return activeLanes.length - 1;
    };

    for (const n of nodes) {
      const topLanes = [...activeLanes];
      const pids = parentsOf(n).filter((p) => nodeLane[p] !== undefined);
      const incomingLanes = pids.map((p) => {
        const idx = activeLanes.indexOf(p);
        return { pid: p, lane: idx !== -1 ? idx : nodeLane[p] };
      });

      let myLane;
      if (incomingLanes.length > 0) {
        const prim = incomingLanes[0];
        myLane = (activeLanes[prim.lane] === prim.pid && remChildren[prim.pid] === 1) ? prim.lane : allocLane(0);
      } else {
        myLane = allocLane(0);
      }
      nodeLane[n.id] = myLane;

      const nextLanes = [...activeLanes];
      while (nextLanes.length <= myLane) nextLanes.push(null);
      for (const p of pids) {
        remChildren[p] = Math.max(0, (remChildren[p] || 1) - 1);
        if (remChildren[p] === 0) {
          const pSlot = nextLanes.indexOf(p);
          if (pSlot !== -1 && pSlot !== myLane) nextLanes[pSlot] = null;
        }
      }
      nextLanes[myLane] = (remChildren[n.id] || 0) > 0 ? n.id : null;
      while (nextLanes.length && nextLanes[nextLanes.length - 1] === null) nextLanes.pop();

      rows.push({
        node: n,
        lane: myLane,
        topLanes,
        bottomLanes: [...nextLanes],
        incomingLanes,
        maxCols: Math.max(1, topLanes.length, nextLanes.length, myLane + 1),
      });
      activeLanes.length = 0;
      activeLanes.push(...nextLanes);
    }

    const totalCols = Math.max(1, ...rows.map((r) => r.maxCols));
    return { rows, totalCols };
  }

  function renderRailCellSVG(row, totalCols, bestId) {
    const stepX = 14, padX = 14;
    const W = Math.max(42, padX * 2 + (totalCols - 1) * stepX);
    const midY = RAIL_MID;
    const xOf = (col) => padX + col * stepX;
    const colorOf = (col) => LANE_COLORS[col % LANE_COLORS.length];

    const parts = [];
    const maxL = Math.max(row.topLanes.length, row.bottomLanes.length);
    for (let c = 0; c < maxL; c++) {
      const topOwner = row.topLanes[c];
      const botOwner = row.bottomLanes[c];
      const x = xOf(c);
      if (topOwner && c !== row.lane && !row.incomingLanes.some((inc) => inc.lane === c && botOwner !== topOwner)) {
        parts.push(`<line x1="${x}" y1="0" x2="${x}" y2="${midY}" stroke="${colorOf(c)}" stroke-width="2" stroke-opacity="0.75"/>`);
      }
      if (botOwner && c !== row.lane) {
        parts.push(`<line x1="${x}" y1="${midY}" x2="${x}" y2="100%" stroke="${colorOf(c)}" stroke-width="2" stroke-opacity="0.75"/>`);
      }
    }

    const nx = xOf(row.lane);
    for (const inc of row.incomingLanes) {
      const px = xOf(inc.lane);
      const stroke = row.node.status === 'PRUNED' ? (isViolation(row.node) ? 'var(--et-prune)' : 'var(--et-fg-faint)')
        : (row.node.status === 'CONVERGED' ? 'var(--et-conv)' : colorOf(inc.lane));
      const dash = row.node.status === 'PRUNED' ? 'stroke-dasharray="3 2"' : '';
      if (px === nx) {
        parts.push(`<line x1="${px}" y1="0" x2="${nx}" y2="${midY}" stroke="${stroke}" stroke-width="2" ${dash}/>`);
      } else {
        parts.push(`<path d="M ${px} 0 C ${px} ${midY * 0.65}, ${nx} ${midY * 0.35}, ${nx} ${midY}" fill="none" stroke="${stroke}" stroke-width="2" ${dash}/>`);
      }
      if (row.bottomLanes[inc.lane] === inc.pid && inc.lane !== row.lane) {
        parts.push(`<line x1="${px}" y1="0" x2="${px}" y2="100%" stroke="${colorOf(inc.lane)}" stroke-width="2" stroke-opacity="0.75"/>`);
      }
    }

    if (row.bottomLanes[row.lane] === row.node.id) {
      const outColor = row.node.status === 'CONVERGED' ? 'var(--et-conv)' : colorOf(row.lane);
      parts.push(`<line x1="${nx}" y1="${midY}" x2="${nx}" y2="100%" stroke="${outColor}" stroke-width="2"/>`);
    }

    const st = row.node.status;
    if (st === 'CONVERGED') {
      parts.push(`<polygon points="${nx},${midY - 6} ${nx + 6},${midY} ${nx},${midY + 6} ${nx - 6},${midY}" fill="var(--et-conv)" stroke="var(--et-card)" stroke-width="1.5"/>`);
    } else if (st === 'PRUNED') {
      const pc = isViolation(row.node) ? 'var(--et-prune)' : 'var(--et-fg-faint)';
      parts.push(`<circle cx="${nx}" cy="${midY}" r="4.5" fill="var(--et-card)" stroke="${pc}" stroke-width="1.6"/>`);
      parts.push(`<path d="M ${nx - 2} ${midY - 2} L ${nx + 2} ${midY + 2} M ${nx + 2} ${midY - 2} L ${nx - 2} ${midY + 2}" stroke="${pc}" stroke-width="1.4"/>`);
    } else if (st === 'EXPLORING' || st === 'SYNTHESIZING') {
      const ph = st === 'EXPLORING' ? activePhase(row.node) : 'evaluating';
      if (ph === 'queued') {
        parts.push(`<circle cx="${nx}" cy="${midY}" r="4.5" fill="var(--et-card)" stroke="var(--et-queue)" stroke-width="1.8"/>`);
      } else if (ph === 'preparing') {
        parts.push(`<circle cx="${nx}" cy="${midY}" r="4.5" fill="var(--et-card)" stroke="var(--et-fg-muted)" stroke-width="1.6" stroke-dasharray="2.5 2"/>`);
      } else {
        parts.push(`<circle class="pulse-ring" cx="${nx}" cy="${midY}" r="8" fill="var(--et-run-bg)"/>`);
        parts.push(`<circle cx="${nx}" cy="${midY}" r="4.5" fill="var(--et-run)"/>`);
      }
    } else {
      const fill = row.node.id === S.tree?.root_id ? 'var(--et-fg-muted)' : 'var(--et-pass)';
      parts.push(`<circle cx="${nx}" cy="${midY}" r="${row.node.id === bestId ? 5.5 : 4.5}" fill="${fill}" stroke="var(--et-card)" stroke-width="1.5"/>`);
    }

    return `<span class="rail-spacer" style="width:${W}px"></span><svg class="rail-cell-svg" width="${W}" height="100%">${parts.join('')}</svg>`;
  }

  function renderTreeTab() {
    if (!S.initialized || !S.tree) return;
    const nodesMap = S.tree.nodes || {};
    const ordered = Object.values(nodesMap).sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
    const matchFilter = (n) => {
      if (S.filter === 'win') return n.status === 'VALIDATED' || n.status === 'CONVERGED';
      if (S.filter === 'pruned') return n.status === 'PRUNED';
      return true;
    };
    // Keep full topology so rails/edges keep their context; non-matching nodes are dimmed.
    const filtered = ordered;
    const lineage = computeLineage(S.selectedId, nodesMap);

    const railBox = $('tree-rail-container');
    const mapWrap = $('tree-map-wrap');
    railBox.classList.toggle('hidden', S.treeMode !== 'rail');
    mapWrap.classList.toggle('hidden', S.treeMode !== 'map');

    if (S.treeMode === 'map') {
      renderTopDownMap(filtered, nodesMap, matchFilter, lineage);
      return;
    }

    const root = nodesMap[S.tree.root_id] || ordered[0];
    const hib = Boolean(S.tree.higher_is_better);
    const { rows, totalCols } = computeMetroLayout(filtered);

    const isMultiObj = (S.tree.objectives || []).length >= 2;
    railBox.innerHTML = rows.map((row) => {
      const n = row.node;
      const isSelected = S.selectedId === n.id;
      const isExpanded = S.expandedId === n.id;
      const isMulti = S.multiSet.has(n.id);
      const dim = !matchFilter(n);
      const vsBase = (!isMultiObj && n.id !== root?.id) ? calcDelta(n.primary_score, root?.primary_score, hib) : null;
      const hp = hypoParts(n);

      return `
        <div class="rail-row-group ${dim ? 'dimmed' : ''}" data-group-id="${esc(n.id)}">
          <div class="rail-row ${isSelected ? 'selected' : ''} ${isMulti ? 'multi-picked' : ''}" data-row-id="${esc(n.id)}">
            <div class="rail-gutter">
              ${renderRailCellSVG(row, totalCols, S.tree.best_node_id)}
            </div>
            <div class="rail-main">
              <div class="rail-head">
                <span class="node-id">${esc(n.id)}</span>
                ${statusBadge(n, S.tree.best_node_id)}
                ${n.revisits_node_id ? `<span class="revisit-pill">${esc(t('revisitTag', n.revisits_node_id))}</span>` : ''}
                <span class="score-pill">${isMultiObj ? '' : fmtNum(n.primary_score)}</span>
                ${vsBase && !vsBase.same ? `<span class="delta-tag ${vsBase.improved ? 'good' : 'bad'}">${esc(vsBase.text)}</span>` : ''}
                <button type="button" class="multi-dot ${isMulti ? 'on' : ''}" data-multi-btn="${esc(n.id)}" title="${esc(t('multiBtnTitle'))}">${ICON_CHECK}</button>
              </div>
              <div class="rail-hypo">${esc(hp.title)}${hp.sub ? ` <span class="hypo-sub">· ${esc(hp.sub)}</span>` : ''}</div>
              ${renderMultiMetricChips(n)}
            </div>
          </div>
          ${isExpanded ? `
            <div class="detail-wrap">
              <div class="rail-gutter">${renderRailContinuationSVG(row, totalCols)}</div>
              <div class="node-detail">${renderNodeSummaryInner(n)}</div>
            </div>` : ''}
        </div>
      `;
    }).join('');

    railBox.querySelectorAll('[data-row-id]').forEach((rowEl) => {
      const id = rowEl.dataset.rowId;
      rowEl.addEventListener('click', (ev) => {
        if (ev.metaKey || ev.ctrlKey || ev.shiftKey) {
          toggleMultiSelect(id);
          return;
        }
        selectNode(id, { toggleExpand: true });
      });
    });

    railBox.querySelectorAll('[data-multi-btn]').forEach((btn) => {
      btn.addEventListener('click', (ev) => {
        ev.stopPropagation();
        toggleMultiSelect(btn.dataset.multiBtn);
      });
    });

    bindSummaryFileJumps(railBox);
  }

  // ---------- Tab 1B: Left-to-Right Compact Centered Tree Canvas ----------
  function renderTopDownMap(nodes, nodesMap, matchFilter = () => true, lineage = new Set()) {
    const depthMap = computeDepthMap(nodesMap);
    const cols = {};
    let maxColRows = 1;
    for (const n of nodes) {
      const d = depthMap[n.id] || 0;
      if (!cols[d]) cols[d] = [];
      cols[d].push(n);
      maxColRows = Math.max(maxColRows, cols[d].length);
    }

    const CARD_W = 204, CARD_H = 78, GAP_X = 36, GAP_Y = 12;
    const sortedDepths = Object.keys(cols).map(Number).sort((a, b) => a - b);
    const coords = {};
    const colIndexOfDepth = {};
    const colCards = [];
    const totalH = Math.max(196, maxColRows * CARD_H + (maxColRows - 1) * GAP_Y + 36);
    const totalW = Math.max(320, sortedDepths.length * CARD_W + (sortedDepths.length - 1) * GAP_X + 32);
    const colX = (ci) => 16 + ci * (CARD_W + GAP_X);

    // Order each column by parent barycenter to minimize crossings, then center vertically.
    sortedDepths.forEach((d, colIdx) => {
      colIndexOfDepth[d] = colIdx;
      const list = cols[d];
      if (colIdx > 0) {
        const avgY = (node) => {
          const py = parentsOf(node).map((p) => coords[p]?.y).filter((v) => v !== undefined);
          return py.length ? py.reduce((s, v) => s + v, 0) / py.length : 0;
        };
        list.sort((a, b) => {
          const diff = avgY(a) - avgY(b);
          return Math.abs(diff) > 1 ? diff : (new Date(a.created_at) - new Date(b.created_at));
        });
      }
      const colH = list.length * CARD_H + (list.length - 1) * GAP_Y;
      const startY = (totalH - colH) / 2;
      colCards[colIdx] = [];
      list.forEach((n, rowIdx) => {
        const c = { x: colX(colIdx), y: startY + rowIdx * (CARD_H + GAP_Y), w: CARD_W, h: CARD_H };
        coords[n.id] = c;
        colCards[colIdx].push(c);
      });
    });

    const pane = $('tree-map-container');
    const doFit = S.mapFitPending && pane && pane.clientWidth > 0;
    if (doFit) {
      const fitX = (pane.clientWidth - 16) / totalW;
      S.mapZoom = Math.max(0.80, Math.min(1, fitX));
    }

    const zoomLayer = $('map-zoom-layer');
    const stage = $('map-stage');
    const svg = $('map-svg');
    const dom = $('map-nodes');

    stage.style.width = `${totalW}px`;
    stage.style.height = `${totalH}px`;
    zoomLayer.style.width = `${totalW * S.mapZoom}px`;
    zoomLayer.style.height = `${totalH * S.mapZoom}px`;
    stage.style.transform = `scale(${S.mapZoom})`;
    stage.style.transformOrigin = '0 0';
    $('zoom-reset').textContent = `${Math.round(S.mapZoom * 100)}%`;
    stage.classList.toggle('compact', S.mapZoom < 0.55);

    svg.setAttribute('width', String(totalW));
    svg.setAttribute('height', String(totalH));

    // Free horizontal channels in a column (above, between and below its cards).
    const freeChannels = (ci) => {
      const cards = colCards[ci] || [];
      if (!cards.length) return [totalH / 2];
      const ys = [Math.max(6, cards[0].y - GAP_Y / 2)];
      for (let k = 0; k < cards.length - 1; k++) ys.push((cards[k].y + cards[k].h + cards[k + 1].y) / 2);
      const last = cards[cards.length - 1];
      ys.push(Math.min(totalH - 6, last.y + last.h + GAP_Y / 2));
      return ys;
    };

    // Edges spanning >1 column are routed through free channels so they never cross card text.
    const edges = [];
    const hilite = lineage.size > 0;
    for (const n of nodes) {
      const to = coords[n.id];
      if (!to) continue;
      for (const pid of parentsOf(n)) {
        const from = coords[pid];
        if (!from) continue;
        const fromCol = colIndexOfDepth[depthMap[pid] || 0];
        const toCol = colIndexOfDepth[depthMap[n.id] || 0];
        const start = { x: from.x + from.w, y: from.y + from.h / 2 };
        const end = { x: to.x, y: to.y + to.h / 2 };
        const pts = [start];
        for (let ci = fromCol + 1; ci < toCol; ci++) {
          const t = (ci - fromCol) / (toCol - fromCol);
          const ideal = start.y + (end.y - start.y) * t;
          const fy = freeChannels(ci).reduce((b, y) => (Math.abs(y - ideal) < Math.abs(b - ideal) ? y : b));
          pts.push({ x: colX(ci) - GAP_X / 2, y: fy }, { x: colX(ci) + CARD_W + GAP_X / 2, y: fy });
        }
        pts.push(end);
        let d = `M ${pts[0].x} ${pts[0].y}`;
        for (let k = 1; k < pts.length; k++) {
          const p = pts[k - 1], q = pts[k];
          if (Math.abs(p.y - q.y) < 0.5) {
            d += ` L ${q.x} ${q.y}`;
          } else {
            const mx = (p.x + q.x) / 2;
            d += ` C ${mx} ${p.y}, ${mx} ${q.y}, ${q.x} ${q.y}`;
          }
        }
        const onLineage = !hilite || (lineage.has(pid) && lineage.has(n.id));
        const color = n.status === 'PRUNED' ? (isViolation(n) ? 'var(--et-prune)' : 'var(--et-fg-faint)')
          : (n.status === 'CONVERGED' ? 'var(--et-conv)' : (hilite && onLineage ? 'var(--et-fg-2)' : 'var(--et-fg-faint)'));
        const dash = n.status === 'PRUNED' ? 'stroke-dasharray="4 3"' : '';
        const faded = !matchFilter(n) || !onLineage;
        edges.push(`<path d="${d}" fill="none" stroke="${color}" stroke-width="${hilite && onLineage ? 2.4 : 1.6}" stroke-opacity="${faded ? 0.2 : 0.95}" ${dash}/>`);
      }
    }
    svg.innerHTML = edges.join('');

    const objsM = S.tree.objectives || [];
    const isMultiM = objsM.length >= 2;
    dom.innerHTML = nodes.map((n) => {
      const c = coords[n.id];
      const isSelected = S.selectedId === n.id;
      const isMulti = S.multiSet.has(n.id);
      const faded = !matchFilter(n) || (lineage.size > 0 && !lineage.has(n.id));
      const hp = hypoParts(n);
      const metricLine = isMultiM && n.metrics
        ? objsM.slice(0, 2).map((o) => `${fmtNum(n.metrics[o.name])}${o.higher_is_better ? '↑' : '↓'}`).join(' · ')
          + (objsM.length > 2 ? ` +${objsM.length - 2}` : '')
        : fmtNum(n.primary_score);
      const metricTitle = isMultiM && n.metrics
        ? objsM.map((o) => `${o.name}=${fmtNum(n.metrics[o.name])}`).join(', ')
        : `${S.tree.metric_name || 'score'}=${fmtNum(n.primary_score)}`;
      const ph = activePhase(n);
      const phaseCls = ph ? `phase-${ph}` : '';
      return `
        <div class="map-card ${esc(n.status)} ${phaseCls} ${isViolation(n) ? 'violation' : ''} ${isSelected ? 'selected' : ''} ${isMulti ? 'multi-picked' : ''} ${faded ? 'faded' : ''}" data-map-id="${esc(n.id)}" style="left:${c.x}px;top:${c.y}px;">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:4px;">
            <strong style="font-family:var(--et-mono);font-size:11px;">${esc(n.id)}</strong>
            ${statusBadge(n, S.tree.best_node_id)}
          </div>
          <div class="map-hypo">${esc(hp.title)}${hp.sub ? ` <span class="hypo-sub">${esc(hp.sub)}</span>` : ''}</div>
          <div class="map-metric" title="${esc(metricTitle)}">
            <span>${esc(metricLine)}</span>
            <span>${esc((n.git_commit || '').slice(0, 7))}</span>
          </div>
        </div>
      `;
    }).join('');

    if (doFit) {
      S.mapFitPending = false;
      const sel = coords[S.selectedId];
      if (sel) {
        pane.scrollLeft = Math.max(0, (sel.x + sel.w / 2) * S.mapZoom - pane.clientWidth / 2);
        pane.scrollTop = Math.max(0, (sel.y + sel.h / 2) * S.mapZoom - pane.clientHeight / 2);
      }
    }

    dom.querySelectorAll('[data-map-id]').forEach((card) => {
      card.addEventListener('click', (ev) => {
        if (card.dataset.dragMoved === '1') return;
        const id = card.dataset.mapId;
        if (ev.metaKey || ev.ctrlKey || ev.shiftKey) {
          toggleMultiSelect(id);
          return;
        }
        selectNode(id, { toggleExpand: false });
      });
    });

    const insp = $('map-inspector');
    const selNode = nodesMap[S.selectedId] || nodes[0];
    if (insp && selNode) {
      insp.innerHTML = `
        <div class="card" style="margin-bottom:0;">
          <div class="card-head" style="margin-bottom:4px;">
            <div style="display:flex;align-items:center;gap:6px;flex-wrap:wrap;">
              <strong style="font-family:var(--et-mono);font-size:12px;">${esc(selNode.id)}</strong>
              ${statusBadge(selNode, S.tree.best_node_id)}
              ${selNode.revisits_node_id ? `<span class="revisit-pill">${esc(t('revisitTag', selNode.revisits_node_id))}</span>` : ''}
            </div>
            <span style="font-family:var(--et-mono);font-size:11px;">${fmtNum(selNode.primary_score)}</span>
          </div>
          <div style="font-size:12px;color:var(--et-fg);margin-bottom:6px;">${esc(hypoParts(selNode).title)}${hypoParts(selNode).sub ? ` <span class="hypo-sub">· ${esc(hypoParts(selNode).sub)}</span>` : ''}</div>
          ${renderMultiMetricChips(selNode)}
          <div style="margin-top:6px;display:flex;flex-direction:column;gap:6px;">
            ${renderNodeSummaryInner(selNode)}
          </div>
        </div>
      `;
      bindSummaryFileJumps(insp);
    }
  }

  // Grab-to-pan & wheel-zoom wiring on Map Pane
  function setupMapPanZoom() {
    const pane = $('tree-map-container');
    if (!pane) return;
    let isDown = false, startX = 0, startY = 0, sLeft = 0, sTop = 0, moved = false;

    pane.addEventListener('mousedown', (e) => {
      if (e.button !== 0) return;
      isDown = true;
      moved = false;
      startX = e.clientX;
      startY = e.clientY;
      sLeft = pane.scrollLeft;
      sTop = pane.scrollTop;
      pane.classList.add('dragging');
      pane.querySelectorAll('[data-map-id]').forEach((c) => { c.dataset.dragMoved = '0'; });
    });

    window.addEventListener('mousemove', (e) => {
      if (!isDown) return;
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;
      if (Math.abs(dx) > 4 || Math.abs(dy) > 4) moved = true;
      if (moved) {
        pane.scrollLeft = sLeft - dx;
        pane.scrollTop = sTop - dy;
      }
    });

    window.addEventListener('mouseup', () => {
      if (!isDown) return;
      isDown = false;
      pane.classList.remove('dragging');
      if (moved) {
        pane.querySelectorAll('[data-map-id]').forEach((c) => { c.dataset.dragMoved = '1'; });
        setTimeout(() => {
          pane.querySelectorAll('[data-map-id]').forEach((c) => { c.dataset.dragMoved = '0'; });
        }, 50);
      }
    });

    pane.addEventListener('wheel', (e) => {
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        const factor = e.deltaY < 0 ? 1.1 : 0.9;
        S.mapZoom = Math.min(1.6, Math.max(0.55, S.mapZoom * factor));
        renderTreeTab();
      }
    }, { passive: false });

    $('zoom-in')?.addEventListener('click', () => { S.mapZoom = Math.min(1.6, S.mapZoom + 0.15); renderTreeTab(); });
    $('zoom-out')?.addEventListener('click', () => { S.mapZoom = Math.max(0.55, S.mapZoom - 0.15); renderTreeTab(); });
    $('zoom-reset')?.addEventListener('click', () => { S.mapZoom = 1; renderTreeTab(); });
  }

  // ---------- Tab 2: Changes (Synchronized with Global S.selectedId) ----------
  function renderDiffSquares(add, del) {
    const tot = add + del;
    if (tot === 0) return '';
    const addBlocks = Math.round((add / tot) * 5);
    const delBlocks = Math.min(5 - addBlocks, Math.ceil((del / tot) * 5));
    const cells = [];
    for (let i = 0; i < 5; i++) {
      if (i < addBlocks) cells.push('<span class="sq add"></span>');
      else if (i < addBlocks + delBlocks) cells.push('<span class="sq del"></span>');
      else cells.push('<span class="sq"></span>');
    }
    return `<span class="diff-squares">${cells.join('')}</span>`;
  }

  function renderChangesTab() {
    if (!S.initialized || !S.tree) return;
    const nodesMap = S.tree.nodes || {};
    const ordered = Object.values(nodesMap).sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
    const curId = S.selectedId || S.tree.best_node_id || S.tree.root_id;
    const curNode = nodesMap[curId];
    const cacheKey = diffCacheKey(curId, S.diffFromId);
    const cached = getCachedDiff(curId, S.diffFromId);
    const isLoading = !cached && S.diffLoadingKey === cacheKey;
    const d = cached || { files: [], compareTargets: [] };
    const files = d.files || [];
    const totalAdd = files.reduce((acc, f) => acc + f.additions, 0);
    const totalDel = files.reduce((acc, f) => acc + f.deletions, 0);
    setChangesCount(files);

    // Auto-collapse extra files when diff is large so opening the tab stays instant.
    if (cached && files.length > 0 && cacheKey !== S.diffAutoCollapsedKey) {
      S.diffAutoCollapsedKey = cacheKey;
      if (files.length > 3 || (totalAdd + totalDel) > 300) {
        const keepIdx = S.pendingScrollFileIdx !== null ? S.pendingScrollFileIdx : 0;
        S.collapsedFiles.clear();
        files.forEach((f, idx) => {
          if (idx !== keepIdx && idx !== 0) S.collapsedFiles.add(f.path);
        });
      }
    }
    if (S.pendingScrollFileIdx !== null && files[S.pendingScrollFileIdx]) {
      S.collapsedFiles.delete(files[S.pendingScrollFileIdx].path);
    }

    const targets = d.compareTargets || [];
    const activeBase = d.fromId || S.diffFromId || '';
    const targetIds = new Set(targets.map((ti) => ti.id));
    const otherNodes = ordered.filter((n) => n.id !== curId && !targetIds.has(n.id));

    const objsC = S.tree.objectives || [];
    const curScoreStr = curNode
      ? ((objsC.length >= 2 && curNode.metrics)
        ? objsC.slice(0, 3).map((o) => (typeof curNode.metrics[o.name] === 'number' ? `${o.name} ${fmtNum(curNode.metrics[o.name])}` : '')).filter(Boolean).join(' · ')
        : (typeof curNode.primary_score === 'number' ? `${S.tree.metric_name || 'score'} ${fmtNum(curNode.primary_score)}` : ''))
      : '';

    const allCollapsed = files.length > 1 && files.every((f) => S.collapsedFiles.has(f.path));

    $('diff-header').innerHTML = `
      <div class="diff-bar-row">
        <span class="select-filter diff-node-pick">
          <select id="diff-node-sel" aria-label="${esc(t('diffSelectNode'))}">
            ${ordered.map((n) => `<option value="${esc(n.id)}" ${n.id === curId ? 'selected' : ''}>${esc(`${n.id} · ${hypoParts(n).title.slice(0, 28)}`)}</option>`).join('')}
          </select>
        </span>
        ${curNode ? statusBadge(curNode, S.tree.best_node_id) : ''}
        <span class="diffstat">
          <span class="add">+${totalAdd}</span>
          <span class="del">−${totalDel}</span>
          ${renderDiffSquares(totalAdd, totalDel)}
        </span>
      </div>
      ${curNode ? `
        <div class="diff-node-ctx" title="${esc(hypoParts(curNode).title)}">
          <span class="diff-ctx-hypo">${esc(hypoParts(curNode).title)}</span>
          ${curScoreStr ? `<span class="diff-ctx-score">${esc(curScoreStr)}</span>` : ''}
        </div>` : ''}
      <div class="diff-bar-row diff-ctrl-row">
        <span class="diff-vs">${esc(t('diffVs'))}</span>
        ${targets.length ? `
          <div class="segmented xs" role="group" aria-label="${esc(t('diffVs'))}">
            ${targets.map((tInfo) => `<button type="button" data-base="${esc(tInfo.id)}" class="${activeBase === tInfo.id ? 'on' : ''}">${esc(tInfo.role === 'baseline' ? t('diffRoleBase') : t('diffRoleParent'))} ${esc(tInfo.id)}</button>`).join('')}
          </div>` : (d.fromLabel ? `<span class="diff-vs">${esc(d.fromLabel)}</span>` : '')}
        ${otherNodes.length ? `
          <select id="diff-base-more" class="diff-base-select" aria-label="${esc(t('diffMoreBase'))}">
            <option value="" ${!otherNodes.some((n) => n.id === activeBase) ? 'selected' : ''}>${esc(t('diffMoreBase'))}</option>
            ${otherNodes.map((n) => `<option value="${esc(n.id)}" ${activeBase === n.id ? 'selected' : ''}>${esc(n.id)}</option>`).join('')}
          </select>` : ''}
        <span class="spacer"></span>
        <button type="button" id="btn-changes-only" class="ctrl-pill ${S.diffChangesOnly ? 'on' : ''}">${esc(t('diffChangesOnly'))}</button>
        ${files.length > 1 ? `<button type="button" id="btn-toggle-all-files" class="ctrl-pill">${esc(allCollapsed ? t('diffExpandAll') : t('diffCollapseAll'))}</button>` : ''}
        <button type="button" id="btn-refresh-diff" class="icon-btn xs" title="${esc(t('diffRefreshTitle'))}" aria-label="${esc(t('diffRefreshTitle'))}">
          <svg class="icon" viewBox="0 0 16 16"><path d="M13.5 8a5.5 5.5 0 1 1-1.35-3.6"/><path d="M13.5 2.5v2.3h-2.3"/></svg>
        </button>
      </div>
    `;

    $('diff-node-sel')?.addEventListener('change', (e) => selectNode(e.target.value));
    $('diff-header').querySelectorAll('[data-base]').forEach((b) => {
      b.addEventListener('click', async () => {
        S.diffFromId = b.dataset.base;
        await ensureNodeDiffCached(curId, S.diffFromId);
        renderChangesTab();
      });
    });
    $('diff-base-more')?.addEventListener('change', async (e) => {
      const val = e.target.value;
      if (!val) return;
      S.diffFromId = val;
      await ensureNodeDiffCached(curId, S.diffFromId);
      renderChangesTab();
    });
    $('btn-changes-only')?.addEventListener('click', () => {
      S.diffChangesOnly = !S.diffChangesOnly;
      renderChangesTab();
    });
    $('btn-toggle-all-files')?.addEventListener('click', () => {
      if (allCollapsed) {
        S.collapsedFiles.clear();
      } else {
        files.forEach((f) => S.collapsedFiles.add(f.path));
      }
      renderChangesTab();
    });
    $('btn-refresh-diff')?.addEventListener('click', async () => {
      await ensureNodeDiffCached(curId, S.diffFromId, { force: true });
      renderChangesTab();
    });

    const pillsEl = $('diff-file-pills');
    const filesEl = $('diff-files-container');

    if (isLoading) {
      pillsEl.innerHTML = '';
      filesEl.innerHTML = `<div class="empty-box diff-loading"><span class="diff-spinner" aria-hidden="true"></span><span>${esc(t('diffLoading'))}</span></div>`;
      return;
    }

    if (d.prunedLegacyNoCommit) {
      pillsEl.innerHTML = '';
      filesEl.innerHTML = `<div class="empty-box"><div class="empty-title">${esc(t('diffLegacyPruned'))}</div><p style="margin:4px 0 0;">${esc(d.pruneDigest)}</p></div>`;
      return;
    }

    if (d.offline) {
      pillsEl.innerHTML = '';
      filesEl.innerHTML = `<div class="empty-box"><div class="empty-title">${esc(t('diffOfflineTitle'))}</div><p style="margin:4px 0 0;">${esc(t('diffOfflineDesc'))}</p></div>`;
      return;
    }

    const prunedBannerHtml = d.isPruned ? `
      <div class="pruned-banner ${isViolation(S.tree.nodes[curId]) ? 'violation' : ''}">
        <strong>${esc(t('diffPrunedSaved'))}</strong> · ${esc(humanizeAction(d.pruneDigest || t('diffPrunedDefault')))}
      </div>
    ` : '';

    if (!files.length) {
      pillsEl.innerHTML = '';
      filesEl.innerHTML = prunedBannerHtml + `<div class="empty-box">${esc(t('diffNoFiles'))}</div>`;
      return;
    }

    if (files.length > 1) {
      pillsEl.innerHTML = `
        <div class="file-summary-card ${S.diffSummaryOpen ? '' : 'collapsed'}">
          <button type="button" class="file-sum-head" id="btn-toggle-file-summary">
            <span class="diff-chev">▾</span>
            <span>${esc(t('diffFilesSummary', files.length))}</span>
            <span class="spacer"></span>
            <span class="diffstat"><span class="add">+${totalAdd}</span><span class="del">−${totalDel}</span></span>
          </button>
          ${S.diffSummaryOpen ? `
            <div class="file-sum-list">
              ${files.map((f, idx) => {
                const slash = f.path.lastIndexOf('/');
                const dir = slash >= 0 ? f.path.slice(0, slash + 1) : '';
                const name = slash >= 0 ? f.path.slice(slash + 1) : f.path;
                return `<button type="button" class="file-sum-row" data-jump-file-idx="${idx}" title="${esc(f.path)}">
                  <span class="file-sum-name">${esc(name)}</span>
                  ${dir ? `<span class="file-sum-dir">${esc(dir)}</span>` : ''}
                  <span class="spacer"></span>
                  <span class="diffstat"><span class="add">+${f.additions}</span><span class="del">−${f.deletions}</span></span>
                </button>`;
              }).join('')}
            </div>` : ''}
        </div>`;

      $('btn-toggle-file-summary')?.addEventListener('click', () => {
        S.diffSummaryOpen = !S.diffSummaryOpen;
        renderChangesTab();
      });
      pillsEl.querySelectorAll('[data-jump-file-idx]').forEach((b) => {
        b.addEventListener('click', () => {
          const idx = Number(b.dataset.jumpFileIdx);
          const f = files[idx];
          if (f && S.collapsedFiles.has(f.path)) {
            S.collapsedFiles.delete(f.path);
            S.pendingScrollFileIdx = idx;
            renderChangesTab();
            return;
          }
          document.getElementById(`diff-f-${idx}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
        });
      });
    } else {
      pillsEl.innerHTML = '';
    }

    filesEl.innerHTML = prunedBannerHtml + files.map((f, idx) => {
      const fid = `diff-f-${idx}`;
      const isCol = S.collapsedFiles.has(f.path);
      let rowsHtml = '';
      if (!isCol) {
        rowsHtml = f.hunks.map((h) => {
          const atIdx = h.header.indexOf(' @@');
          const rangePart = atIdx >= 0 ? h.header.slice(0, atIdx + 3) : h.header;
          const ctxText = h.context || (atIdx >= 0 ? h.header.slice(atIdx + 3).trim() : '');
          const hunkRow = `<tr class="diff-hunk"><td colspan="4"><span class="hunk-range">${esc(rangePart)}</span>${ctxText ? `<span class="hunk-ctx">${esc(ctxText)}</span>` : ''}</td></tr>`;

          const annotated = annotateWordDiff(h.lines);
          let codeRows = '';
          if (S.diffChangesOnly) {
            const parts = [];
            let skippedCtx = false;
            let hasChanged = false;
            for (const ln of annotated) {
              if (ln.type === 'ctx') {
                if (hasChanged) skippedCtx = true;
                continue;
              }
              if (skippedCtx) {
                parts.push('<tr class="diff-elide"><td colspan="4">···</td></tr>');
                skippedCtx = false;
              }
              hasChanged = true;
              if (ln.type === 'meta') {
                parts.push(`<tr><td class="ln"></td><td class="ln"></td><td class="sign">\\</td><td class="code-cell" style="color:var(--et-fg-muted)">${esc(ln.content)}</td></tr>`);
              } else {
                const cls = ln.type === 'add' ? 'line-add' : 'line-del';
                const sign = ln.type === 'add' ? '+' : '-';
                parts.push(`<tr class="${cls}">
                  <td class="ln">${ln.oldNo ?? ''}</td>
                  <td class="ln">${ln.newNo ?? ''}</td>
                  <td class="sign">${sign}</td>
                  <td class="code-cell">${ln.html ?? esc(ln.content)}</td>
                </tr>`);
              }
            }
            codeRows = parts.length ? parts.join('') : `<tr class="diff-elide"><td colspan="4">${esc(t('diffNoChangesHunk'))}</td></tr>`;
          } else {
            codeRows = annotated.map((ln) => {
              if (ln.type === 'meta') {
                return `<tr><td class="ln"></td><td class="ln"></td><td class="sign">\\</td><td class="code-cell" style="color:var(--et-fg-muted)">${esc(ln.content)}</td></tr>`;
              }
              const cls = ln.type === 'add' ? 'line-add' : (ln.type === 'del' ? 'line-del' : '');
              const sign = ln.type === 'add' ? '+' : (ln.type === 'del' ? '-' : ' ');
              return `<tr class="${cls}">
                <td class="ln">${ln.oldNo ?? ''}</td>
                <td class="ln">${ln.newNo ?? ''}</td>
                <td class="sign">${sign}</td>
                <td class="code-cell">${ln.html ?? esc(ln.content)}</td>
              </tr>`;
            }).join('');
          }
          return hunkRow + codeRows;
        }).join('');
      }

      return `
        <div id="${fid}" class="diff-file ${isCol ? 'collapsed' : ''}">
          <div class="diff-file-head" data-toggle-file="${esc(f.path)}">
            <span class="diff-chev">▾</span>
            <span class="diff-path" title="${esc(f.path)}">${esc(f.path)}</span>
            <span class="diffstat">
              <span class="add">+${f.additions}</span>
              <span class="del">−${f.deletions}</span>
            </span>
          </div>
          <div class="diff-file-body">
            ${isCol ? '' : `<table class="diff-table"><tbody>${rowsHtml}</tbody></table>`}
          </div>
        </div>
      `;
    }).join('');

    filesEl.querySelectorAll('[data-toggle-file]').forEach((hdr) => {
      hdr.addEventListener('click', () => {
        const p = hdr.dataset.toggleFile;
        if (S.collapsedFiles.has(p)) S.collapsedFiles.delete(p);
        else S.collapsedFiles.add(p);
        renderChangesTab();
      });
    });

    if (S.pendingScrollFileIdx !== null) {
      const idx = S.pendingScrollFileIdx;
      S.pendingScrollFileIdx = null;
      requestAnimationFrame(() => {
        document.getElementById(`diff-f-${idx}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    }
  }

  function computeMadConfidence(ordered, root, best) {
    if (typeof root?.primary_score !== 'number' || typeof best?.primary_score !== 'number') return null;
    const vals = ordered
      .filter((n) => typeof n.primary_score === 'number' && Number.isFinite(n.primary_score) && !isViolation(n))
      .map((n) => n.primary_score);
    if (vals.length < 3) return null;
    const medianOf = (arr) => {
      const s = [...arr].sort((a, b) => a - b);
      const mid = Math.floor(s.length / 2);
      return s.length % 2 ? s[mid] : 0.5 * (s[mid - 1] + s[mid]);
    };
    const med = medianOf(vals);
    const mad = medianOf(vals.map((v) => Math.abs(v - med)));
    if (mad <= 1e-12) return null;
    const ratio = Math.abs(best.primary_score - root.primary_score) / mad;
    const cls = ratio >= 2.0 ? '' : (ratio >= 1.0 ? 'mid' : 'low');
    return { ratio, cls };
  }

  // ---------- Tab 3: Overview (Pareto Frontier / Trajectory & Step Stream) ----------
  function renderOverviewTab() {
    if (!S.initialized || !S.tree) return;
    const el = $('tab-overview');
    const nodesMap = S.tree.nodes || {};
    const ordered = Object.values(nodesMap).sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
    const root = nodesMap[S.tree.root_id] || ordered[0];
    const best = nodesMap[S.tree.best_node_id] || root;
    const hib = Boolean(S.tree.higher_is_better);
    const depthMap = computeDepthMap(nodesMap);
    const maxGen = Math.max(0, ...Object.values(depthMap));

    const counts = { pass: 0, conv: 0, run: 0, prune: 0 };
    let totalVerifiedSteps = 0;
    const stepEvents = [];

    for (const n of ordered) {
      if (n.status === 'CONVERGED') counts.conv++;
      else if (n.status === 'VALIDATED') counts.pass++;
      else if (n.status === 'PRUNED') counts.prune++;
      else counts.run++;

      for (const st of (n.steps || [])) {
        if (st.verified) totalVerifiedSteps++;
        stepEvents.push({
          nodeId: n.id,
          status: n.status,
          index: st.index,
          action: st.action,
          ts: st.timestamp || n.updated_at,
        });
      }
    }
    stepEvents.sort((a, b) => new Date(b.ts) - new Date(a.ts));

    const total = Math.max(1, ordered.length);
    const delta = calcDelta(best?.primary_score, root?.primary_score, hib);
    const objs = S.tree.objectives || [];
    const cons = S.tree.constraints || [];
    const pFrontIds = S.tree.pareto_front_ids || [];
    const isMulti = objs.length >= 2;

    const scored = ordered.filter((n) => typeof n.primary_score === 'number');
    const paretoPlotHtml = isMulti ? buildParetoPlot(ordered, objs, pFrontIds, root) : '';
    const chartHtml = (!isMulti && scored.length >= 2) ? buildSparkChart(scored, root, best, hib) : '';
    const conf = computeMadConfidence(ordered, root, best);

    const experiments = ordered.filter((n) => n.id !== root?.id);
    const succ = experiments.filter((n) => n.status === 'VALIDATED' || n.status === 'CONVERGED').length;
    const violations = experiments.filter((n) => isViolation(n)).length;
    const summary = [
      t('ovExp', experiments.length),
      t('ovSucc', succ),
      counts.prune ? t('ovPrune', counts.prune, violations) : '',
      counts.conv ? t('ovConv', counts.conv) : '',
      counts.run ? t('ovRun', counts.run) : '',
    ].filter(Boolean).join(' · ');
    const expTotal = Math.max(1, experiments.length);
    const succOnly = Math.max(0, succ - counts.conv);

    const frontRow = isMulti && pFrontIds.length ? `
      <div class="kv-row"><span class="kv-key">${esc(t('ovLead'))}</span><span class="kv-val">${pFrontIds.map((id) => `<button type="button" class="node-link ${S.selectedId === id ? 'on' : ''}" data-jump-node="${esc(id)}">${esc(id)}</button>`).join('<span class="kv-sep">·</span>')}${conf ? `<span class="kv-sep">·</span><span class="conf-pill ${conf.cls}" title="${esc(t('ovConfTitle'))}">${esc(t('ovConf', conf.ratio))}</span>` : ''}</span></div>` : '';
    const consRow = cons.length ? `
      <div class="kv-row"><span class="kv-key">${esc(t('ovCons'))}</span><span class="kv-val mono">${esc(cons.map((c) => `${c.metric} ${opSym(c.op)} ${fmtNum(c.threshold)}`).join(' · '))}</span></div>` : '';
    const heroSingle = !isMulti ? `
      <div class="hero-kpi">
        <span class="hero-best">${fmtNum(best?.primary_score)}</span>
        <span class="hero-unit">${esc(S.tree.metric_name || '')}</span>
        ${delta && !delta.same ? `<span class="hero-delta">${esc(t('ovVsBase', delta.text))}</span>` : ''}
        ${conf ? `<span class="conf-pill ${conf.cls}" title="${esc(t('ovConfTitle'))}">${esc(t('ovConf', conf.ratio))}</span>` : ''}
      </div>
      ${best ? `<div class="kv-row"><span class="kv-key">${esc(t('ovBest'))}</span><span class="kv-val"><button type="button" class="node-link" data-jump-node="${esc(best.id)}">${esc(best.id)}</button> ${esc(hypoParts(best).title)}</span></div>` : ''}
    ` : '';
    void maxGen; void totalVerifiedSteps; void total;

    el.innerHTML = `
      <div class="card">
        <div class="card-head">
          <span class="card-title">${esc(t('ovProgress'))}</span>
          <span class="card-note">${esc(summary)}</span>
        </div>
        <div class="progress-bar">
          <div class="prog-seg pass" style="width:${(succOnly / expTotal) * 100}%"></div>
          <div class="prog-seg conv" style="width:${(counts.conv / expTotal) * 100}%"></div>
          <div class="prog-seg run" style="width:${(counts.run / expTotal) * 100}%"></div>
          <div class="prog-seg prune" style="width:${(counts.prune / expTotal) * 100}%"></div>
        </div>
        ${heroSingle}${frontRow}${consRow}
      </div>

      ${paretoPlotHtml}
      ${chartHtml}

      <div class="card">
        <div class="card-head"><span class="card-title">${esc(t('ovFeed'))}</span></div>
        <div class="step-feed">
          ${stepEvents.length ? stepEvents.slice(0, 14).map((ev) => {
            const node = nodesMap[ev.nodeId];
            const dotCls = isViolation(node) ? 'prune' : (ev.status === 'PRUNED' ? 'muted' : (ev.status === 'CONVERGED' ? 'conv' : ''));
            return `
              <div class="step-item" data-jump-node="${esc(ev.nodeId)}">
                <span class="step-dot ${dotCls}"></span>
                <div class="step-body">
                  <div class="step-top">
                    <span class="step-node">${esc(ev.nodeId)}</span>
                    <span class="spacer"></span>
                    <span class="step-time">${esc(fmtTime(ev.ts))}</span>
                  </div>
                  <div class="step-msg">${esc(humanizeAction(ev.action))}</div>
                </div>
              </div>`;
          }).join('') : `<div class="empty-inline">${esc(t('ovNoFeed'))}</div>`}
        </div>
      </div>
    `;

    el.querySelectorAll('[data-pareto-view]').forEach((btn) => {
      btn.addEventListener('click', () => {
        S.paretoView = btn.dataset.paretoView === 'table' ? 'table' : 'plot';
        renderOverviewTab();
      });
    });

    el.querySelectorAll('[data-pareto-sort]').forEach((th) => {
      th.addEventListener('click', () => {
        const col = th.dataset.paretoSort || '';
        if (!col) {
          S.paretoSortCol = '';
          S.paretoSortDir = 'best';
        } else if (S.paretoSortCol === col) {
          if (S.paretoSortDir === 'best') {
            S.paretoSortDir = 'worst';
          } else {
            S.paretoSortCol = '';
            S.paretoSortDir = 'best';
          }
        } else {
          S.paretoSortCol = col;
          S.paretoSortDir = 'best';
        }
        renderOverviewTab();
      });
    });

    el.querySelectorAll('[data-pareto-axis]').forEach((sel) => {
      sel.addEventListener('change', () => {
        const nextIdx = Number(sel.value);
        if (sel.dataset.paretoAxis === 'y') {
          if (nextIdx === S.paretoXIdx) S.paretoXIdx = S.paretoYIdx;
          S.paretoYIdx = nextIdx;
        } else {
          if (nextIdx === S.paretoYIdx) S.paretoYIdx = S.paretoXIdx;
          S.paretoXIdx = nextIdx;
        }
        renderOverviewTab();
      });
    });

    el.querySelectorAll('[data-jump-node]').forEach((item) => {
      item.addEventListener('click', () => {
        $('chart-tip')?.classList.add('hidden');
        selectNode(item.dataset.jumpNode, { openTab: 'tree' });
      });
    });
    bindChartTips(el);
  }

  // ---------- Wire Static Controls ----------
  document.querySelectorAll('.tab[data-tab]').forEach((btn) => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });

  $('btn-follow').addEventListener('click', () => {
    S.follow = !S.follow;
    $('btn-follow').classList.toggle('on', S.follow);
    $('btn-follow').setAttribute('aria-pressed', String(S.follow));
    showToast(S.follow ? t('toastFollowOn') : t('toastFollowOff'));
  });

  $('btn-refresh').addEventListener('click', async () => {
    await syncBinding(false);
    await pollState(true);
    showToast(t('toastSynced'));
  });

  $('btn-tree-menu')?.addEventListener('click', (e) => {
    e.stopPropagation();
    setMenuOpen(!S.menuOpen);
  });

  $('tree-menu')?.addEventListener('click', (e) => e.stopPropagation());
  document.addEventListener('click', () => {
    if (S.menuOpen) setMenuOpen(false);
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && S.menuOpen) {
      setMenuOpen(false);
    }
  });

  $('menu-toggle-rebind')?.addEventListener('click', () => {
    S.rebindListOpen = !S.rebindListOpen;
    renderMenuRepoList();
  });
  $('menu-unbind')?.addEventListener('click', () => bindRepo(''));

  $('btn-offline-retry')?.addEventListener('click', async () => {
    await syncBinding(false);
    await pollState(true);
    if (S.selectedId) await ensureNodeDiffCached(S.selectedId, S.diffFromId);
    renderAll();
    showToast(S.offline ? t('toastStillOffline') : t('toastReconnected'));
  });

  $('btn-preview-bind')?.addEventListener('click', () => bindRepo(S.previewRepo));
  $('btn-preview-back')?.addEventListener('click', async () => {
    S.previewRepo = '';
    await syncBinding(false);
    await pollState(true);
  });

  $('btn-bind-existing')?.addEventListener('click', () => {
    const val = $('existing-repo-select')?.value || '';
    if (val) bindRepo(val);
  });

  $('mode-rail').addEventListener('click', () => {
    S.treeMode = 'rail';
    $('mode-rail').classList.add('on');
    $('mode-map').classList.remove('on');
    renderTreeTab();
  });

  $('mode-map').addEventListener('click', () => {
    S.treeMode = 'map';
    S.mapFitPending = true;
    $('mode-map').classList.add('on');
    $('mode-rail').classList.remove('on');
    renderTreeTab();
  });

  $('filter-select').addEventListener('change', (e) => {
    S.filter = e.target.value;
    renderTreeTab();
  });

  $('focus-clear').addEventListener('click', () => {
    S.multiSet.clear();
    syncFocusBackend();
    renderTreeTab();
  });

  setupMapPanZoom();
  applyStaticI18n();
  $('mode-rail').classList.toggle('on', S.treeMode === 'rail');
  $('mode-map').classList.toggle('on', S.treeMode === 'map');

  const headerEl = document.querySelector('.header');
  if (headerEl && window.ResizeObserver) {
    new ResizeObserver(() => {
      document.documentElement.style.setProperty('--header-h', `${headerEl.offsetHeight}px`);
    }).observe(headerEl);
  }

  // ---------- Startup & Polling ----------
  async function boot() {
    if (window.sidecar?.getWorkspaceUris) {
      try {
        S.workspaces = await window.sidecar.getWorkspaceUris();
      } catch {
        S.workspaces = [];
      }
      window.sidecar.onWorkspaceChange?.(async (uris) => {
        S.workspaces = uris || [];
        await syncBinding(false);
        await pollState(true);
      });
    }
    await syncBinding(false);
    await pollState(true);
    syncFocusBackend();

    setInterval(async () => {
      const switched = await syncBinding(true);
      await pollState(switched);
    }, 1800);
  }

  boot();
})();
