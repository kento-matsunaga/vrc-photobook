// Wireframe screens — Edit, Publish complete, Manage exchange/page

// ── 4.5 Edit /edit/{id}
function WFEdit_M() {
  return (
    <WFMobile title="編集ページ" back right={<div style={{fontSize:11,color:'var(--wf-ink-3)'}}>v.12</div>}>
      <div className="wf-m-pad">
        <div className="wf-note warn">
          <div><strong>conflict banner:</strong> 別端末で更新されました — [最新を取得]</div>
        </div>
        <div className="wf-note" style={{marginTop:8}}>処理中: 2 件 / 失敗: 1 件 (5秒polling)</div>

        <WFSection title="ページ一覧">
          <div className="wf-grid-3">
            {Array.from({length:6}).map((_,i)=>(
              <div key={i} className={`wf-page-tile ${i===0?'active':''}`}>
                <div className="num">P{String(i+1).padStart(2,'0')}</div>
                <WFImg style={{width:'100%',height:'100%'}}/>
              </div>
            ))}
          </div>
          <div className="wf-btn full" style={{marginTop:10}}>+ ページを追加</div>
        </WFSection>

        <WFSection title="PhotoGrid (P01)">
          <div className="wf-grid-2">
            {Array.from({length:4}).map((_,i)=>(
              <div key={i} className="wf-box" style={{padding:6}}>
                <WFImg style={{aspectRatio:'1/1'}}/>
                <input className="wf-input" placeholder="caption" style={{height:30, marginTop:6, fontSize:11}}/>
                <div className="wf-row" style={{justifyContent:'space-between', marginTop:6, fontSize:10, color:'var(--wf-ink-3)'}}>
                  <span>↑ ↓ ⇈ ⇊</span>
                  <span>表紙 · 削除</span>
                </div>
              </div>
            ))}
          </div>
          <div className="wf-anno">操作: caption保存 / 並び替え / 表紙設定・解除 / 削除</div>
        </WFSection>

        <WFSection title="1枚追加 fallback">
          <div className="wf-box dashed" style={{textAlign:'center', padding:'14px'}}>
            <div className="wf-btn sm">📎 ファイル選択</div>
            <div style={{marginTop:8}}><WFTurnstile done/></div>
            <div className="wf-btn sm primary" style={{marginTop:8}}>アップロード開始</div>
          </div>
        </WFSection>

        <WFSection title="CoverPanel">
          <div className="wf-box">
            <div className="wf-row" style={{gap:12}}>
              <WFImg label="COVER" style={{width:80,height:80}}/>
              <div style={{flex:1}}>
                <div className="wf-line title medium"/>
                <input className="wf-input" placeholder="cover title" style={{marginTop:6}}/>
                <div className="wf-btn sm" style={{marginTop:6}}>表紙を解除</div>
              </div>
            </div>
          </div>
        </WFSection>

        <WFSection title="PublishSettingsPanel">
          <div className="wf-stack">
            <div><label className="wf-label">タイトル</label><input className="wf-input"/></div>
            <div><label className="wf-label">説明</label><textarea className="wf-textarea"/></div>
            <div className="wf-grid-2">
              <div><label className="wf-label">タイプ</label><input className="wf-input" defaultValue="event"/></div>
              <div><label className="wf-label">レイアウト</label><input className="wf-input"/></div>
            </div>
            <div><label className="wf-label">opening style</label><input className="wf-input"/></div>
            <div><label className="wf-label">公開範囲</label>
              <div className="wf-stack" style={{gap:6}}>
                <div className="wf-radio"><div className="dot"/>公開</div>
                <div className="wf-radio active"><div className="dot"/>限定公開</div>
                <div className="wf-radio"><div className="dot"/>非公開</div>
              </div>
            </div>
            <div className="wf-check on"><div className="box">✓</div><div>権利・配慮について確認しました</div></div>
            <div className="wf-row" style={{gap:8}}>
              <div className="wf-btn">下書き保存</div>
              <div className="wf-btn primary" style={{flex:1}}>公開する</div>
            </div>
          </div>
        </WFSection>

        <div className="wf-note warn">
          <div>Publish precondition: rights / not draft / empty title / empty creator / rate limited</div>
        </div>
      </div>
    </WFMobile>
  );
}
function WFEdit_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/edit/abc123">
      <div className="wf-pc-container" style={{maxWidth:1280}}>
        <div className="wf-row" style={{justifyContent:'space-between', marginBottom:14}}>
          <div>
            <div className="wf-eyebrow">Step 3 / 3</div>
            <h1 className="wf-h2" style={{marginTop:4}}>編集ページ <span style={{fontSize:12,color:'var(--wf-ink-3)',fontWeight:400,marginLeft:8}}>version 12</span></h1>
          </div>
          <div className="wf-row" style={{gap:8}}>
            <div className="wf-btn">下書き保存</div>
            <div className="wf-btn primary">公開する</div>
          </div>
        </div>

        <div className="wf-note warn" style={{marginBottom:12}}>
          <div><strong>Conflict banner:</strong> 別端末で更新されました [最新を取得]</div>
        </div>

        <div className="wf-grid-1-2-1">
          {/* LEFT: page list + cover */}
          <div className="wf-stack">
            <div className="wf-box">
              <div className="wf-section-title">ページ一覧</div>
              <div className="wf-grid-2">
                {Array.from({length:6}).map((_,i)=>(
                  <div key={i} className={`wf-page-tile ${i===0?'active':''}`}>
                    <div className="num">P{String(i+1).padStart(2,'0')}</div>
                    <WFImg style={{width:'100%',height:'100%'}}/>
                  </div>
                ))}
              </div>
              <div className="wf-btn full sm" style={{marginTop:10}}>+ ページを追加</div>
            </div>
            <div className="wf-box">
              <div className="wf-section-title">CoverPanel</div>
              <WFImg label="COVER" style={{aspectRatio:'1/1', marginBottom:8}}/>
              <input className="wf-input" placeholder="cover title"/>
              <div className="wf-btn sm full" style={{marginTop:8}}>表紙を解除</div>
            </div>
          </div>

          {/* CENTER: PhotoGrid */}
          <div className="wf-stack">
            <div className="wf-box">
              <div className="wf-row" style={{justifyContent:'space-between'}}>
                <div className="wf-section-title" style={{margin:0}}>PhotoGrid · P01</div>
                <span className="wf-badge">処理中 2 · 失敗 1</span>
              </div>
              <div className="wf-grid-3" style={{marginTop:12}}>
                {Array.from({length:6}).map((_,i)=>(
                  <div key={i} className="wf-box" style={{padding:6}}>
                    <WFImg style={{aspectRatio:'1/1'}}/>
                    <input className="wf-input" placeholder="caption" style={{height:30, marginTop:6, fontSize:11}}/>
                    <div className="wf-row" style={{justifyContent:'space-between', marginTop:6, fontSize:10, color:'var(--wf-ink-3)'}}>
                      <span>↑↓⇈⇊</span><span>表紙·削除</span>
                    </div>
                  </div>
                ))}
              </div>
              <div className="wf-anno">空状態: [最初のページを追加]</div>
            </div>

            <div className="wf-box dashed">
              <div className="wf-section-title">1枚追加 fallback</div>
              <div className="wf-row" style={{gap:10}}>
                <div className="wf-btn sm">📎 ファイル選択</div>
                <div style={{flex:1}}><WFTurnstile done/></div>
                <div className="wf-btn sm primary">アップロード開始</div>
              </div>
            </div>
          </div>

          {/* RIGHT: PublishSettingsPanel */}
          <div className="wf-box">
            <div className="wf-section-title">PublishSettingsPanel</div>
            <div className="wf-stack">
              <div><label className="wf-label">タイトル</label><input className="wf-input"/></div>
              <div><label className="wf-label">説明</label><textarea className="wf-textarea" style={{minHeight:60}}/></div>
              <div className="wf-grid-2">
                <div><label className="wf-label">タイプ</label><input className="wf-input"/></div>
                <div><label className="wf-label">レイアウト</label><input className="wf-input"/></div>
              </div>
              <div><label className="wf-label">opening style</label><input className="wf-input"/></div>
              <div>
                <label className="wf-label">公開範囲</label>
                <div className="wf-stack" style={{gap:6}}>
                  <div className="wf-radio"><div className="dot"/>公開</div>
                  <div className="wf-radio active"><div className="dot"/>限定公開</div>
                  <div className="wf-radio"><div className="dot"/>非公開</div>
                </div>
              </div>
              <div className="wf-check on"><div className="box">✓</div><div>権利・配慮について確認</div></div>
              <div className="wf-btn primary full">公開する</div>
              <div className="wf-anno">precondition error: rights / not draft / empty title / empty creator / rate limited</div>
            </div>
          </div>
        </div>
      </div>
    </WFBrowser>
  );
}

