// Wireframe screens — Viewer, Report, About, Help, Terms, Privacy, ErrorState

// ── 4.9 Public Viewer /p/{slug}
function WFViewer_M() {
  return (
    <WFMobile title="" right={<div style={{fontSize:11}}>⋯</div>}>
      <div className="wf-m-pad">
        <WFImg label="COVER" style={{aspectRatio:'4/5'}}/>
        <h1 className="wf-h1" style={{marginTop:14}}>Cover Title</h1>
        <div className="wf-line medium" style={{marginTop:6}}/>
        <div className="wf-line long"/>
        <div className="wf-row" style={{gap:8, marginTop:12}}>
          <div style={{width:28,height:28,border:'1.5px solid var(--wf-line)',borderRadius:'50%'}}/>
          <span style={{fontSize:12, fontWeight:600}}>creator name</span>
          <span style={{fontSize:11, color:'var(--wf-ink-3)'}}>@x_id</span>
        </div>

        {[1,2,3].map(p=>(
          <WFSection key={p} title={`Page ${String(p).padStart(2,'0')}`}>
            <div className="wf-grid-2">
              {Array.from({length:4}).map((_,i)=> <WFImg key={i} style={{aspectRatio:'1/1'}}/> )}
            </div>
            <div className="wf-line short" style={{marginTop:8}}/>
            <div className="wf-line medium"/>
          </WFSection>
        ))}

        <WFFooter extra={<span style={{textDecoration:'underline'}}>このフォトブックを通報</span>}/>
        <div className="wf-note">404 → not found / 410 → gone / private/hidden/deleted/draft は到達不可</div>
      </div>
    </WFMobile>
  );
}
function WFViewer_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/p/midnight-social-club">
      <div className="wf-pc-container">
        <div className="wf-grid-2-1" style={{alignItems:'flex-start'}}>
          <div>
            <WFImg label="COVER" style={{aspectRatio:'1.4/1'}}/>
            <h1 className="wf-h1 lg" style={{marginTop:18}}>Cover Title</h1>
            <div className="wf-line medium" style={{marginTop:6}}/>
            <div className="wf-line long"/>
            {[1,2,3].map(p=>(
              <div key={p} style={{marginTop:24}}>
                <div className="wf-section-title">Page {String(p).padStart(2,'0')}</div>
                <div className="wf-grid-3">
                  {Array.from({length:6}).map((_,i)=> <WFImg key={i} style={{aspectRatio:'1/1'}}/> )}
                </div>
                <div className="wf-line short" style={{marginTop:8}}/>
                <div className="wf-line medium"/>
              </div>
            ))}
          </div>
          <div className="wf-stack" style={{position:'sticky', top:80}}>
            <div className="wf-box">
              <div className="wf-section-title">Creator</div>
              <div className="wf-row" style={{gap:10}}>
                <div style={{width:36,height:36,border:'1.5px solid var(--wf-line)',borderRadius:'50%'}}/>
                <div>
                  <div style={{fontSize:13,fontWeight:700}}>creator name</div>
                  <div style={{fontSize:11,color:'var(--wf-ink-3)'}}>@x_id</div>
                </div>
              </div>
            </div>
            <div className="wf-box">
              <div className="wf-anno" style={{textDecoration:'underline'}}>このフォトブックを通報 → /p/&#123;slug&#125;/report</div>
            </div>
            <div className="wf-note">OGP: /ogp/&#123;photobookId&#125;?v=1 (失敗時 /og/default.png)</div>
          </div>
        </div>
        <WFFooter/>
      </div>
    </WFBrowser>
  );
}

