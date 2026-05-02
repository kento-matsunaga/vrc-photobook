// Wireframe shared building blocks (mobile + PC shells, primitives) — concept-13 styling

function WFMobile({ title, back, right, children, noTopbar }) {
  return (
    <div className="wf-root">
      <div className="wf-m">
        {!noTopbar && (
          <div className="wf-m-topbar">
            <div className="wf-row" style={{gap:10}}>
              {back ? (
                <div className="nav-icon">←</div>
              ) : (
                <div style={{display:'flex',alignItems:'center',gap:8}}>
                  <div style={{width:22, height:18, background:'linear-gradient(135deg, var(--teal-300), var(--teal-500))', borderRadius:3}}/>
                  <span className="ttl">VRC PhotoBook</span>
                </div>
              )}
              {back && title && <div className="ttl">{title}</div>}
            </div>
            {right || <div className="nav-icon">⋯</div>}
          </div>
        )}
        <div className="wf-m-scroll">{children}</div>
      </div>
    </div>
  );
}

function WFBrowser({ url, children }) {
  return (
    <div className="wf-root">
      <div className="wf-pc">
        <div className="wf-pc-app">
          <div className="wf-pc-header">
            <div className="wf-pc-logo">VRC PhotoBook</div>
            <div className="wf-pc-nav">
              <span>作例</span>
              <span>使い方</span>
              <span>よくある質問</span>
              <div className="wf-btn primary sm">無料で作る</div>
            </div>
          </div>
          {children}
        </div>
      </div>
    </div>
  );
}

function WFImg({ label, style }) {
  return <div className="wf-img" style={style}><span>{label || 'IMAGE'}</span></div>;
}

function WFSection({ title, children, anno }) {
  return (
    <div style={{marginBottom: 22}}>
      {title && <div className="wf-section-title">{title}</div>}
      {children}
      {anno && <div className="wf-anno">▸ {anno}</div>}
    </div>
  );
}

function WFFooter({ extra, trust }) {
  return (
    <div>
      {trust && (
        <div className="wf-trust">
          <span>完全無料</span>
          <span>スマホで完成</span>
          <span>安全・安心</span>
          <span>VRCユーザー向け</span>
        </div>
      )}
      <div className="wf-footer">
        <div>© VRC PhotoBook</div>
        <div className="links">
          <span>About</span><span>Help</span><span>Terms</span><span>Privacy</span>
          {extra}
        </div>
      </div>
    </div>
  );
}

function WFTurnstile({ done }) {
  return (
    <div className="wf-box" style={{display:'flex', alignItems:'center', gap:12, padding:'12px 14px'}}>
      <div className={`wf-check ${done?'on':''}`} style={{pointerEvents:'none'}}>
        <div className="box">{done ? '✓' : ''}</div>
      </div>
      <div style={{flex:1, fontSize:12.5, color:'var(--ink-2)'}}>私はロボットではありません</div>
      <div style={{fontSize:9, color:'var(--ink-4)', textAlign:'right', lineHeight:1.3}}>
        <div style={{fontWeight:700}}>CLOUDFLARE</div>Turnstile
      </div>
    </div>
  );
}

// simple inline icon mark (just decorative; consistent with feature-card aesthetic)
function WFIcon({ name, style }) {
  const map = {
    user: '👤',
    link: '🔗',
    edit: '✎',
    calendar: '📅',
    chat: '💬',
    book: '📖',
    book2: '📘',
    sparkle: '✦',
    image: '🖼',
    lock: '🔒',
    check: '✓',
    flag: '⚐',
    info: 'i',
    plus: '+',
    cog: '⚙',
    eye: '◉',
  };
  return <div className="wf-feat-icon" style={style}>{map[name] || '◆'}</div>;
}

Object.assign(window, { WFMobile, WFBrowser, WFImg, WFSection, WFFooter, WFTurnstile, WFIcon });