// ── 4.6 Publish complete view (within edit state)
function WFComplete_M() {
  return (
    <WFMobile title="公開完了" back={false}>
      <div className="wf-m-pad">
        <div className="wf-eyebrow">Status: PUBLISHED</div>
        <h1 className="wf-h1">フォトブックを<br/>公開しました</h1>

        <WFSection title="公開 URL">
          <div className="wf-box" style={{padding:'10px 12px'}}>
            <div className="wf-row" style={{gap:8}}>
              <span style={{flex:1, fontFamily:'ui-monospace,Menlo,monospace', fontSize:11, color:'var(--wf-ink-3)'}}>https://vrc-photobook.com/p/&#123;slug&#125;</span>
              <div className="wf-btn sm">コピー</div>
            </div>
          </div>
        </WFSection>

        <div className="wf-note warn">
          <div><strong>管理 URL は再表示できません。</strong> 必ず保存してください。</div>
        </div>

        <WFSection title="管理 URL (再表示不可)">
          <div className="wf-box" style={{padding:'10px 12px', borderStyle:'dashed'}}>
            <div className="wf-row" style={{gap:8}}>
              <span style={{flex:1, fontFamily:'ui-monospace,Menlo,monospace', fontSize:11, color:'var(--wf-ink-3)'}}>https://vrc-photobook.com/manage/token/&#123;raw&#125;</span>
              <div className="wf-btn sm">コピー</div>
            </div>
          </div>
        </WFSection>

        <WFSection title="保存方法 panel">
          <div className="wf-stack" style={{gap:8}}>
            <div className="wf-btn full">.txt ファイルとして保存</div>
            <div className="wf-btn full">自分宛にメールを書く (mailto:)</div>
            <div className="wf-check"><div className="box"/><div>管理用 URL を安全な場所に保存しました</div></div>
          </div>
          <div className="wf-anno">localStorage / sessionStorage には保存しない</div>
        </WFSection>

        <WFSection title="Actions">
          <div className="wf-stack" style={{gap:8}}>
            <div className="wf-btn primary full">公開ページを開く → /p/&#123;slug&#125; (新規タブ)</div>
            <div className="wf-btn full">編集ページに戻る → /</div>
          </div>
        </WFSection>

        <div className="wf-note">保存未確認 reminder · FAQ: /help/manage-url</div>
      </div>
    </WFMobile>
  );
}
function WFComplete_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/edit/abc123 (state: published)">
      <div className="wf-pc-container narrow">
        <div className="wf-eyebrow">Status: PUBLISHED</div>
        <h1 className="wf-h1">フォトブックを公開しました</h1>

        <div className="wf-grid-2" style={{marginTop:22}}>
          <div className="wf-box">
            <div className="wf-section-title">公開 URL</div>
            <div className="wf-input" style={{display:'flex',alignItems:'center',color:'var(--wf-ink-3)',fontFamily:'monospace',fontSize:11}}>https://vrc-photobook.com/p/&#123;slug&#125;</div>
            <div className="wf-row" style={{gap:8, marginTop:10}}>
              <div className="wf-btn sm" style={{flex:1}}>コピー</div>
              <div className="wf-btn sm primary" style={{flex:1}}>新規タブで開く</div>
            </div>
          </div>
          <div className="wf-box" style={{borderStyle:'dashed'}}>
            <div className="wf-section-title">管理 URL (再表示不可)</div>
            <div className="wf-input" style={{display:'flex',alignItems:'center',color:'var(--wf-ink-3)',fontFamily:'monospace',fontSize:11}}>.../manage/token/&#123;raw&#125;</div>
            <div className="wf-row" style={{gap:8, marginTop:10}}>
              <div className="wf-btn sm" style={{flex:1}}>コピー</div>
              <div className="wf-btn sm" style={{flex:1}}>.txt 保存</div>
              <div className="wf-btn sm" style={{flex:1}}>mailto:</div>
            </div>
          </div>
        </div>

        <div className="wf-note warn" style={{marginTop:18}}>
          <div><strong>管理 URL は再表示できません。</strong> 必ず保存してください。</div>
        </div>

        <div className="wf-box" style={{marginTop:18}}>
          <div className="wf-section-title">保存確認</div>
          <div className="wf-check"><div className="box"/><div>管理用 URL を安全な場所に保存しました</div></div>
          <div className="wf-anno" style={{marginTop:6}}>未確認時は reminder 表示</div>
        </div>

        <div className="wf-row" style={{justifyContent:'flex-end', gap:10, marginTop:18}}>
          <div className="wf-btn">編集ページに戻る → /</div>
          <div className="wf-btn primary">公開ページを開く</div>
        </div>

        <WFFooter extra={<span>FAQ: /help/manage-url</span>}/>
      </div>
    </WFBrowser>
  );
}

