// Electron window chrome — macOS traffic lights + industrial title bar
const chromeStyles = {
  bar: {
    height: 40,
    display: 'flex',
    alignItems: 'center',
    gap: 14,
    padding: '0 14px 0 12px',
    background: 'var(--panel)',
    borderBottom: '1px solid var(--rule)',
    userSelect: 'none',
    position: 'relative',
    flex: '0 0 auto',
  },
  lights: {
    display: 'flex',
    gap: 8,
    alignItems: 'center',
  },
  light: (bg, border) => ({
    width: 12, height: 12,
    borderRadius: '50%',
    background: bg,
    border: `1px solid ${border}`,
    cursor: 'pointer',
  }),
  logoWrap: {
    display: 'flex', alignItems: 'center', gap: 10,
    marginLeft: 6,
  },
  logoMark: {
    width: 22, height: 22,
    display: 'grid', placeItems: 'center',
    border: '1px solid var(--ink)',
    background: 'var(--ink)',
    color: 'var(--bg)',
    fontFamily: 'var(--mono)',
    fontWeight: 700,
    fontSize: 11,
    letterSpacing: 0,
  },
  brand: {
    fontFamily: 'var(--mono)',
    fontSize: 12,
    fontWeight: 600,
    letterSpacing: '0.14em',
    color: 'var(--ink)',
  },
  divider: {
    width: 1, height: 16,
    background: 'var(--rule-soft)',
  },
  sub: {
    fontFamily: 'var(--sans)',
    fontSize: 12,
    color: 'var(--ink-3)',
    letterSpacing: '0.02em',
  },
  meta: {
    marginLeft: 'auto',
    display: 'flex', alignItems: 'center', gap: 16,
    fontFamily: 'var(--mono)',
    fontSize: 10.5,
    color: 'var(--ink-4)',
    letterSpacing: '0.08em',
    textTransform: 'uppercase',
  },
  metaItem: { display: 'flex', alignItems: 'center', gap: 6 },
  iconBtn: {
    width: 26, height: 26,
    display: 'grid', placeItems: 'center',
    color: 'var(--ink-3)',
    background: 'transparent',
    border: '1px solid transparent',
    cursor: 'pointer',
  },
};

function WindowChrome({ kernel, agentCount, onOpenSettings }) {
  return (
    <div style={chromeStyles.bar}>
      {/* traffic lights */}
      <div style={chromeStyles.lights}>
        <div style={chromeStyles.light('#FF5F57', '#E0443E')} />
        <div style={chromeStyles.light('#FEBC2E', '#DEA123')} />
        <div style={chromeStyles.light('#28C840', '#1AAB29')} />
      </div>

      {/* brand */}
      <div style={chromeStyles.logoWrap}>
        <div style={chromeStyles.logoMark}>B</div>
        <div style={chromeStyles.brand}>BIENE</div>
        <div style={chromeStyles.divider} />
        <div style={chromeStyles.sub}>Workspace</div>
      </div>

      {/* right-side: settings only */}
      <button style={{ ...chromeStyles.iconBtn, marginLeft: 'auto' }} onClick={onOpenSettings}
              onMouseEnter={(e)=> e.currentTarget.style.color='var(--ink)'}
              onMouseLeave={(e)=> e.currentTarget.style.color='var(--ink-3)'}>
        <IconSettings size={15} />
      </button>
    </div>
  );
}

// Bottom status bar
const statusStyles = {
  bar: {
    height: 28,
    display: 'flex',
    alignItems: 'center',
    gap: 18,
    padding: '0 14px',
    background: 'var(--panel)',
    borderTop: '1px solid var(--rule)',
    fontFamily: 'var(--mono)',
    fontSize: 10.5,
    letterSpacing: '0.08em',
    textTransform: 'uppercase',
    color: 'var(--ink-4)',
    flex: '0 0 auto',
  },
  item: { display: 'flex', alignItems: 'center', gap: 6 },
  dot: (c) => ({
    width: 6, height: 6, borderRadius: '50%', background: c, display: 'inline-block',
  }),
};

function StatusBar({ agents, kernel }) {
  const running  = agents.filter(a => a.status === 'running').length;
  const approval = agents.filter(a => a.status === 'approval').length;
  const errors   = agents.filter(a => a.status === 'error').length;

  return (
    <div style={statusStyles.bar}>
      <div style={statusStyles.item}>
        <span style={statusStyles.dot('var(--ok)')} />
        <span>{running} RUNNING</span>
      </div>
      {approval > 0 && (
        <div style={statusStyles.item}>
          <span style={statusStyles.dot('var(--warn)')} />
          <span style={{ color: 'var(--warn)'}}>{approval} PENDING</span>
        </div>
      )}
      {errors > 0 && (
        <div style={statusStyles.item}>
          <span style={statusStyles.dot('var(--err)')} />
          <span style={{ color: 'var(--err)'}}>{errors} ERR</span>
        </div>
      )}

      <div style={{ marginLeft: 'auto', ...statusStyles.item }}>
        <span>API · 127.0.0.1:8080</span>
      </div>
      <div style={statusStyles.item}>
        <span style={statusStyles.dot(kernel ? 'var(--ok)' : 'var(--err)')} />
        <span>CORE {kernel ? 'ONLINE' : 'OFFLINE'}</span>
      </div>
    </div>
  );
}

Object.assign(window, { WindowChrome, StatusBar });
