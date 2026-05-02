// Flow diagrams (sitemap + sequence flows) — shown alongside screens

function FlowBox({ label, sub, w=120, h=46, style, dashed, primary }) {
  return (
    <div style={{
      width: w, minHeight: h,
      border: `${primary?2:1}px ${dashed?'dashed':'solid'} ${primary?'var(--teal-500)':'var(--line)'}`,
      borderRadius: 10,
      background: primary ? 'var(--teal-50)' : 'var(--paper)',
      padding: '8px 12px',
      display:'flex', flexDirection:'column', justifyContent:'center', alignItems:'center',
      textAlign:'center',
      boxShadow: dashed ? 'none' : '0 1px 2px rgba(15,42,46,0.04)',
      ...style,
    }}>
      <div style={{fontSize:11, fontWeight:700, color: primary?'var(--teal-700)':'var(--ink)'}}>{label}</div>
      {sub && <div style={{fontSize:9, color:'var(--ink-3)', marginTop:3, fontFamily:'ui-monospace,Menlo,monospace'}}>{sub}</div>}
    </div>
  );
}

function FlowArrow({ label, vertical, dashed, len=26 }) {
  const color = 'var(--teal-400)';
  if (vertical) {
    return (
      <div style={{display:'flex', flexDirection:'column', alignItems:'center'}}>
        <div style={{
          width:0, borderLeft: `1.5px ${dashed?'dashed':'solid'} ${color}`,
          height: len,
        }}/>
        {label && <div style={{fontSize:10, color:'var(--ink-2)', padding:'2px 8px', background:'var(--bg)', whiteSpace:'nowrap', fontWeight:500}}>{label}</div>}
        <div style={{
          width:0, height:0,
          borderLeft:'5px solid transparent',
          borderRight:'5px solid transparent',
          borderTop:`7px solid ${color}`,
          marginTop: -1,
        }}/>
      </div>
    );
  }
  return (
    <div style={{display:'flex', alignItems:'center'}}>
      <div style={{height:0, borderTop:`1.5px ${dashed?'dashed':'solid'} ${color}`, width: len}}/>
      {label && <div style={{fontSize:10, color:'var(--ink-2)', padding:'0 6px', fontWeight:500}}>{label}</div>}
      <div style={{
        width:0, height:0,
        borderTop:'5px solid transparent',
        borderBottom:'5px solid transparent',
        borderLeft:`7px solid ${color}`,
        marginLeft: -1,
      }}/>
    </div>
  );
}

function Flow_PrimaryFlow() {
  return (
    <div className="wf-root" style={{padding:'30px 40px'}}>
      <div className="wf-eyebrow">Section 3.1</div>
      <h2 className="wf-h2" style={{marginTop:4}}>作成から公開まで Primary Flow</h2>
      <p className="wf-sub" style={{marginTop:6, marginBottom:24}}>ランディング → /create → draft token 交換 → /prepare → /edit → 公開完了</p>

      <div style={{display:'flex', flexDirection:'column', gap:0, alignItems:'flex-start'}}>
        <div style={{display:'flex', alignItems:'center', flexWrap:'wrap', rowGap:14}}>
          <FlowBox label="/" sub="Landing"/>
          <FlowArrow label="今すぐ作る"/>
          <FlowBox label="/create" sub="作成入口" primary/>
          <FlowArrow label="POST /api/photobooks"/>
          <FlowBox label="/draft/{token}" sub="exchange" dashed/>
          <FlowArrow label="Cookie発行"/>
          <FlowBox label="/prepare/{id}" sub="写真追加" primary/>
        </div>

        <div style={{marginLeft: 580, marginTop: -8}}><FlowArrow vertical label="upload + 処理完了"/></div>

        <div style={{display:'flex', alignItems:'center', flexWrap:'wrap', rowGap:14, marginLeft: 480}}>
          <FlowBox label="/edit/{id}" sub="編集"/>
          <FlowArrow label="POST /publish"/>
          <FlowBox label="公開完了 view" sub="state内切替"/>
        </div>

        <div style={{marginLeft: 720, marginTop: 16, display:'flex', gap:32}}>
          <div style={{display:'flex', flexDirection:'column', alignItems:'center'}}>
            <FlowArrow vertical label="公開URL"/>
            <FlowBox label="/p/{slug}" sub="Viewer (新規タブ)"/>
          </div>
          <div style={{display:'flex', flexDirection:'column', alignItems:'center'}}>
            <FlowArrow vertical label="管理URL保存"/>
            <FlowBox label="/manage/token/{token}" sub="raw保存" dashed/>
          </div>
        </div>
      </div>

      <div className="wf-divider" style={{margin:'40px 0 24px'}}/>

      <div className="wf-eyebrow">Section 3.2 / 3.3</div>
      <h2 className="wf-h2" style={{marginTop:4}}>公開後の管理 / 閲覧者と通報</h2>

      <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap:36, marginTop:18}}>
        <div className="wf-box">
          <div className="wf-section-title">管理 flow</div>
          <div style={{display:'flex', flexDirection:'column', alignItems:'center', gap:0}}>
            <FlowBox label="保存済 /manage/token/{token}" w={220} dashed/>
            <FlowArrow vertical label="exchange + Cookie"/>
            <FlowBox label="/manage/{id}" sub="管理ページ" w={220}/>
            <div style={{display:'flex', gap:24, marginTop:14}}>
              <div style={{display:'flex', flexDirection:'column', alignItems:'center'}}>
                <FlowArrow vertical label="公開URLを開く"/>
                <FlowBox label="/p/{slug}" w={140}/>
              </div>
              <div style={{display:'flex', flexDirection:'column', alignItems:'center'}}>
                <FlowArrow vertical label="再発行"/>
                <FlowBox label="後日対応 disabled" w={140} dashed/>
              </div>
            </div>
          </div>
        </div>

        <div className="wf-box">
          <div className="wf-section-title">閲覧者・通報 flow</div>
          <div style={{display:'flex', flexDirection:'column', alignItems:'center', gap:0}}>
            <FlowBox label="共有された /p/{slug}" w={220}/>
            <FlowArrow vertical label="閲覧"/>
            <FlowBox label="公開 Viewer" w={220}/>
            <FlowArrow vertical label="通報を選択"/>
            <FlowBox label="/p/{slug}/report" w={220}/>
            <FlowArrow vertical label="Turnstile + submit"/>
            <FlowBox label="Thanks view" w={220}/>
          </div>
        </div>
      </div>

      <div className="wf-divider" style={{margin:'36px 0 24px'}}/>
      <div className="wf-eyebrow">Section 3.4</div>
      <h2 className="wf-h2" style={{marginTop:4}}>静的情報ページ導線</h2>
      <div style={{display:'flex', gap:14, flexWrap:'wrap', marginTop:14, alignItems:'center'}}>
        <FlowBox label="/" w={90}/>
        <FlowArrow len={14}/>
        <FlowBox label="/about" w={100}/>
        <FlowArrow len={14}/>
        <FlowBox label="/help/manage-url" w={130}/>
        <FlowArrow len={14}/>
        <FlowBox label="/terms" w={90}/>
        <FlowArrow len={14}/>
        <FlowBox label="/privacy" w={100}/>
      </div>
      <div className="wf-anno" style={{marginTop:10}}>
        / → /create, /about, /help, /terms, /privacy ・ /about → /terms, /privacy, /help ・ /terms → /help ・ /privacy → /terms ・ 公開完了 footer → /help
      </div>
    </div>
  );
}

Object.assign(window, { Flow_PrimaryFlow });