// ── 4.10 Report /p/{slug}/report
function WFReport_M() {
  const reasons = ['嫌がらせ・晒し','無断転載の可能性','被写体として削除希望','センシティブ設定の不足','年齢・センシティブ問題','その他'];
  return (
    <WFMobile title="通報" back>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">Report</div>
        <h1 className="wf-h2">{'{title}'} を通報</h1>
        <div className="wf-anno">← /p/&#123;slug&#125; back link</div>

        <WFSection title="Reason select (6種)">
          <div className="wf-stack" style={{gap:6}}>
            {reasons.map((r,i)=>(
              <div key={r} className={`wf-radio ${i===1?'active':''}`}>
                <div className="dot"/>
                <div style={{flex:1, fontSize:13}}>{r}</div>
              </div>
            ))}
          </div>
        </WFSection>

        <WFSection title="Detail (任意・最大2000)">
          <textarea className="wf-textarea"/>
          <div className="wf-counter">0 / 2000</div>
        </WFSection>

        <WFSection title="Contact (任意・最大200)">
          <input className="wf-input" placeholder="メールアドレスまたはX ID"/>
          <div className="wf-counter">0 / 200</div>
        </WFSection>

        <div style={{marginTop:8}}>
          <WFTurnstile done={false}/>
          <div className="wf-anno">action: report-submit</div>
        </div>

        <div className="wf-stack" style={{marginTop:16, gap:8}}>
          <div className="wf-btn primary lg full disabled">通報を送信 (Turnstile未完了)</div>
        </div>

        <div className="wf-divider"/>
        <div className="wf-section-title">Success / Thanks view</div>
        <div className="wf-box" style={{textAlign:'center', padding:'22px 14px'}}>
          <div style={{width:48,height:48,margin:'0 auto 12px',border:'2px solid var(--wf-ink)',borderRadius:'50%',display:'grid',placeItems:'center',fontWeight:700}}>✓</div>
          <div style={{fontWeight:700}}>通報を受け付けました</div>
          <div className="wf-anno" style={{marginTop:6}}>report id は表示しない</div>
        </div>
        <div className="wf-note warn" style={{marginTop:12}}>Errors: invalid payload / Turnstile failed / not found / rate limited / network・server</div>
      </div>
    </WFMobile>
  );
}
function WFReport_PC() {
  const reasons = ['嫌がらせ・晒し','無断転載の可能性','被写体として削除希望','センシティブ設定の不足','年齢・センシティブ問題','その他'];
  return (
    <WFBrowser url="https://vrc-photobook.com/p/midnight-social-club/report">
      <div className="wf-pc-container narrow">
        <div className="wf-eyebrow">Report</div>
        <h1 className="wf-h1">{'{title}'} を通報</h1>
        <div className="wf-anno">← /p/&#123;slug&#125; back link</div>

        <div className="wf-box" style={{marginTop:18}}>
          <div className="wf-section-title">Reason</div>
          <div className="wf-grid-2">
            {reasons.map((r,i)=>(
              <div key={r} className={`wf-radio ${i===1?'active':''}`}>
                <div className="dot"/>
                <div style={{flex:1, fontSize:13}}>{r}</div>
              </div>
            ))}
          </div>
        </div>

        <div className="wf-box" style={{marginTop:14}}>
          <label className="wf-label">Detail (任意・最大2000)</label>
          <textarea className="wf-textarea"/>
          <div className="wf-counter">0 / 2000</div>
          <label className="wf-label" style={{marginTop:14}}>Contact (任意・最大200)</label>
          <input className="wf-input"/>
          <div className="wf-counter">0 / 200</div>
        </div>

        <div style={{marginTop:14}}><WFTurnstile done={false}/></div>

        <div className="wf-row" style={{justifyContent:'flex-end', marginTop:18, gap:10}}>
          <div className="wf-btn">フォトブックに戻る</div>
          <div className="wf-btn primary disabled">通報を送信</div>
        </div>

        <div className="wf-box" style={{marginTop:24, textAlign:'center', padding:'28px'}}>
          <div className="wf-section-title" style={{justifyContent:'center'}}>Thanks view</div>
          <div style={{width:56,height:56,margin:'0 auto 14px',border:'2px solid var(--wf-ink)',borderRadius:'50%',display:'grid',placeItems:'center',fontWeight:700,fontSize:22}}>✓</div>
          <div style={{fontWeight:700, fontSize:15}}>通報を受け付けました</div>
          <div className="wf-anno">report id は表示しない</div>
        </div>
      </div>
    </WFBrowser>
  );
}

