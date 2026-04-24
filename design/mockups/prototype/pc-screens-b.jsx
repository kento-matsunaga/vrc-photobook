// PC Screens 5-8

function PCComplete({ go }) {
  const [modal, setModal] = React.useState(false);
  const [copied, setCopied] = React.useState('');
  const copy = k => { setCopied(k); setTimeout(()=>setCopied(''), 1500); };
  return (
    <PCBrowser url="https://vrc-photobook.com/complete">
      <PCHeader go={go}/>
      <div className="pc-container narrow">
        <div className="pc-success-header">
          <div className="badge"><Icon.Check size={32}/></div>
          <h1>公開が完了しました</h1>
          <div className="sub">あなたのフォトブックが公開されました。</div>
        </div>

        <div className="pc-card" style={{display:'flex', alignItems:'center', gap: 18, marginBottom: 18}}>
          <Photo variant="a" style={{width: 110, height: 110, borderRadius: 10, flexShrink: 0}}/>
          <div style={{flex: 1}}>
            <div style={{fontSize: 19, fontWeight: 800}}>ミッドナイト ソーシャルクラブ</div>
            <div style={{display:'flex', alignItems:'center', gap: 8, marginTop: 10}}>
              <Av label="N" size="sm" variant="a"/>
              <span style={{fontSize: 12.5, fontWeight: 600}}>by nekoma</span>
            </div>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginTop: 4}}>公開日 2026.04.24</div>
          </div>
        </div>

        <div className="pc-complete">
          <div className="pc-card">
            <h3 className="pc-panel-title"><span className="ico"><Icon.Globe size={14}/></span> 公開URL</h3>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 10}}>誰でも閲覧できる一般公開リンクです。</div>
            <div className="pc-url"><Icon.Link size={14}/><span>https://vrc-photobook.com/p/abc123xyz</span></div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 10}}>
              <button className="btn btn-sm btn-ghost-cyan" onClick={()=>copy('p')}><Icon.Copy size={14}/> {copied==='p'?'コピー済':'URLをコピー'}</button>
              <button className="btn btn-sm" onClick={()=>go('pc-viewer')}><Icon.Eye size={14}/> 開く</button>
            </div>
          </div>

          <div className="pc-card" style={{background:'#FAF7FF', borderColor:'#E9DDFD'}}>
            <h3 className="pc-panel-title" style={{color:'var(--violet)'}}><span className="ico" style={{color:'var(--violet)'}}><Icon.Lock size={14}/></span> 管理URL</h3>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 10}}>編集・削除に必要なURLです。他人に共有しないでください。</div>
            <div className="pc-url" style={{color:'var(--violet)', background:'#fff', borderColor:'#E9DDFD'}}><Icon.Link size={14}/><span>https://vrc-photobook.com/manage/9f8e7d6c5b4a</span></div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 10}}>
              <button className="btn btn-sm" style={{borderColor:'#C4B5FD', color:'var(--violet)'}} onClick={()=>copy('m')}><Icon.Copy size={14}/> {copied==='m'?'コピー済':'管理URLをコピー'}</button>
              <button className="btn btn-sm" onClick={()=>go('pc-manage')}><Icon.Pencil size={14}/> 管理を開く</button>
            </div>
          </div>
        </div>

        <div className="warning-card" style={{marginTop: 18}}>
          <Icon.Warn/>
          <div><strong>この管理URLをなくすと、あとから編集・削除できません。</strong> 必ず保存してください。</div>
        </div>

        <div style={{display:'grid', gridTemplateColumns:'1fr 1fr 1fr', gap: 10, marginTop: 18}}>
          <button className="btn"><Icon.X size={14}/> Xで共有</button>
          <button className="btn"><Icon.Mail size={14}/> 管理URLを自分に送る</button>
          <button className="btn" onClick={()=>go('pc-viewer')}><Icon.Eye size={14}/> 閲覧ページを見る</button>
        </div>

        <div className="pc-cta" style={{marginTop: 22}}>
          <div className="eyebrow">今すぐみんなにシェアしよう</div>
          <button className="btn btn-primary btn-lg" style={{minWidth: 320}} onClick={()=>setModal(true)}>
            <Icon.Share size={14}/> 閲覧ページを見る
          </button>
        </div>

        <div style={{textAlign:'center', padding:'20px 0 40px', fontSize: 12, color:'var(--fg-3)'}}>
          公開・管理についての <span style={{color:'var(--teal)'}}>ヘルプ</span>
        </div>
      </div>

      {modal && (
        <div className="modal-backdrop">
          <div className="modal" style={{maxWidth: 420}}>
            <div style={{display:'flex', gap:12, alignItems:'flex-start', marginBottom: 14}}>
              <div style={{width: 40, height: 40, borderRadius: 10, background:'var(--red-soft)', display:'grid', placeItems:'center', color:'var(--red)'}}><Icon.Warn size={20}/></div>
              <div>
                <div style={{fontSize: 17, fontWeight: 800}}>管理URLを保存しましたか?</div>
                <div style={{fontSize: 12.5, color:'var(--fg-3)', marginTop: 6, lineHeight: 1.55}}>このURLをなくすと、あとから編集・削除できません。</div>
              </div>
            </div>
            <div className="pc-url" style={{color:'var(--violet)', fontSize: 11}}><Icon.Link size={12}/><span>vrc-photobook.com/manage/9f8e7d6c5b4a</span></div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 14}}>
              <button className="btn btn-sm" onClick={()=>setModal(false)}>もう一度確認</button>
              <button className="btn btn-sm btn-primary" onClick={()=>{setModal(false); go('pc-viewer');}}>保存済みです</button>
            </div>
          </div>
        </div>
      )}
    </PCBrowser>
  );
}

