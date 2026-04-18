// Main App — biene-aligned
const { useState, useEffect, useMemo } = React;

const styleSheet = document.createElement('style');
styleSheet.textContent = `
  @keyframes pulse {
    0%,100% { opacity: 1; }
    50%     { opacity: .4; }
  }
  @keyframes fadeIn { from { opacity: 0 } to { opacity: 1 } }
  @keyframes popIn {
    from { opacity: 0; transform: translate(-50%,-48%) scale(0.98); }
    to   { opacity: 1; transform: translate(-50%,-50%) scale(1); }
  }
  @keyframes spin { to { transform: rotate(360deg) } }
  ::selection { background: var(--accent); color: var(--ink); }
  button { font-family: inherit; }
  input:focus, textarea:focus, select:focus { border-color: var(--ink) !important; }
`;
document.head.appendChild(styleSheet);

function App() {
  const [agents, setAgents] = useState(INITIAL_AGENTS);
  const [modalOpen, setModalOpen] = useState(false);
  const [settingsAgent, setSettingsAgent] = useState(null);
  const [deletingAgent, setDeletingAgent] = useState(null);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState('all');
  const [refreshing, setRefreshing] = useState(false);
  const [tweaks, setTweaks] = useState(TWEAK_DEFAULTS);
  const [tweaksOpen, setTweaksOpen] = useState(false);
  const [toast, setToast] = useState(null);

  useEffect(() => {
    document.documentElement.style.setProperty('--accent', ACCENT_MAP[tweaks.accent] || ACCENT_MAP.amber);
  }, [tweaks.accent]);

  useEffect(() => {
    const handler = (e) => {
      if (e.data?.type === '__activate_edit_mode')   setTweaksOpen(true);
      if (e.data?.type === '__deactivate_edit_mode') setTweaksOpen(false);
    };
    window.addEventListener('message', handler);
    window.parent.postMessage({ type: '__edit_mode_available' }, '*');
    return () => window.removeEventListener('message', handler);
  }, []);

  const setTweak = (k, v) => {
    setTweaks((t) => ({ ...t, [k]: v }));
    window.parent.postMessage({ type: '__edit_mode_set_keys', edits: { [k]: v } }, '*');
  };

  const showToast = (msg) => {
    setToast(msg);
    setTimeout(() => setToast(null), 2000);
  };

  const filtered = useMemo(() => {
    return agents.filter((a) => {
      if (filter !== 'all' && a.status !== filter) return false;
      if (!search) return true;
      const q = search.toLowerCase();
      return (a.name + a.id + a.work_dir + a.last_message).toLowerCase().includes(q);
    });
  }, [agents, filter, search]);

  const refresh = () => {
    setRefreshing(true);
    setTimeout(() => setRefreshing(false), 700);
  };

  const openAgent = (agent) => {
    // in real app this is desktop:openAgentWindow IPC (creates a new BrowserWindow).
    // for the prototype, open Chat.html in a new tab with the agent id.
    window.open(`Chat.html?id=${encodeURIComponent(agent.id)}`, '_blank');
    showToast(`已在新窗口打开 · ${agent.name}`);
  };

  const handleMenu = (action, agent) => {
    if (action === 'open')     openAgent(agent);
    if (action === 'settings') setSettingsAgent(agent);
    if (action === 'delete')   setDeletingAgent(agent);
  };

  const createAgent = (input) => {
    const id = 'sess_' + Math.random().toString(16).slice(2, 14);
    setAgents((prev) => [...prev, {
      id, name: input.name, work_dir: input.work_dir,
      status: 'running',
      profile: input.profile, permissions: input.permissions,
      active_skill: null, pending_permission: null,
      last_message: '已启动',
      updated_at: '刚刚',
    }]);
    setModalOpen(false);
    showToast(`已创建 · ${input.name}`);
  };

  const saveSettings = (id, patch) => {
    setAgents((prev) => prev.map((a) => a.id === id ? { ...a, ...patch } : a));
    setSettingsAgent(null);
    showToast('已保存');
  };

  const confirmDelete = () => {
    if (!deletingAgent) return;
    setAgents((prev) => prev.filter((a) => a.id !== deletingAgent.id));
    setDeletingAgent(null);
  };

  const counts = useMemo(() => ({
    all:      agents.length,
    running:  agents.filter(a => a.status === 'running').length,
    approval: agents.filter(a => a.status === 'approval').length,
    idle:     agents.filter(a => a.status === 'idle').length,
    error:    agents.filter(a => a.status === 'error').length,
  }), [agents]);

  return (
    <div style={{
      height: '100%', display: 'flex', flexDirection: 'column',
      background: 'var(--bg)', position: 'relative', overflow: 'hidden',
    }}>
      <style>{!tweaks.paperGrain ? 'body::before { display: none; }' : ''}</style>

      {tweaks.showChrome && (
        <WindowChrome kernel={true} agentCount={agents.length}
          onOpenSettings={() => setTweaksOpen(v => !v)} />
      )}

      <div style={{
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '14px 20px',
        borderBottom: '1px solid var(--rule-soft)',
        flex: '0 0 auto',
      }}>
        <div style={{
          fontFamily: 'var(--sans)', fontSize: 20, fontWeight: 700,
          letterSpacing: '-0.02em', color: 'var(--ink)',
          whiteSpace: 'nowrap',
        }}>智能体工作区</div>
        <div style={{
          fontFamily: 'var(--mono)', fontSize: 11,
          color: 'var(--ink-4)', letterSpacing: '0.08em',
          padding: '2px 6px', border: '1px solid var(--rule-soft)',
          marginLeft: 4, whiteSpace: 'nowrap', flex: '0 0 auto',
        }}>
          {String(filtered.length).padStart(2, '0')} / {String(agents.length).padStart(2, '0')}
        </div>

        <div style={{
          display: 'flex', gap: 0, marginLeft: 12,
          border: '1px solid var(--rule-soft)', flex: '0 0 auto',
        }}>
          {[
            { k: 'all',      l: '全部' },
            { k: 'running',  l: '运行中' },
            { k: 'approval', l: '待审批' },
            { k: 'idle',     l: '空闲' },
            { k: 'error',    l: '错误' },
          ].map((f, i) => (
            <button key={f.k} onClick={() => setFilter(f.k)}
              style={{
                padding: '5px 10px',
                fontFamily: 'var(--mono)', fontSize: 11,
                background: filter === f.k ? 'var(--ink)' : 'transparent',
                color: filter === f.k ? 'var(--bg)' : 'var(--ink-3)',
                border: 'none',
                borderLeft: i > 0 ? '1px solid var(--rule-soft)' : 'none',
                cursor: 'pointer', letterSpacing: '0.04em',
                display: 'inline-flex', alignItems: 'center', gap: 6,
                whiteSpace: 'nowrap', flex: '0 0 auto',
              }}>
              <span style={{ whiteSpace: 'nowrap' }}>{f.l}</span>
              <span style={{
                fontSize: 9.5, padding: '0 4px',
                background: filter === f.k ? 'rgba(255,255,255,0.18)' : 'var(--rule-softer)',
                whiteSpace: 'nowrap',
              }}>{counts[f.k]}</span>
            </button>
          ))}
        </div>

        <div style={{
          marginLeft: 'auto',
          display: 'flex', alignItems: 'center', gap: 8,
          padding: '5px 10px', width: 180, minWidth: 120, flex: '0 1 180px',
          border: '1px solid var(--rule-soft)', background: 'var(--panel-2)',
        }}>
          <IconSearch size={12} style={{ color: 'var(--ink-4)', flex: '0 0 auto' }} />
          <input value={search} onChange={(e) => setSearch(e.target.value)}
                 placeholder="搜索..."
                 style={{
                   border: 'none', outline: 'none', background: 'transparent',
                   flex: 1, minWidth: 0, width: '100%',
                   fontSize: 12, fontFamily: 'var(--mono)', color: 'var(--ink-2)',
                 }} />
        </div>

        <button onClick={refresh} title="刷新"
          style={{
            width: 32, height: 32, display: 'grid', placeItems: 'center',
            background: 'var(--panel-2)', color: 'var(--ink-2)',
            border: '1px solid var(--rule-soft)', cursor: 'pointer',
            flex: '0 0 auto',
          }}>
          <span style={{ animation: refreshing ? 'spin 700ms linear' : 'none', display: 'grid' }}>
            <IconRefresh size={14} />
          </span>
        </button>

        <button onClick={() => setModalOpen(true)}
          style={{
            display: 'inline-flex', alignItems: 'center', gap: 8,
            padding: '7px 14px',
            fontFamily: 'var(--mono)', fontSize: 12, fontWeight: 600,
            letterSpacing: '0.08em',
            background: 'var(--ink)', color: 'var(--bg)',
            border: '1px solid var(--ink)', cursor: 'pointer',
            whiteSpace: 'nowrap', flex: '0 0 auto',
          }}>
          <IconPlus size={13} /> <span style={{ whiteSpace: 'nowrap' }}>新建智能体</span>
        </button>
      </div>

      <div style={{
        flex: 1, overflow: 'auto',
        padding: tweaks.viewMode === 'grid' ? '20px' : 0,
        position: 'relative',
      }}>
        {filtered.length === 0 ? (
          <EmptyState onNew={() => setModalOpen(true)} />
        ) : tweaks.viewMode === 'grid' ? (
          <div style={{
            display: 'grid',
            gridTemplateColumns: `repeat(auto-fill, minmax(${tweaks.density === 'compact' ? 260 : 300}px, 1fr))`,
            gap: tweaks.density === 'compact' ? 12 : 16,
          }}>
            {filtered.map((a) => (
              <AgentCard key={a.id} agent={a}
                onOpen={openAgent} onMenu={handleMenu}
                showMeta={tweaks.showMetrics} />
            ))}
          </div>
        ) : (
          <div>
            <div style={{
              display: 'grid',
              gridTemplateColumns: '2fr 2fr 100px 110px 110px',
              padding: '10px 14px', gap: 12,
              background: 'var(--panel)',
              borderBottom: '1px solid var(--rule)',
              fontFamily: 'var(--mono)', fontSize: 10,
              letterSpacing: '0.14em', textTransform: 'uppercase',
              color: 'var(--ink-4)',
              position: 'sticky', top: 0, zIndex: 1,
            }}>
              <div>NAME · SESSION</div>
              <div>WORK DIR</div>
              <div>PROFILE</div>
              <div>UPDATED</div>
              <div>STATUS</div>
            </div>
            {filtered.map((a) => (
              <AgentRow key={a.id} agent={a} onOpen={openAgent} />
            ))}
          </div>
        )}
      </div>

      <StatusBar agents={agents} kernel={true} />

      <NewInstanceModal open={modalOpen} onClose={() => setModalOpen(false)}
        onCreate={createAgent} existingNames={agents.map(a => a.name)} />
      <SettingsModal agent={settingsAgent} onClose={() => setSettingsAgent(null)}
        onSave={saveSettings} />
      <ConfirmModal open={!!deletingAgent}
        title="删除智能体"
        message={deletingAgent ? `确认删除 "${deletingAgent.name}"？此操作会清除该智能体的工作目录元数据，不可恢复。` : ''}
        onCancel={() => setDeletingAgent(null)} onConfirm={confirmDelete} />
      <TweaksPanel tweaks={tweaks} setTweak={setTweak} visible={tweaksOpen} />

      {toast && (
        <div style={{
          position: 'absolute', bottom: 48, left: '50%',
          transform: 'translateX(-50%)', zIndex: 50,
          padding: '8px 14px',
          fontFamily: 'var(--mono)', fontSize: 11,
          letterSpacing: '0.08em',
          background: 'var(--ink)', color: 'var(--bg)',
          border: '1px solid var(--ink)',
          boxShadow: '3px 3px 0 0 var(--rule)',
        }}>{toast}</div>
      )}
    </div>
  );
}

