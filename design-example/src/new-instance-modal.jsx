// New agent modal — matches biene's NewAgentModal fields
function NewInstanceModal({ open, onClose, onCreate, existingNames }) {
  const [name, setName] = React.useState('');
  const [workDir, setWorkDir] = React.useState('');
  const [domain, setDomain] = React.useState('coding');
  const [style, setStyle] = React.useState('balanced');
  const [permExec, setPermExec] = React.useState(true);
  const [permWrite, setPermWrite] = React.useState(true);
  const [permSend, setPermSend] = React.useState(true);
  const [instructions, setInstructions] = React.useState('');

  React.useEffect(() => {
    if (open) {
      const n = existingNames?.length || 0;
      setName(`智能体 ${String(n + 1).padStart(2, '0')}`);
      setWorkDir(''); setDomain('coding'); setStyle('balanced');
      setPermExec(true); setPermWrite(true); setPermSend(true);
      setInstructions('');
    }
  }, [open]);

  if (!open) return null;

  const submit = () => {
    onCreate({
      name: name || '新智能体',
      work_dir: workDir || '/Users/yu/workspace/new-agent',
      profile: { domain, style, custom_instructions: instructions },
      permissions: { execute: permExec, write: permWrite, send_to_agent: permSend },
    });
  };

  return (
    <>
      <div onClick={onClose} style={{
        position: 'absolute', inset: 0, background: 'rgba(20,18,15,0.45)',
        zIndex: 30, animation: 'fadeIn 160ms',
      }} />
      <div style={{
        position: 'absolute', top: '50%', left: '50%',
        transform: 'translate(-50%, -50%)',
        width: 560, maxHeight: '88vh',
        background: 'var(--panel-2)', border: '1px solid var(--rule)',
        boxShadow: '8px 8px 0 0 var(--rule)',
        zIndex: 31, animation: 'popIn 180ms cubic-bezier(.2,.7,.2,1)',
        display: 'flex', flexDirection: 'column',
      }}>
        <div style={{
          display: 'flex', alignItems: 'center', gap: 10,
          padding: '10px 14px',
          borderBottom: '1px solid var(--rule)',
          background: 'var(--ink)', color: 'var(--bg)',
        }}>
          <IconPlus size={14} />
          <div style={{
            fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 600,
            letterSpacing: '0.18em',
          }}>NEW · AGENT</div>
          <button onClick={onClose} style={{
            marginLeft: 'auto', width: 24, height: 24,
            background: 'transparent', border: 'none', color: 'var(--bg)',
            cursor: 'pointer', display: 'grid', placeItems: 'center',
          }}><IconClose size={14} /></button>
        </div>

        <div style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 14, overflow: 'auto' }}>
          <Field label="NAME" sub="智能体名称">
            <input autoFocus value={name} onChange={(e) => setName(e.target.value)}
                   placeholder="例如: 重构 auth 模块" style={inputStyle} />
          </Field>

          <Field label="WORK DIR" sub="工作目录 (留空自动创建)">
            <input value={workDir} onChange={(e) => setWorkDir(e.target.value)}
                   placeholder="/Users/yu/workspace/project"
                   style={{ ...inputStyle, fontFamily: 'var(--mono)' }} />
          </Field>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 14 }}>
            <Field label="DOMAIN" sub="领域">
              <Segment value={domain} onChange={setDomain}
                options={[{ v: 'coding', l: '编程' }, { v: 'general', l: '通用' }]} />
            </Field>
            <Field label="STYLE" sub="风格">
              <select value={style} onChange={(e) => setStyle(e.target.value)}
                      style={{ ...inputStyle, fontFamily: 'var(--mono)', fontSize: 12 }}>
                {Object.entries(STYLE_LABELS).map(([k, l]) => (
                  <option key={k} value={k}>{k} · {l}</option>
                ))}
              </select>
            </Field>
          </div>

          <Field label="PERMISSIONS" sub="工具权限">
            <div style={{ display: 'grid', gap: 0, border: '1px solid var(--rule-soft)' }}>
              <PermRow label="执行命令" en="execute" value={permExec} onChange={setPermExec}
                       desc="允许运行 shell 命令 / 脚本" />
              <PermRow label="修改文件" en="write" value={permWrite} onChange={setPermWrite}
                       desc="允许写入、编辑工作目录文件" border />
              <PermRow label="跨智能体通信" en="send_to_agent" value={permSend} onChange={setPermSend}
                       desc="允许向其他智能体发送消息 / 文件" border />
            </div>
          </Field>

          <Field label="INSTRUCTIONS" sub="自定义指令 (可选)">
            <textarea value={instructions} onChange={(e) => setInstructions(e.target.value)}
                      placeholder="附加的系统提示..."
                      rows={3} style={{ ...inputStyle, resize: 'none', lineHeight: 1.5 }} />
          </Field>
        </div>

        <div style={{
          display: 'flex', gap: 8, justifyContent: 'flex-end',
          padding: '12px 20px', borderTop: '1px solid var(--rule-softer)',
          background: 'var(--panel)',
        }}>
          <button onClick={onClose} style={{
            padding: '8px 14px', fontFamily: 'var(--mono)', fontSize: 11,
            background: 'transparent', color: 'var(--ink-3)',
            border: '1px solid var(--rule-soft)', cursor: 'pointer',
            letterSpacing: '0.08em',
          }}>CANCEL</button>
          <button onClick={submit} style={{
            padding: '8px 18px', fontFamily: 'var(--mono)', fontSize: 11,
            background: 'var(--ink)', color: 'var(--bg)',
            border: '1px solid var(--ink)', cursor: 'pointer',
            letterSpacing: '0.08em', fontWeight: 600,
            display: 'inline-flex', alignItems: 'center', gap: 6,
          }}>
            <IconBolt size={12} /> CREATE
          </button>
        </div>
      </div>
    </>
  );
}