// ── 4.7 Manage token exchange
function WFManageExchange() {
  return (
    <div className="wf-root">
      <div className="wf-route-card" style={{height:'100%'}}>
        <div className="inner">
          <div className="spinner" style={{margin:'0 auto 18px',width:32,height:32,border:'3px solid var(--wf-line)',borderTopColor:'var(--wf-ink)',borderRadius:'50%'}}/>
          <div style={{fontWeight:700, fontSize:13}}>/manage/token/&#123;token&#125;</div>
          <div className="wf-anno" style={{marginTop:6, fontStyle:'normal'}}>Route Handler — UI なし</div>
          <div className="wf-divider"/>
          <div style={{fontSize:11, color:'var(--wf-ink-2)', textAlign:'left', lineHeight:1.7}}>
            1. POST /api/auth/manage-session-exchange<br/>
            2. Set-Cookie: vrcpb_manage_&#123;photobookId&#125;<br/>
            3. Cache-Control: no-store<br/>
            4. ✓ → redirect /manage/&#123;photobookId&#125;<br/>
            5. ✗ → redirect /?reason=invalid_manage_token
          </div>
        </div>
      </div>
    </div>
  );
}

// ── 4.8 Manage page
function WFManage_M() {
  return (
    <WFMobile title="管理ページ" back>
      <div className="wf-m-pad">
        <div className="wf-row" style={{gap:8, marginBottom:6}}>
          <span className="wf-badge dark">PUBLISHED</span>
          <span className="wf-badge">hidden</span>
        </div>
        <div className="wf-line h1"/>

        <div className="wf-note warn" style={{marginTop:10}}>
          <div>HiddenByOperatorBanner: 運営により一時非表示</div>
        </div>

        <WFSection title="公開 URL">
          <div className="wf-box" style={{padding:'10px 12px'}}>
            <div style={{fontFamily:'monospace',fontSize:11,color:'var(--wf-ink-3)'}}>/p/&#123;slug&#125;</div>
          </div>
          <div className="wf-anno">未公開なら dashed empty state</div>
        </WFSection>

        <WFSection title="情報 panel">
          <div className="wf-box">
            <div className="wf-row" style={{justifyContent:'space-between',padding:'6px 0'}}><span style={{fontSize:12,color:'var(--wf-ink-3)'}}>公開写真数</span><span style={{fontWeight:700}}>23</span></div>
            <div className="wf-divider" style={{margin:'4px 0'}}/>
            <div className="wf-row" style={{justifyContent:'space-between',padding:'6px 0'}}><span style={{fontSize:12,color:'var(--wf-ink-3)'}}>公開設定</span><span style={{fontWeight:700}}>限定公開</span></div>
            <div className="wf-divider" style={{margin:'4px 0'}}/>
            <div className="wf-row" style={{justifyContent:'space-between',padding:'6px 0'}}><span style={{fontSize:12,color:'var(--wf-ink-3)'}}>管理リンク version</span><span style={{fontWeight:700}}>v1</span></div>
            <div className="wf-divider" style={{margin:'4px 0'}}/>
            <div className="wf-row" style={{justifyContent:'space-between',padding:'6px 0'}}><span style={{fontSize:12,color:'var(--wf-ink-3)'}}>公開日時</span><span style={{fontWeight:700}}>2026.04.24</span></div>
          </div>
        </WFSection>

        <WFSection title="管理リンク再発行 panel">
          <div className="wf-box dashed">
            <div className="wf-line short"/>
            <div className="wf-line medium"/>
            <div className="wf-btn full disabled" style={{marginTop:10}}>再発行 (後日対応)</div>
          </div>
        </WFSection>

        <WFFooter/>
      </div>
    </WFMobile>
  );
}
function WFManage_PC() {
  return (
    <WFBrowser url="https://vrc-photobook.com/manage/abc123">
      <div className="wf-pc-container">
        <div className="wf-row" style={{gap:8, marginBottom:6}}>
          <span className="wf-badge dark">PUBLISHED</span>
          <span className="wf-badge">hidden</span>
        </div>
        <h1 className="wf-h1">管理ページ</h1>

        <div className="wf-note warn" style={{marginTop:14}}>
          <div>HiddenByOperatorBanner: 運営により一時非表示</div>
        </div>

        <div className="wf-grid-2-1" style={{marginTop:18}}>
          <div className="wf-stack">
            <div className="wf-box">
              <div className="wf-section-title">公開 URL</div>
              <div style={{fontFamily:'monospace',fontSize:12,color:'var(--wf-ink-3)'}}>https://vrc-photobook.com/p/&#123;slug&#125;</div>
              <div className="wf-anno">未公開: dashed empty state</div>
            </div>
            <div className="wf-box">
              <div className="wf-section-title">情報 panel</div>
              <div className="wf-grid-4">
                <div><div style={{fontSize:11,color:'var(--wf-ink-3)'}}>公開写真</div><div style={{fontWeight:700,fontSize:18}}>23</div></div>
                <div><div style={{fontSize:11,color:'var(--wf-ink-3)'}}>公開設定</div><div style={{fontWeight:700,fontSize:13}}>限定公開</div></div>
                <div><div style={{fontSize:11,color:'var(--wf-ink-3)'}}>管理 ver.</div><div style={{fontWeight:700,fontSize:13}}>v1</div></div>
                <div><div style={{fontSize:11,color:'var(--wf-ink-3)'}}>公開日時</div><div style={{fontWeight:700,fontSize:13}}>2026.04.24</div></div>
              </div>
            </div>
          </div>
          <div className="wf-stack">
            <div className="wf-box dashed">
              <div className="wf-section-title">管理リンク再発行</div>
              <div className="wf-line short"/>
              <div className="wf-line medium"/>
              <div className="wf-btn full disabled" style={{marginTop:10}}>再発行 (後日対応)</div>
            </div>
            <div className="wf-note">copy action は ManagePanel にはない (現状は表示のみ)</div>
          </div>
        </div>

        <WFFooter/>
      </div>
    </WFBrowser>
  );
}

Object.assign(window, { WFEdit_M, WFEdit_PC, WFComplete_M, WFComplete_PC, WFManageExchange, WFManage_M, WFManage_PC });