function EmptyState({ onNew }) {
  return (
    <div style={{ position: 'absolute', inset: 0, display: 'grid', placeItems: 'center' }}>
      <div style={{ textAlign: 'center', maxWidth: 340 }}>
        <div style={{
          width: 48, height: 48, border: '1px solid var(--rule)',
          display: 'inline-grid', placeItems: 'center',
          marginBottom: 14, color: 'var(--ink-3)',
        }}><IconChip size={22} /></div>
        <div style={{ fontSize: 15, fontWeight: 600, marginBottom: 6 }}>没有匹配的智能体</div>
        <div style={{
          fontFamily: 'var(--mono)', fontSize: 11, color: 'var(--ink-4)',
          letterSpacing: '0.06em', lineHeight: 1.6,
        }}>调整筛选条件，或创建新智能体</div>
        <button onClick={onNew} style={{
          marginTop: 16, padding: '8px 16px',
          fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 600,
          letterSpacing: '0.08em',
          background: 'var(--ink)', color: 'var(--bg)',
          border: '1px solid var(--ink)', cursor: 'pointer',
          display: 'inline-flex', alignItems: 'center', gap: 6,
        }}>
          <IconPlus size={12} /> 新建智能体
        </button>
      </div>
    </div>
  );
}

ReactDOM.createRoot(document.getElementById('root')).render(<App />);