function PCViewer({ go }) {
  const pages = [
    { n:'01', t:'集合写真', s1:'みんなで集まって、', s2:'パシャリ。今日のはじまり!', v:'a' },
    { n:'02', t:'クラブでダンス!', s1:'音楽にあわせてノリノリで!', s2:'最高の時間を一緒に。', v:'c' },
    { n:'03', t:'まったりタイム', s1:'ちょっと一息。', s2:'おしゃべりも楽しいよね。', v:'b' },
  ];
  return (
    <PCBrowser url="https://vrc-photobook.com/view/midnight-social-club">
      <PCHeader go={go}/>
      <div className="pc-container">
        <div className="pc-2col">
          <div style={{display:'flex', flexDirection:'column', gap: 16}}>
            <div className="pc-view-hero">
              <Photo variant="a" className="cover"/>
              <div className="meta">
                <span className="chip" style={{alignSelf:'flex-start'}}>Event Photo Book</span>
                <div className="t">ミッドナイト<br/>ソーシャルクラブ</div>
                <div className="d">2026.04.24</div>
                <div className="by">
                  <Av label="N" size="sm" variant="a"/>
                  <span>nekoma</span>
                </div>
                <div className="stat">
                  <div className="row"><Icon.Image size={14}/> <span>128枚</span></div>
                  <div className="row"><Icon.World size={14}/> <span>Midnight Social Club</span></div>
                </div>
              </div>
            </div>

            <div className="pc-card">
              <h3 className="pc-panel-title"><span className="ico"><Icon.Book size={14}/></span> この日の思い出</h3>
              <div style={{fontSize: 13.5, color:'var(--fg-2)', lineHeight: 1.7}}>
                真夜中の街で集まった、特別なひととき。<br/>
                笑って、踊って、語り合った、かけがえのない思い出をこの一冊に。
              </div>
            </div>

            {pages.map(p=>(
              <div key={p.n} className="pc-page-row">
                <Photo variant={p.v} className="thumb"/>
                <div className="meta">
                  <div className="num">Page {p.n}</div>
                  <div className="t">{p.t}</div>
                  <div className="s">{p.s1}<br/>{p.s2}</div>
                </div>
                <Icon.Chev/>
              </div>
            ))}
          </div>

          <div className="pc-rail">
            <div className="pc-card">
              <h3 className="pc-panel-title"><span className="ico"><Icon.Send size={14}/></span> みんなにシェアしよう</h3>
              <div style={{display:'grid', gap: 10}}>
                <button className="btn" style={{height: 44, background:'#111827', color:'#fff', borderColor:'#111827'}}><Icon.X size={13}/> Xで共有</button>
                <button className="btn btn-ghost-cyan" style={{height: 44}}><Icon.Copy size={13}/> URLをコピー</button>
              </div>
              <div style={{fontSize: 11, color:'var(--fg-3)', textAlign:'center', marginTop: 12}}>このフォトブックは誰でも閲覧できます。</div>
            </div>

            <div className="pc-card" style={{background:'var(--teal-soft)', borderColor:'rgba(20,184,166,0.2)'}}>
              <div style={{display:'flex', alignItems:'center', gap: 8, marginBottom: 4}}>
                <Icon.Sparkle size={14} style={{color:'var(--teal)'}}/>
                <div style={{fontSize: 14, fontWeight: 800}}>ログイン不要で<br/>フォトブックを作る</div>
              </div>
              <div style={{fontSize: 11.5, color:'var(--fg-2)', lineHeight: 1.6, marginTop: 8, textAlign:'center'}}>
                思い出の写真をまとめて、<br/>かんたんにフォトブックを作成。<br/>すぐにシェアできます。
              </div>
              <div style={{display:'grid', gap: 8, marginTop: 14}}>
                <button className="btn btn-primary btn-lg" onClick={()=>go('pc-create')}><Icon.Sparkle size={14}/> 今すぐ作る</button>
                <button className="btn"><Icon.Image size={13}/> 作例を見る</button>
              </div>
            </div>
          </div>
        </div>

        <PCTrust/>
        <div style={{textAlign:'center', paddingBottom: 16, fontSize: 11, color:'var(--fg-4)'}}>
          <span className="tap" onClick={()=>go('pc-report')}>問題を報告</span>
        </div>
      </div>
    </PCBrowser>
  );
}

