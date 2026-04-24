// VRC PhotoBook — Shared UI components & icons
// Icons: simple stroked SVG set (no libraries)

const Icon = {
  Book: (p) => (
    <svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M4 5.5a2 2 0 0 1 2-2h5v17H6a2 2 0 0 1-2-2V5.5Z"/><path d="M20 5.5a2 2 0 0 0-2-2h-5v17h5a2 2 0 0 0 2-2V5.5Z"/>
    </svg>
  ),
  Menu: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><path d="M4 7h16M4 12h16M4 17h16"/></svg>),
  ArrowLeft: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 6l-6 6 6 6"/></svg>),
  ArrowRight: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6"/></svg>),
  Chev: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 6l6 6-6 6"/></svg>),
  Check: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M5 12l4.5 4.5L19 7"/></svg>),
  Eye: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/></svg>),
  EyeOff: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M3 3l18 18"/><path d="M10.5 6.3A9.8 9.8 0 0 1 12 6c6.5 0 10 6 10 6a16 16 0 0 1-3 3.7"/><path d="M6.2 6.9A16 16 0 0 0 2 12s3.5 6 10 6a9 9 0 0 0 4-1"/></svg>),
  Link: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M10 14a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1.5 1.5"/><path d="M14 10a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1.5-1.5"/></svg>),
  Copy: (p) => (<svg width={p.size||15} height={p.size||15} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><rect x="8" y="8" width="13" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/></svg>),
  Camera: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M4 8a2 2 0 0 1 2-2h2l1.5-2h5L16 6h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8Z"/><circle cx="12" cy="13" r="3.5"/></svg>),
  Image: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="2"/><path d="M21 17l-5-5-9 9"/></svg>),
  Plus: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round"><path d="M12 5v14M5 12h14"/></svg>),
  Upload: (p) => (<svg width={p.size||18} height={p.size||18} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M12 15V4M7 9l5-5 5 5"/><path d="M4 15v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3"/></svg>),
  X: (p) => (<svg width={p.size||15} height={p.size||15} viewBox="0 0 24 24" fill="currentColor"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>),
  Share: (p) => (<svg width={p.size||15} height={p.size||15} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="M8.6 10.5l6.8-4M8.6 13.5l6.8 4"/></svg>),
  World: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3.5 3 14 0 18M12 3c-3 3.5-3 14 0 18"/></svg>),
  Users: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M17 20v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 20v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/></svg>),
  Globe: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="12" cy="12" r="9"/><path d="M3 12h18" strokeLinecap="round"/><ellipse cx="12" cy="12" rx="4.5" ry="9"/></svg>),
  Lock: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><rect x="4" y="10" width="16" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></svg>),
  Warn: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M10.3 3.5L1.7 18a2 2 0 0 0 1.7 3h17.2a2 2 0 0 0 1.7-3L13.7 3.5a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4M12 17h.01"/></svg>),
  Info: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 16v-4M12 8h.01"/></svg>),
  Sparkle: (p) => (<svg width={p.size||14} height={p.size||14} viewBox="0 0 24 24" fill="currentColor"><path d="M12 2l1.8 5.4L19 9.2l-5.2 1.8L12 16l-1.8-5L5 9.2l5.2-1.8z"/><path d="M18 14l.9 2.7L21 18l-2.1 1L18 22l-.9-3L15 18l2.1-1z" opacity=".6"/></svg>),
  Pencil: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4 12.5-12.5z"/></svg>),
  Trash: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg>),
  Refresh: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M3 12a9 9 0 0 1 15-6.7L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-15 6.7L3 16"/><path d="M3 21v-5h5"/></svg>),
  Calendar: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/></svg>),
  Mail: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 7l9 6 9-6"/></svg>),
  Send: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M21 3L3 10l8 3 3 8 7-18Z"/><path d="M21 3l-10 10"/></svg>),
  Grid: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>),
  Spread: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="3" y="5" width="8" height="14" rx="1"/><rect x="13" y="5" width="8" height="14" rx="1"/></svg>),
  Full: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="3" y="4" width="18" height="16" rx="2"/></svg>),
  Collage: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><rect x="3" y="3" width="10" height="10" rx="1"/><rect x="15" y="5" width="6" height="6" rx="1"/><rect x="15" y="13" width="6" height="8" rx="1"/><rect x="3" y="15" width="10" height="6" rx="1"/></svg>),
  Drag: (p) => (<svg width={p.size||14} height={p.size||14} viewBox="0 0 24 24" fill="currentColor"><circle cx="9" cy="6" r="1.4"/><circle cx="15" cy="6" r="1.4"/><circle cx="9" cy="12" r="1.4"/><circle cx="15" cy="12" r="1.4"/><circle cx="9" cy="18" r="1.4"/><circle cx="15" cy="18" r="1.4"/></svg>),
  Flag: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M5 22V4M5 4h12l-2 4 2 4H5"/></svg>),
  Shield: (p) => (<svg width={p.size||16} height={p.size||16} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2l8 3v7c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V5l8-3Z"/></svg>),
};