// ── 4.11 About /about
function WFAbout_M() {
  return (
    <WFMobile title="About" back>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">About</div>
        <h1 className="wf-h1">VRC PhotoBook<br/>について</h1>

        <WFSection title="サービスの位置づけ">
          <div className="wf-m-card">
            <div className="wf-line long"/><div className="wf-line long"/><div className="wf-line medium"/>
          </div>
        </WFSection>

        <WFSection title="できること (6件)">
          <div className="wf-stack" style={{gap:8}}>
            {[1,2,3,4,5,6].map(i=>(
              <div key={i} className="wf-row" style={{gap:10, padding:'8px 0', borderBottom:'1px solid var(--wf-line-2)'}}>
                <div style={{width:18,height:18,border:'1.5px solid var(--wf-ink)',borderRadius:'50%',display:'grid',placeItems:'center',fontSize:10,fontWeight:700}}>✓</div>
                <div style={{flex:1, fontSize:13}}>できること {i}</div>
              </div>
            ))}
          </div>
        </WFSection>

        <WFSection title="MVPでできないこと (4件)">
          <div className="wf-stack" style={{gap:8}}>
            {[1,2,3,4].map(i=>(
              <div key={i} className="wf-row" style={{gap:10, padding:'8px 0', borderBottom:'1px solid var(--wf-line-2)'}}>
                <div style={{width:18,height:18,border:'1.5px solid var(--wf-ink)',borderRadius:'50%',display:'grid',placeItems:'center',fontSize:11,fontWeight:700}}>×</div>
                <div style={{flex:1, fontSize:13}}>未対応 {i}</div>
              </div>
            ))}
          </div>
        </WFSection>

        <WFSection title="ポリシーと窓口">
          <div className="wf-stack" style={{gap:6}}>
            <div className="wf-btn full">/terms</div>
            <div className="wf-btn full">/privacy</div>
            <div className="wf-btn full">/help/manage-url</div>
            <div className="wf-anno">通報は Viewer から</div>
          </div>
        </WFSection>

        <WFFooter/>
      </div>
    </WFMobile>
  );
}
function WFAbout_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/about">
      <div className="wf-pc-container narrow">
        <div className="wf-eyebrow">About</div>
        <h1 className="wf-h1 lg">VRC PhotoBook について</h1>

        <div className="wf-box" style={{marginTop:20}}>
          <div className="wf-section-title">サービスの位置づけ</div>
          <div className="wf-line long"/><div className="wf-line long"/><div className="wf-line medium"/>
        </div>

        <div className="wf-grid-2" style={{marginTop:18, gap:18}}>
          <div className="wf-box">
            <div className="wf-section-title">できること (6)</div>
            {[1,2,3,4,5,6].map(i=>(
              <div key={i} className="wf-row" style={{gap:10, padding:'8px 0', borderBottom:'1px solid var(--wf-line-2)'}}>
                <span style={{fontSize:14}}>✓</span>
                <div className="wf-line long" style={{margin:0, flex:1}}/>
              </div>
            ))}
          </div>
          <div className="wf-box">
            <div className="wf-section-title">MVPでできないこと (4)</div>
            {[1,2,3,4].map(i=>(
              <div key={i} className="wf-row" style={{gap:10, padding:'8px 0', borderBottom:'1px solid var(--wf-line-2)'}}>
                <span style={{fontSize:14}}>×</span>
                <div className="wf-line long" style={{margin:0, flex:1}}/>
              </div>
            ))}
          </div>
        </div>

        <div className="wf-box" style={{marginTop:18}}>
          <div className="wf-section-title">ポリシーと窓口</div>
          <div className="wf-grid-3">
            <div className="wf-btn full">/terms</div>
            <div className="wf-btn full">/privacy</div>
            <div className="wf-btn full">/help/manage-url</div>
          </div>
        </div>
        <WFFooter/>
      </div>
    </WFBrowser>
  );
}

