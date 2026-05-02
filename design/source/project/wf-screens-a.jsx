// Wireframe screens — Public + Create + Draft exchange + Prepare

// ── 4.1 Landing /
function MockBook({ small }) {
  // illustrative wireframe of an open photobook (cover + spread)
  const h = small ? 200 : 280;
  return (
    <div style={{position:'relative', height: h}}>
      <div style={{
        position:'absolute', left:0, top:0,
        width:'58%', height:'100%',
        background:'var(--paper)',
        border:'1px solid var(--line)',
        borderRadius:'4px 14px 14px 4px',
        boxShadow:'var(--shadow-lg)',
        padding:20,
        display:'flex', flexDirection:'column', justifyContent:'flex-end',
        backgroundImage:'linear-gradient(135deg, var(--teal-50) 0%, var(--paper) 60%)',
      }}>
        <div className="wf-line title" style={{width:'70%'}}/>
        <div className="wf-line short" style={{height:6}}/>
        <div className="wf-line short" style={{height:6, width:'30%'}}/>
      </div>
      <div style={{
        position:'absolute', right:0, top: small? 20 : 30,
        width:'48%', height: '85%',
        display:'grid',
        gridTemplateColumns:'1fr 1fr',
        gridTemplateRows:'1fr 1fr',
        gap:6,
        padding:6,
        background:'var(--paper)',
        border:'1px solid var(--line)',
        borderRadius:'14px 4px 4px 14px',
        boxShadow:'var(--shadow-lg)',
      }}>
        <WFImg style={{gridColumn:'1 / span 2'}}/>
        <WFImg/>
        <WFImg/>
      </div>
    </div>
  );
}

