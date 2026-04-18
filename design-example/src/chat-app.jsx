// Chat shell — one Electron window per agent
function ChatApp() {
  const [agent, setAgent] = React.useState(getUrlAgent());
  const [messages, setMessages] = React.useState(INITIAL_MESSAGES);
  const [streaming, setStreaming] = React.useState(false);
  const [interrupting, setInterrupting] = React.useState(false);
  const [activeSkill, setActiveSkill] = React.useState(null);
  const [pendingPerm, setPendingPerm] = React.useState(
    agent.status === 'approval'
      ? { request_id: 'req_1', permission: 'write', tool: 'edit_file', path: 'src/auth/middleware.ts' }
      : null
  );
  const listRef = React.useRef(null);
  const meta = STATUS_META[agent.status];

  React.useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight;
  }, [messages.length, pendingPerm]);

  const send = (text) => {
    const id = 'm' + Math.random().toString(16).slice(2, 8);
    setMessages((prev) => [...prev, {
      id, role: 'user', author_type: 'human',
      text, created_at: new Date().toISOString(), tool_calls: [],
    }]);
    setStreaming(true);
    setActiveSkill('reviewer');
    // simulate response
    setTimeout(() => {
      setMessages((prev) => [...prev, {
        id: 'm' + Math.random().toString(16).slice(2, 8),
        role: 'assistant', used_skill_name: 'reviewer',
        text: '收到。我先看一下当前结构再给出分步计划。',
        tool_calls: [
          { tool_id: 't' + Math.random().toString(16).slice(2, 6),
            tool_name: 'list_files', status: 'done',
            tool_summary: 'src/auth',
            tool_input: { path: 'src/auth' },
            result: 'middleware.ts\nroutes.ts\nutils/\nverify-jwt.ts' },
        ],
      }]);
      setStreaming(false);
      setActiveSkill(null);
    }, 1400);
  };

  const interrupt = () => {
    setInterrupting(true);
    setTimeout(() => {
      setStreaming(false); setInterrupting(false); setActiveSkill(null);
    }, 500);
  };

  const resolvePerm = (_decision) => setPendingPerm(null);
  const lastIsUser = messages[messages.length - 1]?.role === 'user';

  return (
    <div style={{
      height: '100%', display: 'flex', flexDirection: 'column',
      background: 'var(--bg)', position: 'relative', overflow: 'hidden',
    }}>
      {/* window chrome — compact for agent window */}
      <div style={{
        height: 36, display: 'flex', alignItems: 'center', gap: 12,
        padding: '0 14px 0 12px',
        background: 'var(--panel)', borderBottom: '1px solid var(--rule)',
        flex: '0 0 auto', userSelect: 'none',
      }}>
        <div style={{ display: 'flex', gap: 8 }}>
          <div style={{ width: 12, height: 12, borderRadius: '50%', background: '#FF5F57', border: '1px solid #E0443E' }} />
          <div style={{ width: 12, height: 12, borderRadius: '50%', background: '#FEBC2E', border: '1px solid #DEA123' }} />
          <div style={{ width: 12, height: 12, borderRadius: '50%', background: '#28C840', border: '1px solid #1AAB29' }} />
        </div>
        <div style={{
          fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 600,
          letterSpacing: '0.14em', color: 'var(--ink)',
        }}>BIENE · AGENT</div>
        <div style={{ width: 1, height: 14, background: 'var(--rule-soft)' }} />
        <div style={{
          fontSize: 12, color: 'var(--ink-2)', fontWeight: 500,
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>{agent.name}</div>
        <div style={{ flex: 1 }} />
        <div style={{
          display: 'inline-flex', alignItems: 'center', gap: 6,
          fontFamily: 'var(--mono)', fontSize: 10, fontWeight: 600,
          letterSpacing: '0.14em', color: meta.color,
          border: '1px solid currentColor', padding: '2px 7px',
        }}>
          <span style={{
            width: 6, height: 6, borderRadius: '50%', background: meta.color,
            animation: meta.dotAnim ? 'pulse 1.6s infinite' : 'none',
          }} />
          {meta.en}
        </div>
      </div>

      {/* session meta strip */}
      <div style={{
        display: 'flex', alignItems: 'center', gap: 16,
        padding: '10px 20px',
        borderBottom: '1px solid var(--rule-soft)',
        background: 'var(--panel-2)',
        flex: '0 0 auto',
      }}>
        <div>
          <div style={{
            fontSize: 17, fontWeight: 700, letterSpacing: '-0.01em',
            color: 'var(--ink)', lineHeight: 1.1,
          }}>{agent.name}</div>
          <div style={{
            marginTop: 3,
            fontFamily: 'var(--mono)', fontSize: 11, color: 'var(--ink-4)',
            letterSpacing: '0.02em',
          }}>{agent.id}</div>
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{
            fontFamily: 'var(--mono)', fontSize: 10,
            letterSpacing: '0.18em', color: 'var(--ink-4)', fontWeight: 600,
          }}>WORK DIR</div>
          <div style={{
            marginTop: 2,
            fontFamily: 'var(--mono)', fontSize: 12, color: 'var(--ink-2)',
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}>{agent.work_dir}</div>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <MetaChip label={DOMAIN_LABELS[agent.profile.domain]} />
          <MetaChip label={STYLE_LABELS[agent.profile.style]} />
        </div>
        <div style={{ display: 'flex', gap: 4 }}>
          <PermTag on={agent.permissions.execute} label="EXEC" />
          <PermTag on={agent.permissions.write} label="WRITE" />
          <PermTag on={agent.permissions.send_to_agent} label="SEND" />
        </div>
      </div>

      {/* messages */}
      <div ref={listRef} style={{
        flex: 1, overflow: 'auto', padding: '4px 28px 20px',
      }}>
        {messages.length === 0 ? (
          <EmptyChat agent={agent} />
        ) : messages.map((m) => <MessageItem key={m.id} msg={m} />)}

        {pendingPerm && <PermissionDialog req={pendingPerm} onResolve={resolvePerm} />}

        {streaming && lastIsUser && !pendingPerm && (
          <div style={{
            display: 'flex', alignItems: 'center', gap: 10,
            padding: '12px 0 12px 30px',
          }}>
            <div style={{ display: 'flex', gap: 4 }}>
              {[0, 1, 2].map((i) => (
                <span key={i} style={{
                  width: 6, height: 6, background: 'var(--ink-3)',
                  animation: `blink 1.1s ${i * 0.15}s infinite`,
                }} />
              ))}
            </div>
            {activeSkill && (
              <span style={{
                fontFamily: 'var(--mono)', fontSize: 11,
                color: 'var(--accent)', letterSpacing: '0.08em',
              }}>使用技能 · {activeSkill}</span>
            )}
          </div>
        )}
      </div>

      <InputBar
        disabled={streaming}
        interruptible={streaming}
        interrupting={interrupting}
        onSend={send}
        onInterrupt={interrupt}
      />
    </div>
  );
}

function MetaChip({ label }) {
  return (
    <div style={{
      fontFamily: 'var(--mono)', fontSize: 10, letterSpacing: '0.08em',
      padding: '3px 7px', color: 'var(--ink-3)',
      border: '1px solid var(--rule-softer)',
      background: 'var(--panel-2)',
    }}>{label}</div>
  );
}

function PermTag({ on, label }) {
  return (
    <div style={{
      fontFamily: 'var(--mono)', fontSize: 10, fontWeight: 600,
      letterSpacing: '0.1em',
      padding: '3px 6px',
      color: on ? 'var(--ink-2)' : 'var(--ink-4)',
      background: on ? 'var(--bg-2)' : 'transparent',
      border: `1px solid ${on ? 'var(--rule-soft)' : 'var(--rule-softer)'}`,
      textDecoration: on ? 'none' : 'line-through',
      opacity: on ? 1 : 0.6,
    }}>{label}</div>
  );
}

function EmptyChat({ agent }) {
  return (
    <div style={{
      flex: 1, display: 'flex', flexDirection: 'column',
      alignItems: 'center', justifyContent: 'center',
      padding: '80px 20px', textAlign: 'center', gap: 10,
    }}>
      <div style={{
        width: 52, height: 52, border: '1px solid var(--rule)',
        display: 'grid', placeItems: 'center', color: 'var(--ink-3)',
        marginBottom: 4,
      }}>⚡</div>
      <div style={{
        fontSize: 15, fontWeight: 600, color: 'var(--ink-2)',
      }}>智能体就绪</div>
      <div style={{
        fontFamily: 'var(--mono)', fontSize: 11,
        color: 'var(--ink-4)', letterSpacing: '0.04em',
      }}>WORK DIR · {agent.work_dir}</div>
    </div>
  );
}

// keyframes for blink
const sty = document.createElement('style');
sty.textContent = `
  @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:.4} }
  @keyframes spin  { to { transform: rotate(360deg) } }
  @keyframes blink { 0%,100%{opacity:.2} 50%{opacity:1} }
  button { font-family: inherit; }
  textarea:focus, input:focus { outline: none; }
  ::selection { background: var(--accent); color: var(--ink); }
`;
document.head.appendChild(sty);

ReactDOM.createRoot(document.getElementById('root')).render(<ChatApp />);