// ── 4.12 Help /help/manage-url
function WFHelp_M() {
  const sections = [
    '公開URLと管理用URLの違い',
    '管理用URLは再表示不可',
    '紛失時は編集/公開停止不可',
    '保存方法',
    'メール送信機能は現在なし',
    '外部共有禁止',
  ];
  return (
    <WFMobile title="管理URL FAQ" back>
      <div className="wf-m-pad">
        <h1 className="wf-h1">管理 URL の<br/>使い方</h1>
        <div className="wf-stack" style={{marginTop:14, gap:12}}>
          {sections.map((s,i)=>(
            <div key={s} className="wf-m-card">
              <div style={{fontSize:13, fontWeight:700, marginBottom:6}}>Q{i+1}. {s}</div>
              <div className="wf-line long"/><div className="wf-line long"/><div className="wf-line medium"/>
            </div>
          ))}
        </div>
        <WFFooter/>
      </div>
    </WFMobile>
  );
}
function WFHelp_PC() {
  const sections = [
    '公開URLと管理用URLの違い',
    '管理用URLは再表示不可',
    '紛失時は編集/公開停止不可',
    '保存方法',
    'メール送信機能は現在なし',
    '外部共有禁止',
  ];
  return (
    <WFBrowser url="https://vrc-photobook.com/help/manage-url">
      <div className="wf-pc-container narrow">
        <div className="wf-eyebrow">Help</div>
        <h1 className="wf-h1 lg">管理 URL の使い方</h1>
        <div className="wf-stack" style={{marginTop:20, gap:14}}>
          {sections.map((s,i)=>(
            <div key={s} className="wf-box">
              <div style={{fontSize:14, fontWeight:700, marginBottom:8}}>Q{i+1}. {s}</div>
              <div className="wf-line long"/><div className="wf-line long"/><div className="wf-line medium"/>
            </div>
          ))}
        </div>
        <WFFooter/>
      </div>
    </WFBrowser>
  );
}

// ── 4.13 Terms /terms
function WFTerms_M() {
  return (
    <WFMobile title="利用規約" back>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">Terms · 最終更新 2026.05.01</div>
        <h1 className="wf-h1">利用規約</h1>
        <div className="wf-note" style={{marginTop:14}}>Notice</div>
        <WFSection title="目次 (TOC)">
          <div className="wf-toc">
            {Array.from({length:9}).map((_,i)=> <span key={i}>Article {i+1}</span> )}
          </div>
        </WFSection>
        {Array.from({length:9}).map((_,i)=>(
          <div key={i} className="wf-m-card">
            <div style={{fontWeight:700, marginBottom:6}}>Article {i+1}</div>
            <div className="wf-line long"/><div className="wf-line long"/><div className="wf-line medium"/>
          </div>
        ))}
        <div className="wf-stack" style={{gap:8, marginTop:14}}>
          <div className="wf-btn full">/help/manage-url</div>
          <div className="wf-btn full">External: X</div>
        </div>
        <WFFooter/>
      </div>
    </WFMobile>
  );
}
function WFTerms_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/terms">
      <div className="wf-pc-container narrow">
        <div className="wf-eyebrow">Terms · 最終更新 2026.05.01</div>
        <h1 className="wf-h1 lg">利用規約</h1>
        <div className="wf-note" style={{marginTop:14}}>Notice</div>
        <div className="wf-box" style={{marginTop:18}}>
          <div className="wf-section-title">目次</div>
          <div className="wf-toc">
            {Array.from({length:9}).map((_,i)=> <span key={i}>Article {i+1}</span> )}
          </div>
        </div>
        {Array.from({length:9}).map((_,i)=>(
          <div key={i} className="wf-box" style={{marginTop:12}}>
            <div style={{fontWeight:700, marginBottom:8}}>Article {i+1}</div>
            <div className="wf-line long"/><div className="wf-line long"/><div className="wf-line medium"/>
          </div>
        ))}
        <WFFooter extra={<><span>/help/manage-url</span><span>X (external)</span></>}/>
      </div>
    </WFBrowser>
  );
}