function WFLanding_M() {
  return (
    <WFMobile>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">VRC PhotoBook</div>
        <h1 className="wf-h1" style={{marginTop:6}}>VRC写真を、<br/>Webフォトブックに。</h1>
        <p className="wf-sub" style={{marginTop:10}}>ログイン不要で、だれでもかんたんに。<br/>思い出をきれいにまとめて、すぐにシェア。</p>

        <div className="wf-stack" style={{marginTop:16, gap:10}}>
          <div className="wf-btn primary lg full">✦ 今すぐ作る</div>
          <div className="wf-btn lg full">🖼 作例を見る</div>
        </div>

        <div style={{marginTop:18}}>
          <MockBook small/>
        </div>

        <div className="wf-grid-4" style={{marginTop:14, gap:6}}>
          {Array.from({length:4}).map((_,i)=> <WFImg key={i} style={{aspectRatio:'1/1', borderRadius:6}}/> )}
        </div>

        <WFSection title="VRC PhotoBookの特徴">
          {[
            {ic:'user',  t:'ログイン不要',  d:'アカウント登録なしで、すぐフォトブック作成。'},
            {ic:'link',  t:'URLで共有',     d:'生成されたURLを共有するだけで、みんなが楽しめる。'},
            {ic:'edit',  t:'管理URLで編集', d:'管理URLがあれば、いつでも編集・追加・並べ替え。'},
            {ic:'calendar', t:'イベント・作品集', d:'イベント記録やおはツイ、作品集など様々な用途に。'},
          ].map((f,i)=>(
            <div key={i} className="wf-m-card">
              <div className="wf-row" style={{gap:12, alignItems:'flex-start'}}>
                <WFIcon name={f.ic}/>
                <div style={{flex:1}}>
                  <div style={{fontSize:13,fontWeight:700}}>{f.t}</div>
                  <div style={{fontSize:11.5, color:'var(--ink-3)', marginTop:4, lineHeight:1.55}}>{f.d}</div>
                </div>
              </div>
            </div>
          ))}
        </WFSection>

        <WFSection title="こんなシーンで活用できます">
          {[
            {ic:'calendar', t:'イベント',  d:'イベントの記録や、思い出のシーンをまとめます。'},
            {ic:'chat',     t:'おはツイ',  d:'おはツイの記録や、日々の交流をまとめます。'},
            {ic:'book',     t:'作品集',    d:'ワールドや写真作品を、美しくまとめます。'},
          ].map((u,i)=>(
            <div key={i} className="wf-m-card">
              <div className="wf-row" style={{gap:12}}>
                <WFIcon name={u.ic}/>
                <div style={{flex:1}}>
                  <div style={{fontSize:13,fontWeight:700}}>{u.t}</div>
                  <div style={{fontSize:11.5, color:'var(--ink-3)', marginTop:4}}>{u.d}</div>
                </div>
                <WFImg style={{width:64, height:48}}/>
              </div>
            </div>
          ))}
        </WFSection>

        <div className="wf-cta-band" style={{padding:'22px 18px'}}>
          <div style={{fontSize:14, fontWeight:700, marginBottom:10}}>さあ、あなたの思い出をカタチにしよう</div>
          <div className="wf-btn primary lg full">✦ 無料でフォトブックを作る</div>
          <div style={{fontSize:11, color:'var(--ink-3)', marginTop:8}}>ログイン不要・完全無料</div>
        </div>

        <div className="wf-note" style={{marginTop:18}}>
          <div>?reason=invalid_draft_token / invalid_manage_token 付きで戻る場合あり (現状専用UIなし)</div>
        </div>

        <WFFooter trust/>
      </div>
    </WFMobile>
  );
}
function WFLanding_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/">
      <div className="wf-pc-container">
        {/* HERO */}
        <div className="wf-grid-2" style={{alignItems:'center', gap:48, paddingTop:8}}>
          <div>
            <h1 className="wf-h1 lg">VRC写真を、<br/>Webフォトブックに。</h1>
            <p className="wf-sub" style={{marginTop:18, fontSize:15}}>
              ログイン不要で、だれでもかんたんに。<br/>
              思い出をきれいにまとめて、すぐにシェア。
            </p>
            <div className="wf-row" style={{gap:12, marginTop:26}}>
              <div className="wf-btn primary lg" style={{minWidth:180}}>✦ 今すぐ作る</div>
              <div className="wf-btn lg" style={{minWidth:180}}>🖼 作例を見る</div>
            </div>
            <div className="wf-anno" style={{marginTop:8}}>→ /create / → 作例 (LP内 anchor)</div>
          </div>
          <div><MockBook/></div>
        </div>

        {/* sample strip */}
        <div className="wf-grid-4" style={{marginTop:36, gridTemplateColumns:'repeat(5, 1fr)', gap:12}}>
          {Array.from({length:5}).map((_,i)=> <WFImg key={i} style={{aspectRatio:'4/3', borderRadius:10}}/> )}
        </div>

        {/* 特徴 */}
        <div className="wf-box lg" style={{marginTop:32}}>
          <div className="wf-section-title" style={{display:'flex', alignItems:'center'}}>
            <span style={{display:'inline-flex',gap:8,alignItems:'center'}}>
              <span style={{width:18,height:14,background:'linear-gradient(135deg,var(--teal-300),var(--teal-500))',borderRadius:2,display:'inline-block'}}/>
              VRC PhotoBookの特徴
            </span>
          </div>
          <div className="wf-grid-4">
            {[
              {ic:'user',  t:'ログイン不要',  d:'アカウント登録なしですぐフォトブックを作成できます。'},
              {ic:'link',  t:'URLで共有',     d:'生成されたURLを共有するだけで、みんなが写真を楽しめます。'},
              {ic:'edit',  t:'管理URLで編集', d:'管理用URLがあればいつでも編集・追加・並べ替えが可能です。'},
              {ic:'calendar', t:'イベント・おはツイ・作品集', d:'イベントの記録やおはツイ・作品集など様々な用途に活用できます。'},
            ].map((f,i)=>(
              <div key={i} className="wf-box" style={{padding:18}}>
                <WFIcon name={f.ic} style={{marginBottom:10}}/>
                <div style={{fontSize:13.5, fontWeight:700}}>{f.t}</div>
                <div style={{fontSize:11.5, color:'var(--ink-3)', marginTop:6, lineHeight:1.6}}>{f.d}</div>
              </div>
            ))}
          </div>
        </div>

        {/* 用途 */}
        <div className="wf-box lg" style={{marginTop:18}}>
          <div className="wf-section-title">こんなシーンで活用できます</div>
          <div className="wf-grid-3">
            {[
              {ic:'calendar', t:'イベント', d:'イベントの記録や、思い出のシーンをまとめます。'},
              {ic:'chat',     t:'おはツイ', d:'おはツイの記録や、日々の交流をまとめます。'},
              {ic:'book',     t:'作品集',   d:'ワールドや写真作品を、美しくまとめます。'},
            ].map((u,i)=>(
              <div key={i} className="wf-box" style={{padding:14, display:'flex', gap:12, alignItems:'center'}}>
                <WFIcon name={u.ic}/>
                <div style={{flex:1}}>
                  <div style={{fontSize:13,fontWeight:700}}>{u.t}</div>
                  <div style={{fontSize:11, color:'var(--ink-3)', marginTop:4, lineHeight:1.5}}>{u.d}</div>
                </div>
                <WFImg style={{width:90, height:60, borderRadius:8}}/>
              </div>
            ))}
          </div>
        </div>

        {/* bottom CTA band */}
        <div className="wf-cta-band">
          <div style={{fontSize:16, fontWeight:700, marginBottom:14, color:'var(--ink)'}}>
            ✦ さあ、あなたの思い出をカタチにしよう ✦
          </div>
          <div className="wf-btn primary lg" style={{minWidth:300}}>✦ 無料でフォトブックを作る</div>
          <div style={{fontSize:11.5, color:'var(--ink-3)', marginTop:10}}>ログイン不要・完全無料</div>
        </div>

        <WFFooter trust/>
      </div>
    </WFBrowser>
  );
}

