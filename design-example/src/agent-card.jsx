// Agent session card — aligned with biene's SessionCard model
const cardStyles = {
  card: (hover, borderColor) => ({
    position: 'relative',
    background: 'var(--panel-2)',
    border: `1px solid ${borderColor || 'var(--rule)'}`,
    padding: 0,
    cursor: 'pointer',
    transition: 'transform 180ms cubic-bezier(.2,.7,.2,1), box-shadow 180ms',
    transform: hover ? 'translate(-2px,-2px)' : 'none',
    boxShadow: hover ? '4px 4px 0 0 var(--rule)' : '0 0 0 0 transparent',
    display: 'flex',
    flexDirection: 'column',
    minHeight: 180,
  }),
  topStrip: {
    display: 'grid',
    gridTemplateColumns: '1fr auto',
    alignItems: 'center',
    padding: '10px 14px',
    borderBottom: '1px dashed var(--rule-soft)',
    gap: 10,
  },
  statusTag: (color) => ({
    fontFamily: 'var(--mono)', fontSize: 10, fontWeight: 600,
    letterSpacing: '0.14em', color,
    display: 'inline-flex', alignItems: 'center', gap: 6,
    border: '1px solid currentColor', padding: '2px 7px',
    whiteSpace: 'nowrap',
  }),
  statusDot: (color, anim) => ({
    width: 6, height: 6, borderRadius: '50%', background: color,
    animation: anim ? 'pulse 1.6s infinite' : 'none',
  }),
  body: {
    padding: '14px 14px 12px',
    display: 'flex', flexDirection: 'column', gap: 10, flex: 1,
  },
  name: {
    fontFamily: 'var(--sans)', fontSize: 17, fontWeight: 600,
    letterSpacing: '-0.01em', color: 'var(--ink)',
    lineHeight: 1.15,
    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
  },
  lastMsg: {
    fontFamily: 'var(--sans)', fontSize: 12.5,
    color: 'var(--ink-3)', lineHeight: 1.5,
    display: '-webkit-box', WebkitLineClamp: 2,
    WebkitBoxOrient: 'vertical', overflow: 'hidden',
  },
  pathRow: {
    display: 'flex', alignItems: 'center', gap: 6,
    fontFamily: 'var(--mono)', fontSize: 11, color: 'var(--ink-4)',
    overflow: 'hidden',
  },
  footer: {
    display: 'flex', alignItems: 'center', gap: 6,
    borderTop: '1px solid var(--rule)',
    padding: '8px 12px',
    background: 'var(--panel)',
    flexWrap: 'wrap',
  },
  chip: {
    fontFamily: 'var(--mono)', fontSize: 10,
    letterSpacing: '0.08em',
    padding: '2px 6px',
    color: 'var(--ink-3)',
    border: '1px solid var(--rule-softer)',
    background: 'var(--panel-2)',
  },
  permChip: (on) => ({
    fontFamily: 'var(--mono)', fontSize: 9.5, fontWeight: 600,
    letterSpacing: '0.1em',
    padding: '2px 5px',
    color: on ? 'var(--ink-2)' : 'var(--ink-4)',
    background: on ? 'var(--bg-2)' : 'transparent',
    border: `1px solid ${on ? 'var(--rule-soft)' : 'var(--rule-softer)'}`,
    textDecoration: on ? 'none' : 'line-through',
    opacity: on ? 1 : 0.6,
  }),
};