// ─────────────────────────────────────────────────────────────
// Phone shell — internal view that sits inside an iOS frame
// ─────────────────────────────────────────────────────────────
function PhoneShell({ children, stars = true }) {
  return (
    <div className="pb-root">
      {stars && <div className="pb-stars" />}
      {children}
    </div>
  );
}

function Logo() {
  return (
    <div className="logo">
      <div className="logo-ico"><Icon.Book size={14}/></div>
      <span>VRC <span style={{color:'var(--teal)'}}>PhotoBook</span></span>
    </div>
  );
}

function TopBar({ back, title, right }) {
  return (
    <div className="topbar">
      {back ? (
        <div style={{display:'flex',alignItems:'center',gap:10}}>
          <div className="icon-btn tap" onClick={back}><Icon.ArrowLeft/></div>
          {title && <div style={{fontWeight:700, fontSize:15}}>{title}</div>}
        </div>
      ) : (
        <Logo/>
      )}
      {right !== undefined ? right : <div className="icon-btn tap"><Icon.Menu/></div>}
    </div>
  );
}

function Steps({ active }) {
  const items = [
    { k: 'type', label: 'タイプ' },
    { k: 'edit', label: '編集' },
    { k: 'pub', label: '公開' },
  ];
  const idx = items.findIndex(x=>x.k===active);
  return (
    <div className="steps">
      {items.map((it, i) => (
        <React.Fragment key={it.k}>
          <div className={`step ${i<idx?'done':i===idx?'active':''}`}>
            <div className="dot">{i<idx ? <Icon.Check size={10}/> : i+1}</div>
            <span>{it.label}</span>
          </div>
          {i<items.length-1 && <div className={`step-line ${i<idx?'done':''}`} />}
        </React.Fragment>
      ))}
    </div>
  );
}

// Photo placeholder
function Photo({ variant = 'a', label, style, aspect, className = '' }) {
  return (
    <div
      className={`photo v-${variant} ${className}`}
      style={{ aspectRatio: aspect, ...style }}
    >
      {label && <div className="ph-label">{label}</div>}
    </div>
  );
}

// Avatar with "initial" or gradient
function Av({ label = 'N', size = 'md', variant = 'a', style }) {
  const cls = size === 'lg' ? 'avatar lg' : size === 'sm' ? 'avatar sm' : 'avatar';
  const bg = {
    a: 'linear-gradient(135deg, #8B5CF6, #3B82F6)',
    b: 'linear-gradient(135deg, #EC4899, #8B5CF6)',
    c: 'linear-gradient(135deg, #06B6D4, #3B82F6)',
    d: 'linear-gradient(135deg, #F472B6, #C084FC)',
    e: 'linear-gradient(135deg, #34D399, #06B6D4)',
  }[variant];
  return <div className={cls} style={{ background: bg, ...style }}>{label}</div>;
}

// URL row used across pages
function UrlRow({ url, cyan }) {
  return (
    <div className="url-pill" style={{color: cyan === false ? 'var(--violet)' : 'var(--teal)'}}>
      <Icon.Link size={14}/>
      <span style={{flex:1, overflow:'hidden', textOverflow:'ellipsis'}}>{url}</span>
    </div>
  );
}

Object.assign(window, { Icon, PhoneShell, Logo, TopBar, Steps, Photo, Av, UrlRow });