function PCManage({ go }) {
  const [sen, setSen] = React.useState(false);
  const [del, setDel] = React.useState(false);
  return (
    <PCBrowser url="https://vrc-photobook.com/m/ab12cd34ef56">
      <PCHeader go={go}/>
      <div className="pc-container">
        <div style={{padding:'8px 0 24px'}}>
          <h1 style={{fontSize: 28, fontWeight: 800, margin: 0}}>管理ページ</h1>
          <div style={{fontSize: 13, color:'var(--fg-3)', marginTop: 6}}>このフォトブックの編集・公開範囲変更・削除ができます。</div>
        </div>

        <div className="pc-manage">
          <div style={{display:'flex', flexDirection:'column', gap: 18}}>
            <div className="pc-card" style={{display:'flex', gap: 18}}>
              <Photo variant="a" style={{width: 140, height: 140, borderRadius: 12, flexShrink: 0}}/>
              <div style={{flex: 1, minWidth: 0}}>
                <div style={{display:'flex', alignItems:'center', gap: 10, marginBottom: 6}}>
                  <div style={{fontSize: 19, fontWeight: 800}}>ミッドナイト ソーシャルクラブ</div>
                  <span className="chip"><Icon.Lock size={10}/> 限定公開</span>
                </div>
                <div style={{fontSize: 13, color:'var(--fg-3)', lineHeight: 1.6}}>最高の夜を、もう一度。</div>
                <div style={{display:'flex', gap: 20, marginTop: 14, paddingTop: 14, borderTop:'1px solid var(--border-2)', fontSize: 12, color:'var(--fg-3)'}}>
                  <span><Icon.Users size={12}/> 作成者 nekoma</span>
                  <span><Icon.Calendar size={12}/> 作成日 2026.04.24</span>
                  <span><Icon.Refresh size={12}/> 最終更新 2026.04.24 23:47</span>
                </div>
              </div>
            </div>

            <div className="pc-card">
              <h3 className="pc-panel-title"><span className="ico"><Icon.Globe size={14}/></span> 公開URL</h3>
              <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 8}}>このURLからフォトブックを閲覧できます。</div>
              <div style={{display:'flex', gap: 8}}>
                <div className="pc-url"><span>https://vrc-photobook.com/p/midnight-social-club</span></div>
                <button className="btn btn-sm btn-ghost-cyan"><Icon.Copy size={13}/> コピー</button>
              </div>

              <div className="divider"/>

              <h3 className="pc-panel-title" style={{color:'var(--violet)'}}><span className="ico" style={{color:'var(--violet)'}}><Icon.Link size={14}/></span> 管理URL</h3>
              <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 8}}>このURLからフォトブックを管理できます。他人に共有しないでください。</div>
              <div style={{display:'flex', gap: 8}}>
                <div className="pc-url" style={{color:'var(--violet)'}}><span>https://vrc-photobook.com/m/ab12cd34ef56</span></div>
                <button className="btn btn-sm" style={{color:'var(--violet)', borderColor:'#E9DDFD'}}><Icon.Copy size={13}/> コピー</button>
              </div>
            </div>

            <div className="pc-card" style={{display:'flex', alignItems:'center', gap: 14}}>
              <div style={{width: 40, height: 40, borderRadius: 10, background:'var(--teal-soft)', color:'var(--teal)', display:'grid', placeItems:'center'}}><Icon.Shield size={18}/></div>
              <div style={{flex:1}}>
                <div style={{fontSize: 14, fontWeight: 800}}>センシティブな内容を含む</div>
                <div style={{fontSize: 11.5, color:'var(--fg-3)', marginTop: 2}}>ONにすると閲覧時に注意メッセージが表示されます。</div>
              </div>
              <div className={`switch tap ${sen?'on':''}`} onClick={()=>setSen(!sen)}/>
            </div>

            <div className="pc-card">
              <h3 className="pc-panel-title"><span className="ico"><Icon.Info size={14}/></span> 詳細情報</h3>
              <div style={{display:'grid', gridTemplateColumns:'repeat(4, 1fr)', gap: 16}}>
                <div>
                  <div style={{fontSize: 11, color:'var(--fg-3)', marginBottom: 4}}>ページ数</div>
                  <div style={{fontFamily:'var(--font-num)', fontSize: 16, fontWeight: 700}}>3</div>
                </div>
                <div>
                  <div style={{fontSize: 11, color:'var(--fg-3)', marginBottom: 4}}>写真枚数</div>
                  <div style={{fontFamily:'var(--font-num)', fontSize: 16, fontWeight: 700}}>23</div>
                </div>
                <div>
                  <div style={{fontSize: 11, color:'var(--fg-3)', marginBottom: 4}}>作成日</div>
                  <div style={{fontFamily:'var(--font-num)', fontSize: 13, fontWeight: 700}}>2026.04.24</div>
                </div>
                <div>
                  <div style={{fontSize: 11, color:'var(--fg-3)', marginBottom: 4}}>最終更新</div>
                  <div style={{fontFamily:'var(--font-num)', fontSize: 13, fontWeight: 700}}>23:47</div>
                </div>
              </div>
            </div>
          </div>

          <div style={{display:'flex', flexDirection:'column', gap: 10}}>
            <div className="row tap" onClick={()=>go('pc-edit')}>
              <div className="ico-wrap"><Icon.Pencil size={16}/></div>
              <div className="meta"><div className="t">編集する</div><div className="s">ページや写真を編集</div></div>
              <Icon.Chev/>
            </div>
            <div className="row tap">
              <div className="ico-wrap"><Icon.Users size={16}/></div>
              <div className="meta"><div className="t">公開範囲を変更</div><div className="s">公開状態を変更</div></div>
              <Icon.Chev/>
            </div>
            <div className="row tap">
              <div className="ico-wrap"><Icon.Refresh size={16}/></div>
              <div className="meta"><div className="t">X共有文を再生成</div><div className="s">共有文を作り直す</div></div>
              <Icon.Chev/>
            </div>
            <div className="row danger tap" onClick={()=>setDel(true)}>
              <div className="ico-wrap"><Icon.Trash size={16}/></div>
              <div className="meta"><div className="t" style={{color:'var(--red)'}}>削除する</div><div className="s">復元できません</div></div>
              <Icon.Chev/>
            </div>

            <div className="info-card" style={{background:'#FAF7FF', borderColor:'#E9DDFD', marginTop: 6}}>
              <Icon.Shield/>
              <div style={{color:'var(--fg-2)'}}>この管理URLを知っている人は編集・削除できます。他人に共有しないでください。</div>
            </div>
          </div>
        </div>

        <div style={{height: 40}}/>
      </div>

      {del && (
        <div className="modal-backdrop">
          <div className="modal" style={{maxWidth: 420}}>
            <div style={{display:'flex', gap:12, alignItems:'flex-start', marginBottom: 14}}>
              <div style={{width: 40, height: 40, borderRadius: 10, background:'var(--red-soft)', display:'grid', placeItems:'center', color:'var(--red)'}}><Icon.Trash size={20}/></div>
              <div>
                <div style={{fontSize: 17, fontWeight: 800}}>このフォトブックを削除しますか?</div>
                <div style={{fontSize: 12.5, color:'var(--fg-3)', marginTop: 6}}>削除すると <strong style={{color:'var(--red)'}}>復元できません</strong>。</div>
              </div>
            </div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 14}}>
              <button className="btn btn-sm" onClick={()=>setDel(false)}>キャンセル</button>
              <button className="btn btn-sm btn-danger" onClick={()=>setDel(false)}>削除する</button>
            </div>
          </div>
        </div>
      )}
    </PCBrowser>
  );
}

