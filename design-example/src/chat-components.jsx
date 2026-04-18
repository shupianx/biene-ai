// Industrial chat view — biene AgentChatView.vue adapted
const { useState: useChatState, useEffect: useChatEffect, useRef: useChatRef, useMemo: useChatMemo } = React;

function getUrlAgent() {
  const p = new URLSearchParams(window.location.search);
  const id = p.get('id');
  if (!id) return INITIAL_AGENTS[0];
  return INITIAL_AGENTS.find(a => a.id === id) || INITIAL_AGENTS[0];
}

function fmtTime(iso) {
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  } catch { return ''; }
}

// ── Tool call card ─────────────────────────────────────────────────────────
function ToolCallCard({ tc }) {
  const [open, setOpen] = useChatState(false);
  const colors = {
    composing: 'var(--ink-4)',
    pending:   'var(--warn)',
    done:      'var(--ok)',
    error:     'var(--err)',
    denied:    'var(--ink-4)',
    cancelled: 'var(--ink-3)',
  };
  const glyph = {
    composing: '…', pending: '⟳', done: '✓', error: '✗', denied: '⊘', cancelled: '■',
  }[tc.status] || '?';
  const color = colors[tc.status] || 'var(--ink-3)';

  return (
    <div style={{
      margin: '6px 0',
      border: '1px solid var(--rule-soft)',
      background: 'var(--panel-2)',
    }}>
      <div onClick={() => setOpen(v => !v)}
        style={{
          display: 'flex', alignItems: 'center', gap: 10,
          padding: '8px 12px',
          borderBottom: open ? '1px dashed var(--rule-soft)' : 'none',
          cursor: 'pointer',
          background: open ? 'var(--panel)' : 'transparent',
        }}>
        <span style={{
          width: 18, height: 18, display: 'grid', placeItems: 'center',
          fontFamily: 'var(--mono)', fontSize: 11,
          color, border: `1px solid ${color}`,
          animation: tc.status === 'pending' ? 'spin 1.2s linear infinite' : 'none',
        }}>{glyph}</span>
        <span style={{
          fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 600,
          letterSpacing: '0.08em', color: 'var(--ink-2)',
        }}>{tc.tool_name.toUpperCase()}</span>
        <span style={{
          flex: 1, minWidth: 0,
          fontFamily: 'var(--mono)', fontSize: 11.5, color: 'var(--ink-3)',
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>{tc.tool_summary}</span>
        <span style={{
          fontFamily: 'var(--mono)', fontSize: 10,
          letterSpacing: '0.12em', color,
        }}>{tc.status.toUpperCase()}</span>
        <span style={{ color: 'var(--ink-4)', fontSize: 10 }}>{open ? '▲' : '▼'}</span>
      </div>
      {open && (
        <div style={{ padding: '10px 12px' }}>
          {tc.tool_input && (
            <CodeSection label="INPUT" content={typeof tc.tool_input === 'string'
              ? tc.tool_input : JSON.stringify(tc.tool_input, null, 2)} />
          )}
          {tc.result && (
            <CodeSection label="OUTPUT" content={tc.result} />
          )}
        </div>
      )}
    </div>
  );
}

function CodeSection({ label, content }) {
  return (
    <div style={{ marginBottom: 8 }}>
      <div style={{
        fontFamily: 'var(--mono)', fontSize: 9.5, fontWeight: 600,
        letterSpacing: '0.18em', color: 'var(--ink-4)', marginBottom: 4,
      }}>{label}</div>
      <pre style={{
        margin: 0,
        padding: 10,
        background: 'var(--ink)', color: '#E8DFC9',
        fontFamily: 'var(--mono)', fontSize: 11.5, lineHeight: 1.5,
        whiteSpace: 'pre-wrap', wordBreak: 'break-word',
        maxHeight: 200, overflow: 'auto',
        border: '1px solid var(--ink)',
      }}>{content}</pre>
    </div>
  );
}

// ── Message item ────────────────────────────────────────────────────────────
function MessageItem({ msg }) {
  if (msg.role === 'user') {
    const fromAgent = msg.author_type === 'agent';
    return (
      <div style={{ display: 'flex', justifyContent: 'flex-end', padding: '12px 0' }}>
        <div style={{ maxWidth: '72%', display: 'flex', flexDirection: 'column', alignItems: 'flex-end' }}>
          {fromAgent && (
            <div style={{
              fontFamily: 'var(--mono)', fontSize: 10, letterSpacing: '0.14em',
              color: 'var(--accent)', marginBottom: 4,
            }}>FROM · {msg.author_name || msg.author_id}</div>
          )}
          <div style={{
            padding: '10px 14px',
            background: fromAgent ? 'var(--panel)' : 'var(--bg-2)',
            border: `1px solid ${fromAgent ? 'var(--accent)' : 'var(--rule-soft)'}`,
            color: 'var(--ink)', fontSize: 14, lineHeight: 1.55,
            whiteSpace: 'pre-wrap', wordBreak: 'break-word',
          }}>{msg.text}</div>
          <div style={{
            marginTop: 4, fontFamily: 'var(--mono)', fontSize: 10,
            color: 'var(--ink-4)', letterSpacing: '0.06em',
          }}>{fmtTime(msg.created_at)}</div>
        </div>
      </div>
    );
  }

  // assistant
  return (
    <div style={{ padding: '12px 0', position: 'relative' }}>
      <div style={{
        display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8,
      }}>
        <div style={{
          width: 22, height: 22, display: 'grid', placeItems: 'center',
          background: 'var(--ink)', color: 'var(--bg)',
          fontFamily: 'var(--mono)', fontSize: 10, fontWeight: 700,
          letterSpacing: 0,
        }}>B</div>
        <div style={{
          fontFamily: 'var(--mono)', fontSize: 10,
          letterSpacing: '0.16em', color: 'var(--ink-3)', fontWeight: 600,
        }}>ASSISTANT</div>
        {msg.used_skill_name && (
          <div style={{
            fontFamily: 'var(--mono)', fontSize: 10, letterSpacing: '0.1em',
            color: 'var(--accent)', border: '1px solid var(--accent)',
            padding: '1px 6px',
          }}>⚡ SKILL · {msg.used_skill_name}</div>
        )}
        {msg.streaming && (
          <div style={{
            fontFamily: 'var(--mono)', fontSize: 10, color: 'var(--ok)',
            letterSpacing: '0.12em',
          }}>▌STREAMING</div>
        )}
      </div>
      {msg.text && (
        <div style={{
          fontSize: 14, lineHeight: 1.65, color: 'var(--ink)',
          whiteSpace: 'pre-wrap', wordBreak: 'break-word',
          paddingLeft: 30,
        }}>{msg.text}</div>
      )}
      {msg.tool_calls?.length > 0 && (
        <div style={{ paddingLeft: 30, marginTop: msg.text ? 8 : 0 }}>
          {msg.tool_calls.map((tc, i) => <ToolCallCard key={i} tc={tc} />)}
        </div>
      )}
    </div>
  );
}

// ── Permission dialog ───────────────────────────────────────────────────────
function PermissionDialog({ req, onResolve }) {
  if (!req) return null;
  return (
    <div style={{
      margin: '12px 0',
      border: '1px solid var(--warn)',
      background: 'var(--panel-2)',
      boxShadow: '3px 3px 0 0 var(--warn)',
    }}>
      <div style={{
        padding: '8px 12px',
        background: 'var(--warn)', color: 'var(--ink)',
        fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 700,
        letterSpacing: '0.18em',
        display: 'flex', alignItems: 'center', gap: 8,
      }}>
        <span>⚠</span>
        <span>PERMISSION REQUIRED · {req.permission?.toUpperCase() || 'WRITE'}</span>
      </div>
      <div style={{ padding: '14px 14px 6px', fontSize: 13, color: 'var(--ink-2)' }}>
        智能体请求执行 <span style={{
          fontFamily: 'var(--mono)', fontSize: 12, color: 'var(--ink)',
          background: 'var(--bg-2)', padding: '1px 6px',
          border: '1px solid var(--rule-soft)',
        }}>{req.tool}</span> 工具。
      </div>
      {req.path && (
        <div style={{
          padding: '0 14px 10px', fontFamily: 'var(--mono)', fontSize: 11.5,
          color: 'var(--ink-3)', letterSpacing: '0.04em',
        }}>TARGET · {req.path}</div>
      )}
      <div style={{
        padding: '10px 14px',
        borderTop: '1px dashed var(--rule-soft)',
        display: 'flex', gap: 8, justifyContent: 'flex-end',
        background: 'var(--panel)',
      }}>
        <PermBtn onClick={() => onResolve('deny')} variant="deny">拒绝</PermBtn>
        <PermBtn onClick={() => onResolve('allow')} variant="allow">允许一次</PermBtn>
        <PermBtn onClick={() => onResolve('always')} variant="always">总是允许</PermBtn>
      </div>
    </div>
  );
}

function PermBtn({ children, onClick, variant }) {
  const styles = {
    deny:   { bg: 'transparent', color: 'var(--ink-2)', border: 'var(--rule-soft)' },
    allow:  { bg: 'var(--panel)', color: 'var(--ink)', border: 'var(--rule)' },
    always: { bg: 'var(--ink)', color: 'var(--bg)', border: 'var(--ink)' },
  }[variant];
  return (
    <button onClick={onClick} style={{
      padding: '7px 14px',
      fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 600,
      letterSpacing: '0.08em',
      background: styles.bg, color: styles.color,
      border: `1px solid ${styles.border}`, cursor: 'pointer',
    }}>{children}</button>
  );
}

// ── Input bar ───────────────────────────────────────────────────────────────
function InputBar({ disabled, interruptible, interrupting, onSend, onInterrupt }) {
  const [text, setText] = useChatState('');
  const taRef = useChatRef(null);

  const resize = () => {
    const el = taRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 160) + 'px';
  };

  const submit = () => {
    const t = text.trim();
    if (!t || disabled) return;
    onSend(t);
    setText('');
    setTimeout(() => {
      if (taRef.current) taRef.current.style.height = 'auto';
    }, 0);
  };

  const onKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      if (!interruptible) submit();
    }
  };

  const handleAction = () => {
    if (interruptible) {
      if (!interrupting) onInterrupt?.();
    } else submit();
  };

  return (
    <div style={{
      padding: '12px 20px 16px',
      borderTop: '1px solid var(--rule-soft)',
      background: 'var(--bg)',
      flex: '0 0 auto',
    }}>
      <div style={{
        display: 'flex', flexDirection: 'column',
        border: '1px solid var(--rule)',
        background: 'var(--panel-2)',
        padding: '12px 14px 10px',
      }}>
        <div style={{
          display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8,
        }}>
          <span style={{
            fontFamily: 'var(--mono)', fontSize: 9.5, fontWeight: 600,
            letterSpacing: '0.18em', color: 'var(--ink-4)',
          }}>MSG · INPUT</span>
          <span style={{ flex: 1, height: 1, background: 'var(--rule-softer)' }} />
          <span style={{
            fontFamily: 'var(--mono)', fontSize: 9.5,
            letterSpacing: '0.1em', color: 'var(--ink-4)',
          }}>{text.length} CHARS</span>
        </div>
        <textarea ref={taRef} value={text}
          onChange={(e) => { setText(e.target.value); resize(); }}
          onKeyDown={onKeyDown}
          disabled={disabled}
          placeholder="输入消息,  Enter 发送,  Shift+Enter 换行"
          rows={1}
          style={{
            width: '100%', minHeight: 40, maxHeight: 160,
            resize: 'none', border: 'none', outline: 'none',
            background: 'transparent',
            fontFamily: 'var(--sans)', fontSize: 14,
            lineHeight: 1.55, color: 'var(--ink)',
            padding: 0,
          }} />
        <div style={{
          display: 'flex', alignItems: 'center', gap: 8, marginTop: 10,
        }}>
          <div style={{
            fontFamily: 'var(--mono)', fontSize: 10, color: 'var(--ink-4)',
            letterSpacing: '0.08em',
          }}>
            ⌘↵ / SHIFT+↵ · 换行
          </div>
          <div style={{ flex: 1 }} />
          <button onClick={handleAction}
            disabled={interruptible ? interrupting : (disabled || !text.trim())}
            style={{
              padding: '7px 16px',
              fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 700,
              letterSpacing: '0.14em',
              background: interruptible ? 'var(--err)' : 'var(--ink)',
              color: interruptible ? '#fff' : 'var(--bg)',
              border: `1px solid ${interruptible ? 'var(--err)' : 'var(--ink)'}`,
              cursor: (interruptible ? interrupting : (disabled || !text.trim())) ? 'not-allowed' : 'pointer',
              opacity: (interruptible ? interrupting : (disabled || !text.trim())) ? 0.6 : 1,
              display: 'inline-flex', alignItems: 'center', gap: 6,
            }}>
            {interruptible
              ? (interrupting ? '中断中…' : '■ 中断')
              : <>发送 <span style={{ fontSize: 13 }}>↵</span></>}
          </button>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, { MessageItem, ToolCallCard, PermissionDialog, InputBar, getUrlAgent, fmtTime });
