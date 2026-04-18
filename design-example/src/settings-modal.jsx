// Settings modal — edit name / profile / permissions for existing agent
function SettingsModal({ agent, onClose, onSave }) {
  const [name, setName] = React.useState(agent?.name || '');
  const [domain, setDomain] = React.useState(agent?.profile?.domain || 'coding');
  const [style, setStyle] = React.useState(agent?.profile?.style || 'balanced');
  const [permExec, setPermExec] = React.useState(agent?.permissions?.execute ?? true);
  const [permWrite, setPermWrite] = React.useState(agent?.permissions?.write ?? true);
  const [permSend, setPermSend] = React.useState(agent?.permissions?.send_to_agent ?? true);

  React.useEffect(() => {
    if (!agent) return;
    setName(agent.name);
    setDomain(agent.profile.domain);
    setStyle(agent.profile.style);
    setPermExec(agent.permissions.execute);
    setPermWrite(agent.permissions.write);
    setPermSend(agent.permissions.send_to_agent);
  }, [agent]);

  if (!agent) return null;
  const save = () => onSave(agent.id, {
    name, profile: { domain, style },
    permissions: { execute: permExec, write: permWrite, send_to_agent: permSend },
  });

  return (
    <>
      <div onClick={onClose} style={{
        position: 'absolute', inset: 0, background: 'rgba(20,18,15,0.45)', zIndex: 30,
      }} />
      <div style={{
        position: 'absolute', top: '50%', left: '50%',
        transform: 'translate(-50%, -50%)', width: 520,
        background: 'var(--panel-2)', border: '1px solid var(--rule)',
        boxShadow: '8px 8px 0 0 var(--rule)', zIndex: 31,
      }}>
        <div style={{
          padding: '10px 14px', borderBottom: '1px solid var(--rule)',
          background: 'var(--ink)', color: 'var(--bg)',
          display: 'flex', alignItems: 'center', gap: 10,
        }}>
          <IconSettings size={13} />
          <div style={{
            fontFamily: 'var(--mono)', fontSize: 11, fontWeight: 600, letterSpacing: '0.18em',
          }}>SETTINGS · {agent.id}</div>
          <button onClick={onClose} style={{
            marginLeft: 'auto', width: 24, height: 24, background: 'transparent',
            border: 'none', color: 'var(--bg)', cursor: 'pointer',
            display: 'grid', placeItems: 'center',
          }}><IconClose size={14} /></button>
        </div>

        <div style={{ padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
          <Field label="NAME" sub="智能体名称">
            <input value={name} onChange={(e) => setName(e.target.value)} style={inputStyle} />
          </Field>
          <Field label="WORK DIR" sub="只读">
            <div style={{
              padding: '9px 11px', fontFamily: 'var(--mono)', fontSize: 12,
              color: 'var(--ink-3)', background: 'var(--bg-2)',
              border: '1px solid var(--rule-softer)',
            }}>{agent.work_dir}</div>
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
            <div style={{ border: '1px solid var(--rule-soft)' }}>
              <PermRow label="执行命令" en="execute" value={permExec} onChange={setPermExec}
                       desc="运行 shell 命令 / 脚本" />
              <PermRow label="修改文件" en="write" value={permWrite} onChange={setPermWrite}
                       desc="写入、编辑工作目录文件" border />
              <PermRow label="跨智能体通信" en="send_to_agent" value={permSend} onChange={setPermSend}
                       desc="向其他智能体发送消息 / 文件" border />
            </div>
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
          <button onClick={save} style={{
            padding: '8px 18px', fontFamily: 'var(--mono)', fontSize: 11,
            background: 'var(--ink)', color: 'var(--bg)',
            border: '1px solid var(--ink)', cursor: 'pointer',
            letterSpacing: '0.08em', fontWeight: 600,
          }}>SAVE</button>
        </div>
      </div>
    </>
  );
}

Object.assign(window, { SettingsModal });
