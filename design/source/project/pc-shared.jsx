// PC shared shell components

function PCBrowser({ url = 'https://vrc-photobook.com', children }) {
  return (
    <div className="pc-browser">
      <div className="pc-browser-chrome">
        <div className="pc-lights">
          <div className="pc-light" style={{background:'#FF5F57'}}/>
          <div className="pc-light" style={{background:'#FEBC2E'}}/>
          <div className="pc-light" style={{background:'#28C840'}}/>
        </div>
        <div className="pc-navs">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M15 6l-6 6 6 6"/></svg>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M9 6l6 6-6 6"/></svg>
        </div>
        <div className="pc-urlbar">
          <span className="lock"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg></span>
          <span>{url}</span>
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 12a9 9 0 0 1 15-6.7L21 8M21 3v5h-5"/></svg>
        </div>
      </div>
      <div className="pc-app">{children}</div>
    </div>
  );
}

function PCHeader({ go }) {
  return (
    <div className="pc-header">
      <div className="logo-xl tap" onClick={()=>go && go('pc-lp')}>
        <span className="logo-ico"><Icon.Book size={18}/></span>
        <span>VRC <span style={{color:'var(--teal)'}}>PhotoBook</span></span>
      </div>
      <div className="nav">
        <a>作例</a>
        <a>使い方</a>
        <a>よくある質問</a>
        <button className="btn btn-primary btn-sm" onClick={()=>go && go('pc-create')}>無料で作る</button>
      </div>
    </div>
  );
}

function PCSteps({ active }) {
  const items = [
    { k:'type', label:'1. タイプ選択' },
    { k:'edit', label:'2. 編集' },
    { k:'pub', label:'3. 公開' },
  ];
  const idx = items.findIndex(x=>x.k===active);
  return (
    <div className="pc-steps">
      {items.map((it, i) => (
        <React.Fragment key={it.k}>
          <div className={`step ${i<idx?'done':i===idx?'active':''}`}>
            <div className="dot">{i<idx ? <Icon.Check size={12}/> : i+1}</div>
            <span>{it.label}</span>
          </div>
          {i<items.length-1 && <div className={`line ${i<idx?'done':''}`} />}
        </React.Fragment>
      ))}
    </div>
  );
}

function PCTrust() {
  return (
    <div className="pc-trust">
      <div className="cell"><span className="ico"><Icon.Check size={14}/></span>完全無料</div>
      <div className="cell"><span className="ico"><Icon.Camera size={14}/></span>スマホで完結</div>
      <div className="cell"><span className="ico"><Icon.Lock size={14}/></span>安全・安心</div>
      <div className="cell"><span className="ico"><Icon.Sparkle size={14}/></span>VRCユーザー向け</div>
    </div>
  );
}

Object.assign(window, { PCBrowser, PCHeader, PCSteps, PCTrust });