function PermRow({ label, en, value, onChange, desc, border }) {
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 12,
      padding: '10px 12px',
      borderTop: border ? '1px dashed var(--rule-soft)' : 'none',
    }}>
      <div style={{ flex: 1 }}>
        <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--ink)' }}>
          {label}
          <span style={{
            marginLeft: 8, fontFamily: 'var(--mono)', fontSize: 10,
            letterSpacing: '0.12em', color: 'var(--ink-4)',
          }}>{en}</span>
        </div>
        <div style={{ fontSize: 11, color: 'var(--ink-4)', marginTop: 2 }}>{desc}</div>
      </div>
      <button
        onClick={() => onChange(!value)}
        style={{
          width: 36, height: 18,
          background: value ? 'var(--ok)' : 'var(--rule-softer)',
          border: '1px solid var(--rule)',
          position: 'relative', cursor: 'pointer', padding: 0,
        }}>
        <span style={{
          position: 'absolute', top: 1, left: value ? 19 : 1,
          width: 14, height: 14,
          background: value ? 'var(--bg)' : 'var(--panel-2)',
          transition: 'left 140ms',
        }} />
      </button>
    </div>
  );
}

function Segment({ value, options, onChange }) {
  return (
    <div style={{ display: 'flex', border: '1px solid var(--rule-soft)' }}>
      {options.map((o, i) => (
        <button key={o.v} onClick={() => onChange(o.v)}
          style={{
            flex: 1, padding: '8px 10px',
            fontFamily: 'var(--mono)', fontSize: 11,
            background: value === o.v ? 'var(--ink)' : 'var(--panel-2)',
            color: value === o.v ? 'var(--bg)' : 'var(--ink-3)',
            border: 'none',
            borderLeft: i > 0 ? '1px solid var(--rule-soft)' : 'none',
            cursor: 'pointer', letterSpacing: '0.02em',
          }}>{o.l}</button>
      ))}
    </div>
  );
}

const inputStyle = {
  width: '100%', padding: '9px 11px',
  fontFamily: 'var(--sans)', fontSize: 13, color: 'var(--ink)',
  background: 'var(--bg)', border: '1px solid var(--rule-soft)',
  outline: 'none', boxSizing: 'border-box',
};

function Field({ label, sub, children }) {
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 10, marginBottom: 6 }}>
        <span style={{
          fontFamily: 'var(--mono)', fontSize: 10,
          letterSpacing: '0.18em', color: 'var(--ink-4)',
        }}>{label}</span>
        <span style={{ fontSize: 11, color: 'var(--ink-4)' }}>{sub}</span>
      </div>
      {children}
    </div>
  );
}

// Confirm modal for delete
function ConfirmModal({ open, title, message, onCancel, onConfirm }) {
  if (!open) return null;
  return (
    <>
      <div onClick={onCancel} style={{
        position: 'absolute', inset: 0, background: 'rgba(20,18,15,0.45)',
        zIndex: 32, animation: 'fadeIn 160ms',
      }} />
      <div style={{
        position: 'absolute', top: '50%', left: '50%',
        transform: 'translate(-50%, -50%)',
        width: 400,
        background: 'var(--panel-2)', border: '1px solid var(--rule)',
        boxShadow: '8px 8px 0 0 var(--rule)', zIndex: 33,
        animation: 'popIn 180ms cubic-bezier(.2,.7,.2,1)',
      }}>
        <div style={{
          padding: '14px 18px', borderBottom: '1px solid var(--rule-softer)',
          fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 600,
          letterSpacing: '0.18em', color: 'var(--err)',
        }}>{title}</div>
        <div style={{ padding: 18, fontSize: 13, lineHeight: 1.6, color: 'var(--ink-2)' }}>
          {message}
        </div>
        <div style={{
          display: 'flex', gap: 8, justifyContent: 'flex-end',
          padding: '12px 18px', borderTop: '1px solid var(--rule-softer)',
          background: 'var(--panel)',
        }}>
          <button onClick={onCancel} style={{
            padding: '7px 14px', fontFamily: 'var(--mono)', fontSize: 11,
            background: 'transparent', color: 'var(--ink-3)',
            border: '1px solid var(--rule-soft)', cursor: 'pointer',
            letterSpacing: '0.08em',
          }}>取消</button>
          <button onClick={onConfirm} style={{
            padding: '7px 14px', fontFamily: 'var(--mono)', fontSize: 11,
            background: 'var(--err)', color: 'white',
            border: '1px solid var(--err)', cursor: 'pointer',
            letterSpacing: '0.08em', fontWeight: 600,
          }}>删除</button>
        </div>
      </div>
    </>
  );
}

Object.assign(window, { NewInstanceModal, ConfirmModal });