// ── 4.2 Create /create
function WFCreate_M() {
  const types = ['memory','event','daily','portfolio','avatar','world','free'];
  return (
    <WFMobile title="作成入口" back>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">Step 1</div>
        <h1 className="wf-h1">どんなフォトブック<br/>を作りますか?</h1>
        <p className="wf-sub" style={{marginTop:8}}>タイプを選び、必要なら情報を入力。</p>

        <WFSection title="タイプ選択 (radio · 7種)">
          <div className="wf-stack" style={{gap:8}}>
            {types.map((t,i)=>(
              <div key={t} className={`wf-radio ${i===1?'active':''}`}>
                <div className="dot"/>
                <div style={{flex:1}}>{t}</div>
              </div>
            ))}
          </div>
        </WFSection>

        <WFSection title="任意入力">
          <label className="wf-label">タイトル (任意・最大100文字)</label>
          <input className="wf-input" placeholder="ミッドナイト ソーシャルクラブ"/>
          <div className="wf-counter">0 / 100</div>

          <label className="wf-label" style={{marginTop:12}}>作成者の表示名 (任意・最大50文字)</label>
          <input className="wf-input"/>
          <div className="wf-counter">0 / 50</div>
        </WFSection>

        <div className="wf-note">既定公開範囲: <strong>限定公開</strong> (公開設定で変更可)</div>

        <div style={{marginTop:14}}>
          <WFTurnstile done={false}/>
          <div className="wf-anno">action: photobook-create</div>
        </div>

        <div className="wf-stack" style={{marginTop:16, gap:8}}>
          <div className="wf-btn primary lg full disabled">編集を始める (Turnstile未完了で disabled)</div>
          <div className="wf-anno">成功: POST /api/photobooks → draft_edit_url_path へ replace</div>
        </div>

        <div className="wf-note warn" style={{marginTop:14}}>
          <div>Error states: invalid payload / Turnstile failed / unavailable / network / server error</div>
        </div>
      </div>
    </WFMobile>
  );
}
function WFCreate_PC() {
  const types = ['memory','event','daily','portfolio','avatar','world','free'];
  return (
    <WFBrowser url="https://vrc-photobook.com/create">
      <div className="wf-pc-container narrow">
        <div className="wf-eyebrow">Step 1 / 3</div>
        <h1 className="wf-h1">どんなフォトブックを作りますか?</h1>
        <p className="wf-sub" style={{marginTop:8}}>タイプを選び、必要なら情報を入力。</p>

        <div className="wf-box" style={{marginTop:22}}>
          <div className="wf-section-title">タイプ選択 (radio · 7種)</div>
          <div className="wf-grid-3">
            {types.map((t,i)=>(
              <div key={t} className={`wf-radio ${i===1?'active':''}`} style={{flexDirection:'column', alignItems:'flex-start', minHeight:80}}>
                <div className="wf-row" style={{justifyContent:'space-between', width:'100%'}}>
                  <div style={{fontWeight:700}}>{t}</div>
                  <div className="dot"/>
                </div>
                <div className="wf-line short"/>
              </div>
            ))}
          </div>
        </div>

        <div className="wf-box" style={{marginTop:18}}>
          <div className="wf-grid-2">
            <div>
              <label className="wf-label">タイトル (任意・最大100)</label>
              <input className="wf-input"/>
              <div className="wf-counter">0 / 100</div>
            </div>
            <div>
              <label className="wf-label">作成者の表示名 (任意・最大50)</label>
              <input className="wf-input"/>
              <div className="wf-counter">0 / 50</div>
            </div>
          </div>
          <div className="wf-note" style={{marginTop:14}}>既定公開範囲: 限定公開</div>
        </div>

        <div style={{marginTop:18}}>
          <WFTurnstile done={false}/>
        </div>

        <div className="wf-row" style={{justifyContent:'flex-end', marginTop:20}}>
          <div className="wf-btn primary lg disabled">編集を始める</div>
        </div>
        <div className="wf-anno" style={{textAlign:'right'}}>POST /api/photobooks → /draft/{'{token}'} へ replace</div>

        <WFFooter/>
      </div>
    </WFBrowser>
  );
}