// ── 4.14 Privacy /privacy
function WFPrivacy_M() {
  return (
    <WFMobile title="プライバシー" back>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">Privacy · 最終更新 2026.05.01</div>
        <h1 className="wf-h1">プライバシー<br/>ポリシー</h1>
        <div className="wf-note" style={{marginTop:14}}>Notice</div>
        <WFSection title="目次 (TOC)">
          <div className="wf-toc">
            {Array.from({length:10}).map((_,i)=> <span key={i}>Article {i+1}</span> )}
          </div>
        </WFSection>
        {Array.from({length:10}).map((_,i)=>(
          <div key={i} className="wf-m-card">
            <div style={{fontWeight:700, marginBottom:6}}>Article {i+1}</div>
            <div className="wf-line long"/><div className="wf-line long"/>
          </div>
        ))}
        <WFSection title="External services chips">
          <div className="wf-row" style={{gap:6, flexWrap:'wrap'}}>
            {['Cloudflare','Turnstile','R2','Sentry','PostHog'].map(c=> <span key={c} className="wf-badge">{c}</span> )}
          </div>
        </WFSection>
        <div className="wf-btn full">/terms</div>
        <WFFooter/>
      </div>
    </WFMobile>
  );
}
function WFPrivacy_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/privacy">
      <div className="wf-pc-container narrow">
        <div className="wf-eyebrow">Privacy · 最終更新 2026.05.01</div>
        <h1 className="wf-h1 lg">プライバシーポリシー</h1>
        <div className="wf-note" style={{marginTop:14}}>Notice</div>
        <div className="wf-box" style={{marginTop:18}}>
          <div className="wf-section-title">目次</div>
          <div className="wf-toc">
            {Array.from({length:10}).map((_,i)=> <span key={i}>Article {i+1}</span> )}
          </div>
        </div>
        {Array.from({length:10}).map((_,i)=>(
          <div key={i} className="wf-box" style={{marginTop:12}}>
            <div style={{fontWeight:700, marginBottom:8}}>Article {i+1}</div>
            <div className="wf-line long"/><div className="wf-line long"/>
          </div>
        ))}
        <div className="wf-box" style={{marginTop:14}}>
          <div className="wf-section-title">External services</div>
          <div className="wf-row" style={{gap:8, flexWrap:'wrap'}}>
            {['Cloudflare','Turnstile','R2','Sentry','PostHog'].map(c=> <span key={c} className="wf-badge">{c}</span> )}
          </div>
        </div>
        <WFFooter extra={<span>/terms</span>}/>
      </div>
    </WFBrowser>
  );
}

// ── Common ErrorState (unauthorized / not_found / gone / server_error)
function WFErrorStates() {
  const states = [
    { code:'401', title:'Unauthorized', msg:'draft / manage session がありません' },
    { code:'404', title:'Not found', msg:'photobook / slug が存在しません' },
    { code:'410', title:'Gone', msg:'公開ページは削除されました' },
    { code:'500', title:'Server error', msg:'API / env / network failure' },
  ];
  return (
    <div className="wf-root">
      <div style={{padding:'24px 24px', display:'grid', gridTemplateColumns:'1fr 1fr', gap:18}}>
        {states.map(s=>(
          <div key={s.code} className="wf-box" style={{padding:0, overflow:'hidden'}}>
            <div className="wf-pc-chrome" style={{height:30}}>
              <div className="wf-pc-lights"><span/><span/><span/></div>
              <div style={{flex:1, fontSize:11, color:'var(--wf-ink-3)', textAlign:'center'}}>ErrorState · {s.code}</div>
            </div>
            <div className="wf-error-shell">
              <div className="icon">{s.code}</div>
              <div style={{fontSize:18, fontWeight:800}}>{s.title}</div>
              <div style={{fontSize:12, color:'var(--wf-ink-3)', marginTop:6}}>{s.msg}</div>
              <div className="wf-row" style={{gap:8, marginTop:18, justifyContent:'center'}}>
                <div className="wf-btn sm">トップへ戻る</div>
                <div className="wf-btn sm primary">再試行</div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

Object.assign(window, {
  WFViewer_M, WFViewer_PC, WFReport_M, WFReport_PC,
  WFAbout_M, WFAbout_PC, WFHelp_M, WFHelp_PC,
  WFTerms_M, WFTerms_PC, WFPrivacy_M, WFPrivacy_PC, WFErrorStates,
});