function PCReport({ go }) {
  const [reason, setReason] = React.useState('unauthorized');
  const reasons = [
    { k:'self', t:'自分が写っているため削除してほしい', i:<Icon.Users size={14}/> },
    { k:'unauthorized', t:'無断転載の可能性', i:<Icon.Shield size={14}/> },
    { k:'sensitive', t:'センシティブ設定が不足している', i:<Icon.EyeOff size={14}/> },
    { k:'harass', t:'嫌がらせ・晒し', i:<Icon.Warn size={14}/> },
    { k:'other', t:'その他', i:<Icon.Info size={14}/> },
  ];
  return (
    <PCBrowser url="https://vrc-photobook.com/report">
      <PCHeader go={go}/>
      <div className="pc-container narrow">
        <div style={{textAlign:'center', padding:'8px 0 22px'}}>
          <h1 style={{fontSize: 28, fontWeight: 800, margin: 0}}>問題を報告</h1>
          <div style={{fontSize: 13, color:'var(--fg-3)', marginTop: 8}}>
            権利侵害や不適切な内容を見つけた場合は、こちらからご連絡ください。
          </div>
        </div>

        <div className="pc-card" style={{marginBottom: 18}}>
          <label className="field-label"><Icon.Link size={11}/> 対象のURL <span style={{color:'var(--red)'}}>*</span></label>
          <input className="input" placeholder="例: https://vrc-photobook.com/p/xxxxx"/>
        </div>

        <div className="pc-card" style={{marginBottom: 18}}>
          <label className="field-label" style={{marginBottom: 12}}><Icon.Flag size={11}/> 報告理由 <span style={{color:'var(--red)'}}>*</span></label>
          <div style={{display:'grid', gap: 8}}>
            {reasons.map(r=>(
              <div key={r.k} className={`radio-card ${reason===r.k?'active':''}`} onClick={()=>setReason(r.k)} style={{padding: 13}}>
                <div className="radio-dot"/>
                <div style={{color:'var(--fg-3)'}}>{r.i}</div>
                <div style={{flex: 1, fontSize: 13.5}}>{r.t}</div>
              </div>
            ))}
          </div>
        </div>

        <div className="pc-card" style={{marginBottom: 18}}>
          <label className="field-label">詳細</label>
          <textarea className="textarea" placeholder="詳しい状況や理由をご記入ください(任意)" style={{minHeight: 110}}/>
          <div style={{textAlign:'right', fontSize: 10.5, color:'var(--fg-4)', marginTop: 4}}>0 / 500</div>
        </div>

        <div className="pc-card" style={{marginBottom: 22}}>
          <label className="field-label">連絡先(任意)</label>
          <input className="input" placeholder="メールアドレスまたはX ID"/>
          <div style={{fontSize: 11, color:'var(--fg-4)', marginTop: 6}}>
            ご記入いただいた場合、必要に応じてご連絡することがあります。
          </div>
        </div>

        <div style={{display:'flex', justifyContent:'center'}}>
          <button className="btn btn-primary btn-lg" style={{minWidth: 360}} onClick={()=>go('pc-viewer')}>
            <Icon.Send size={14}/> 送信する
          </button>
        </div>
        <div style={{textAlign:'center', marginTop: 10, fontSize: 11, color:'var(--fg-4)', paddingBottom: 40}}>
          いただいた報告は運営チームが内容を確認し、ガイドラインに基づいて適切に対応します。
        </div>
      </div>
    </PCBrowser>
  );
}

Object.assign(window, { PCComplete, PCViewer, PCManage, PCReport });