// ── 4.3 Draft token exchange (route handler)
function WFDraftExchange() {
  return (
    <div className="wf-root">
      <div className="wf-route-card" style={{height:'100%'}}>
        <div className="inner">
          <div className="spinner" style={{margin:'0 auto 18px',width:32,height:32,border:'3px solid var(--wf-line)',borderTopColor:'var(--wf-ink)',borderRadius:'50%'}}/>
          <div style={{fontWeight:700, fontSize:13}}>/draft/{'{token}'}</div>
          <div className="wf-anno" style={{marginTop:6, fontStyle:'normal'}}>Route Handler — UI なし</div>
          <div className="wf-divider"/>
          <div style={{fontSize:11, color:'var(--wf-ink-2)', textAlign:'left', lineHeight:1.7}}>
            1. POST /api/auth/draft-session-exchange<br/>
            2. Set-Cookie: vrcpb_draft_{'{photobookId}'}<br/>
            3. Cache-Control: no-store<br/>
            4. ✓ → redirect /prepare/{'{photobookId}'}<br/>
            5. ✗ → redirect /?reason=invalid_draft_token
          </div>
        </div>
      </div>
    </div>
  );
}

// ── 4.4 Prepare /prepare/{id}
function WFPrepare_M() {
  return (
    <WFMobile title="写真を追加" back>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">Step 2 / 3</div>
        <h1 className="wf-h1">写真をまとめて追加</h1>
        <div className="wf-sub" style={{marginTop:8}}>
          JPEG/PNG/WebP · 最大10MB/枚 · 最大20枚<br/>HEIC/HEIF 未対応
        </div>

        <div style={{marginTop:14}}>
          <WFTurnstile done/>
          <div className="wf-anno">action: upload</div>
        </div>

        <WFSection title="ファイル選択">
          <div className="wf-box dashed" style={{textAlign:'center', padding:'24px 14px'}}>
            <div className="wf-btn primary">📎 写真を選択 (multiple)</div>
            <div className="wf-anno">Turnstile完了後に有効化</div>
          </div>
        </WFSection>

        <WFSection title="進捗パネル">
          <div className="wf-box" style={{padding:'10px 12px'}}>
            <div className="wf-row" style={{justifyContent:'space-between', fontSize:12}}>
              <span>完了 12 / 合計 18</span>
              <span style={{color:'var(--wf-ink-3)'}}>処理中 4 · 失敗 2</span>
            </div>
            <div className="wf-line long" style={{marginTop:8, height:6}}/>
            <div className="wf-anno">10分超で混雑 notice 表示</div>
          </div>
        </WFSection>

        <WFSection title="画像タイル grid (2列)">
          <div className="wf-grid-2">
            {[
              {l:'available'},{l:'processing', p:'70%'},
              {l:'uploading', p:'40%'},{l:'failed', f:true},
              {l:'queued'},{l:'completing', p:'90%'},
            ].map((x,i)=>(
              <div key={i} className={`wf-upload-tile ${x.f?'failed':''}`}>
                <WFImg label={x.l} style={{aspectRatio:'1/1'}}/>
                <div className="bar"><i style={{'--p': x.p||'0%'}}/></div>
                <div className="stat"><span>{x.l}</span><span>—</span></div>
              </div>
            ))}
          </div>
          <div className="wf-anno">states: queued / verifying / uploading / completing / processing / available / failed</div>
        </WFSection>
      </div>
      <div className="wf-m-stick-cta">
        <div className="wf-btn primary lg full">編集へ進む</div>
      </div>
    </WFMobile>
  );
}
function WFPrepare_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/prepare/abc123">
      <div className="wf-pc-container">
        <div className="wf-eyebrow">Step 2 / 3</div>
        <h1 className="wf-h1">写真をまとめて追加</h1>
        <p className="wf-sub" style={{marginTop:8}}>JPEG/PNG/WebP · 最大10MB/枚 · 最大20枚 · HEIC/HEIF 未対応</p>

        <div className="wf-grid-2-1" style={{marginTop:22, alignItems:'flex-start'}}>
          <div className="wf-stack lg">
            <div className="wf-box dashed" style={{padding:'36px', textAlign:'center'}}>
              <div className="wf-btn primary lg">📎 写真を選択 (multiple)</div>
              <div className="wf-anno">またはここにドラッグ&ドロップ · Turnstile完了後に有効</div>
            </div>

            <div>
              <div className="wf-section-title">画像タイル grid</div>
              <div className="wf-grid-4">
                {['available','processing','uploading','failed','queued','completing','available','available'].map((s,i)=>(
                  <div key={i} className={`wf-upload-tile ${s==='failed'?'failed':''}`}>
                    <WFImg label={s} style={{aspectRatio:'1/1'}}/>
                    <div className="bar"><i style={{'--p': s==='processing'?'70%':s==='uploading'?'40%':s==='completing'?'90%':'0%'}}/></div>
                    <div className="stat"><span>{s}</span><span>—</span></div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="wf-stack">
            <div className="wf-box">
              <div className="wf-section-title">進捗</div>
              <div className="wf-row" style={{justifyContent:'space-between', fontSize:13}}>
                <span>完了</span><span style={{fontWeight:700}}>12 / 18</span>
              </div>
              <div className="wf-line long" style={{height:6, marginTop:6}}/>
              <div className="wf-divider"/>
              <div className="wf-row" style={{justifyContent:'space-between', fontSize:12}}>
                <span style={{color:'var(--wf-ink-3)'}}>処理中</span><span>4</span>
              </div>
              <div className="wf-row" style={{justifyContent:'space-between', fontSize:12}}>
                <span style={{color:'var(--wf-ink-3)'}}>失敗</span><span>2</span>
              </div>
            </div>
            <WFTurnstile done/>
            <div className="wf-btn primary lg full">編集へ進む</div>
            <div className="wf-anno">POST /prepare/attach-images → /edit/{'{id}'}</div>
            <div className="wf-note">遅延 notice / 上限到達 (20枚) / reload復元 / Failed reasons (verification, rate limited, validation, upload, complete, network, unknown)</div>
          </div>
        </div>
      </div>
    </WFBrowser>
  );
}

Object.assign(window, { WFLanding_M, WFLanding_PC, WFCreate_M, WFCreate_PC, WFDraftExchange, WFPrepare_M, WFPrepare_PC });
