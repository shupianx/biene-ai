// Tweaks panel — exposes design variants
const TWEAK_DEFAULTS = /*EDITMODE-BEGIN*/{
  "viewMode": "grid",
  "density": "comfortable",
  "accent": "amber",
  "showMetrics": true,
  "showLog": true,
  "showChrome": true,
  "paperGrain": true
}/*EDITMODE-END*/;

const ACCENT_MAP = {
  amber:   'oklch(68% 0.14 58)',
  rust:    'oklch(58% 0.16 38)',
  olive:   'oklch(62% 0.10 115)',
  steel:   'oklch(58% 0.08 240)',
  ink:     'oklch(28% 0.02 60)',
};

function TweaksPanel({ tweaks, setTweak, visible }) {
  if (!visible) return null;
  return (
    <div style={{
      position: 'absolute',
      right: 18, bottom: 48,
      width: 260,
      background: 'var(--panel-2)',
      border: '1px solid var(--ink)',
      boxShadow: '4px 4px 0 0 var(--rule)',
      zIndex: 40,
      fontFamily: 'var(--sans)',
    }}>
      <div style={{
        padding: '8px 12px',
        background: 'var(--ink)',
        color: 'var(--bg)',
        fontFamily: 'var(--mono)', fontSize: 10,
        letterSpacing: '0.2em',
        fontWeight: 600,
      }}>TWEAKS</div>

      <div style={{ padding: 12, display: 'flex', flexDirection: 'column', gap: 14 }}>
        <TweakRow label="布局">
          <Segmented
            value={tweaks.viewMode}
            options={[{ v: 'grid', l: '卡片' }, { v: 'list', l: '列表' }]}
            onChange={(v) => setTweak('viewMode', v)}
          />
        </TweakRow>

        <TweakRow label="密度">
          <Segmented
            value={tweaks.density}
            options={[
              { v: 'comfortable', l: '宽' },
              { v: 'compact', l: '紧凑' },
            ]}
            onChange={(v) => setTweak('density', v)}
          />
        </TweakRow>

        <TweakRow label="强调色">
          <div style={{ display: 'flex', gap: 6 }}>
            {Object.entries(ACCENT_MAP).map(([k, c]) => (
              <button
                key={k}
                onClick={() => setTweak('accent', k)}
                title={k}
                style={{
                  width: 22, height: 22,
                  background: c,
                  border: tweaks.accent === k ? '2px solid var(--ink)' : '1px solid var(--rule-soft)',
                  cursor: 'pointer',
                  padding: 0,
                }}
              />
            ))}
          </div>
        </TweakRow>

        <TweakToggle label="显示指标" value={tweaks.showMetrics} onChange={(v) => setTweak('showMetrics', v)} />
        <TweakToggle label="显示日志" value={tweaks.showLog} onChange={(v) => setTweak('showLog', v)} />
        <TweakToggle label="窗口栏" value={tweaks.showChrome} onChange={(v) => setTweak('showChrome', v)} />
        <TweakToggle label="纸纹" value={tweaks.paperGrain} onChange={(v) => setTweak('paperGrain', v)} />
      </div>
    </div>
  );
}

function TweakRow({ label, children }) {
  return (
    <div>
      <div style={{
        fontFamily: 'var(--mono)', fontSize: 10,
        letterSpacing: '0.14em', color: 'var(--ink-4)',
        marginBottom: 6,
      }}>{label.toUpperCase()}</div>
      {children}
    </div>
  );
}

function TweakToggle({ label, value, onChange }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
      <span style={{
        fontFamily: 'var(--mono)', fontSize: 11,
        letterSpacing: '0.08em', color: 'var(--ink-2)',
      }}>{label}</span>
      <button
        onClick={() => onChange(!value)}
        style={{
          width: 36, height: 18,
          background: value ? 'var(--ink)' : 'var(--rule-softer)',
          border: '1px solid var(--rule)',
          position: 'relative',
          cursor: 'pointer',
          padding: 0,
        }}
      >
        <span style={{
          position: 'absolute',
          top: 1, left: value ? 19 : 1,
          width: 14, height: 14,
          background: value ? 'var(--bg)' : 'var(--panel-2)',
          transition: 'left 140ms',
        }} />
      </button>
    </div>
  );
}

function Segmented({ value, options, onChange }) {
  return (
    <div style={{ display: 'flex', border: '1px solid var(--rule-soft)' }}>
      {options.map((o, i) => (
        <button key={o.v}
          onClick={() => onChange(o.v)}
          style={{
            flex: 1,
            padding: '5px 8px',
            fontFamily: 'var(--mono)', fontSize: 11,
            background: value === o.v ? 'var(--ink)' : 'transparent',
            color: value === o.v ? 'var(--bg)' : 'var(--ink-3)',
            border: 'none',
            borderLeft: i > 0 ? '1px solid var(--rule-soft)' : 'none',
            cursor: 'pointer',
            letterSpacing: '0.04em',
          }}
        >{o.l}</button>
      ))}
    </div>
  );
}

Object.assign(window, { TweaksPanel, TWEAK_DEFAULTS, ACCENT_MAP });