function AgentCard({ agent, onOpen, onMenu, showMeta = true }) {
  const [hover, setHover] = React.useState(false);
  const [menuOpen, setMenuOpen] = React.useState(false);
  const meta = STATUS_META[agent.status];
  const borderColor =
    agent.status === 'approval' ? 'var(--warn)' :
    agent.status === 'error'    ? 'var(--err)'  : 'var(--rule)';

  return (
    <div
      style={cardStyles.card(hover, borderColor)}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={() => onOpen(agent)}
    >
      {/* top: name + menu */}
      <div style={cardStyles.topStrip}>
        <div style={cardStyles.name}>{agent.name}</div>
        <div style={{ position: 'relative' }} onClick={(e) => e.stopPropagation()}>
          <button
            onClick={() => setMenuOpen(v => !v)}
            style={{
              width: 24, height: 24, border: 'none',
              background: menuOpen ? 'var(--bg-2)' : 'transparent',
              color: 'var(--ink-4)', cursor: 'pointer',
              display: 'grid', placeItems: 'center',
              opacity: hover || menuOpen ? 1 : 0,
              transition: 'opacity 120ms',
            }}
          ><IconMore size={14} /></button>
          {menuOpen && (
            <div style={{
              position: 'absolute', top: 28, right: 0,
              minWidth: 140, background: 'var(--panel-2)',
              border: '1px solid var(--rule)',
              boxShadow: '3px 3px 0 0 var(--rule)',
              padding: 4, zIndex: 10,
            }}>
              <MenuItem onClick={() => { setMenuOpen(false); onMenu('open', agent); }}>打开窗口</MenuItem>
              <MenuItem onClick={() => { setMenuOpen(false); onMenu('settings', agent); }}>设置</MenuItem>
              <div style={{ height: 1, background: 'var(--rule-softer)', margin: '4px 0' }} />
              <MenuItem onClick={() => { setMenuOpen(false); onMenu('delete', agent); }} danger>删除</MenuItem>
            </div>
          )}
        </div>
      </div>

      {/* body */}
      <div style={cardStyles.body}>
        <div style={cardStyles.pathRow}>
          <IconFolder size={12} />
          <span style={{
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}>{shortDir(agent.work_dir)}</span>
        </div>

        <div style={cardStyles.lastMsg}>
          {agent.status === 'approval' && agent.pending_permission ? (
            <span style={{ color: 'var(--warn)', fontFamily: 'var(--mono)', fontSize: 11.5 }}>
              ⚠ 请求: {agent.pending_permission.tool} · {agent.pending_permission.path}
            </span>
          ) : (
            agent.last_message
          )}
        </div>

        <div style={{ marginTop: 'auto', display: 'flex', alignItems: 'center', gap: 8 }}>
          <div style={cardStyles.statusTag(meta.color)}>
            <span style={cardStyles.statusDot(meta.color, meta.dotAnim)} />
            {meta.en}
          </div>
          {agent.active_skill && (
            <div style={{
              fontFamily: 'var(--mono)', fontSize: 10, letterSpacing: '0.1em',
              color: 'var(--accent)',
              border: '1px solid var(--accent)', padding: '2px 6px',
              whiteSpace: 'nowrap',
            }}>
              ⚡ {agent.active_skill}
            </div>
          )}
          <div style={{
            marginLeft: 'auto',
            fontFamily: 'var(--mono)', fontSize: 10, color: 'var(--ink-4)',
            letterSpacing: '0.06em',
          }}>{agent.updated_at}</div>
        </div>
      </div>

      {/* footer: profile + permissions */}
      {showMeta && (
        <div style={cardStyles.footer}>
          <div style={cardStyles.chip}>{DOMAIN_LABELS[agent.profile.domain]}</div>
          <div style={cardStyles.chip}>{STYLE_LABELS[agent.profile.style]}</div>
          <div style={{ marginLeft: 'auto', display: 'flex', gap: 4 }}>
            <div style={cardStyles.permChip(agent.permissions.execute)}>EXEC</div>
            <div style={cardStyles.permChip(agent.permissions.write)}>WRITE</div>
            <div style={cardStyles.permChip(agent.permissions.send_to_agent)}>SEND</div>
          </div>
        </div>
      )}
    </div>
  );
}

function MenuItem({ children, onClick, danger }) {
  const [hov, setHov] = React.useState(false);
  return (
    <button
      onClick={onClick}
      onMouseEnter={() => setHov(true)}
      onMouseLeave={() => setHov(false)}
      style={{
        width: '100%', textAlign: 'left',
        padding: '6px 10px',
        fontFamily: 'var(--sans)', fontSize: 12,
        color: danger ? 'var(--err)' : 'var(--ink-2)',
        background: hov ? (danger ? '#FCEBE6' : 'var(--bg-2)') : 'transparent',
        border: 'none', cursor: 'pointer',
      }}
    >{children}</button>
  );
}

function AgentRow({ agent, onOpen }) {
  const [hover, setHover] = React.useState(false);
  const meta = STATUS_META[agent.status];
  return (
    <div
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onClick={() => onOpen(agent)}
      style={{
        display: 'grid',
        gridTemplateColumns: '2fr 2fr 100px 110px 110px',
        alignItems: 'center',
        padding: '12px 14px',
        borderBottom: '1px solid var(--rule-soft)',
        background: hover ? 'var(--panel-2)' : 'transparent',
        cursor: 'pointer', gap: 12, fontSize: 13,
      }}
    >
      <div>
        <div style={{ fontWeight: 600, color: 'var(--ink)', overflow: 'hidden',
          textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{agent.name}</div>
        <div style={{ fontFamily: 'var(--mono)', fontSize: 10.5, color: 'var(--ink-4)', marginTop: 2 }}>
          {agent.id}
        </div>
      </div>
      <div style={{
        fontFamily: 'var(--mono)', fontSize: 11, color: 'var(--ink-3)',
        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
      }}>{shortDir(agent.work_dir)}</div>
      <div style={{ fontFamily: 'var(--mono)', fontSize: 11, color: 'var(--ink-3)' }}>
        {DOMAIN_LABELS[agent.profile.domain]} · {STYLE_LABELS[agent.profile.style]}
      </div>
      <div style={{ fontFamily: 'var(--mono)', fontSize: 10.5, color: 'var(--ink-4)' }}>
        {agent.updated_at}
      </div>
      <div style={cardStyles.statusTag(meta.color)}>
        <span style={cardStyles.statusDot(meta.color, meta.dotAnim)} />
        {meta.en}
      </div>
    </div>
  );
}

Object.assign(window, { AgentCard, AgentRow });
