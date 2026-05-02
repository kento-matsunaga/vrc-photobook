// Screens 5-8: Complete, Viewer, Manage, Report (Light theme)

function Complete({ go }) {
  const [confirmLeave, setConfirmLeave] = React.useState(false);
  const [copied, setCopied] = React.useState('');
  const copy = (k) => { setCopied(k); setTimeout(()=>setCopied(''), 1500); };
  return (
    <PhoneShell>
      <TopBar right={<div className="icon-btn tap"><Icon.Menu/></div>}/>
      <div className="pb-scroll" style={{paddingBottom: 30}}>
        <div style={{padding: '20px 20px 0', textAlign:'center'}}>
          <div style={{
            margin:'0 auto 12px', width: 56, height: 56, borderRadius: '50%',
            background:'var(--teal-soft)', border:'1px solid rgba(20,184,166,0.3)',
            display:'grid', placeItems:'center', color:'var(--teal)',
          }}>
            <Icon.Check size={28}/>
          </div>
          <div style={{fontSize: 22, fontWeight: 800, letterSpacing:'-0.01em'}}>公開が完了しました</div>
          <div style={{color:'var(--fg-3)', fontSize: 13, marginTop: 6}}>あなたのフォトブックが公開されました。</div>
        </div>

        <div style={{padding: '16px 16px 0'}}>
          <div className="card" style={{display:'flex', gap: 12, alignItems:'center'}}>
            <Photo variant="a" style={{width: 80, height: 80, borderRadius: 10, flexShrink: 0}}/>
            <div style={{flex:1, minWidth: 0}}>
              <div style={{fontSize: 15, fontWeight: 800, lineHeight: 1.25}}>ミッドナイト<br/>ソーシャルクラブ</div>
              <div style={{display:'flex', alignItems:'center', gap: 6, marginTop: 8}}>
                <Av label="N" size="sm" variant="a"/>
                <div style={{fontSize: 11.5, fontWeight: 600}}>by nekoma</div>
              </div>
              <div style={{fontSize: 11, color:'var(--fg-3)', marginTop: 4}}>公開日 2026.04.24</div>
            </div>
          </div>
        </div>

        {/* Public URL */}
        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Globe size={14}/> 公開URL</div></div>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 10}}>誰でも閲覧できる一般公開リンクです。</div>
            <UrlRow url="https://vrc-photobook.com/p/abc123xyz"/>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 10}}>
              <button className="btn btn-sm btn-ghost-cyan" onClick={()=>copy('pub')}>
                <Icon.Copy size={14}/> {copied==='pub' ? 'コピー済' : 'URLをコピー'}
              </button>
              <button className="btn btn-sm" onClick={()=>go('viewer')}><Icon.Eye size={14}/> 開く</button>
            </div>
          </div>
        </div>

        {/* Management URL */}
        <div style={{padding: '14px 16px 0'}}>
          <div className="card" style={{background:'#FAF7FF', borderColor:'#E9DDFD'}}>
            <div className="sect-head">
              <div className="title" style={{color: 'var(--violet)'}}><Icon.Lock size={14}/> 管理URL</div>
            </div>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 10}}>編集・削除などの管理に必要なURLです。他人に共有しないでください。</div>
            <div className="url-pill" style={{color: 'var(--violet)', background:'#fff', borderColor:'#E9DDFD'}}>
              <Icon.Link size={14}/>
              <span style={{flex:1, overflow:'hidden', textOverflow:'ellipsis'}}>vrc-photobook.com/manage/9f8e7d6c5b4a</span>
            </div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 10}}>
              <button className="btn btn-sm" style={{borderColor:'#C4B5FD', color:'var(--violet)'}} onClick={()=>copy('mng')}>
                <Icon.Copy size={14}/> {copied==='mng'?'コピー済':'管理URLをコピー'}
              </button>
              <button className="btn btn-sm" onClick={()=>go('manage')}>
                <Icon.Pencil size={14}/> 管理を開く
              </button>
            </div>
            <div className="warning-card" style={{marginTop: 12}}>
              <Icon.Warn/>
              <div><strong>この管理URLをなくすと、あとから編集・削除できません。</strong> 必ず保存してください。</div>
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="sect-head"><div className="title"><Icon.Sparkle size={14}/> その他のアクション</div></div>
          <div style={{display:'grid', gridTemplateColumns:'1fr 1fr 1fr', gap: 8}}>
            <button className="btn btn-sm" style={{flexDirection:'column', height: 64, fontSize: 11, gap: 4}}><Icon.X size={14}/> Xで共有</button>
            <button className="btn btn-sm" style={{flexDirection:'column', height: 64, fontSize: 10.5, gap: 4, lineHeight: 1.2}}><Icon.Mail size={14}/> 管理URLを<br/>自分に送る</button>
            <button className="btn btn-sm" style={{flexDirection:'column', height: 64, fontSize: 11, gap: 4}} onClick={()=>go('viewer')}><Icon.Eye size={14}/> 閲覧ページ</button>
          </div>
        </div>

        <div style={{padding: '16px 16px 0'}}>
          <div className="cta-block">
            <div className="eyebrow">今すぐみんなにシェアしよう</div>
            <button className="btn btn-primary btn-lg btn-block tap" onClick={()=>setConfirmLeave(true)}>
              <Icon.Share size={14}/> 閲覧ページを見る
            </button>
          </div>
        </div>

        <div style={{textAlign:'center', padding: '14px 0 20px', fontSize: 11, color:'var(--fg-3)'}}>
          公開・管理についての <span style={{color:'var(--teal)'}}>ヘルプ</span>
        </div>
      </div>

      {confirmLeave && (
        <div className="modal-backdrop">
          <div className="modal">
            <div style={{display:'flex', gap:10, alignItems:'flex-start', marginBottom: 14}}>
              <div style={{width: 36, height:36, borderRadius: 10, background:'var(--red-soft)', display:'grid', placeItems:'center', color:'var(--red)', flexShrink:0}}>
                <Icon.Warn size={18}/>
              </div>
              <div>
                <div style={{fontSize: 16, fontWeight: 800}}>管理URLを保存しましたか?</div>
                <div style={{fontSize: 12, color:'var(--fg-3)', marginTop: 6, lineHeight: 1.55}}>このURLをなくすと、あとから編集・削除できません。</div>
              </div>
            </div>
            <div className="url-pill" style={{color:'var(--violet)', fontSize: 10.5}}>
              <Icon.Link size={12}/>
              <span style={{flex:1, overflow:'hidden', textOverflow:'ellipsis'}}>vrc-photobook.com/manage/9f8e7d6c5b4a</span>
            </div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 14}}>
              <button className="btn btn-sm" onClick={()=>setConfirmLeave(false)}>もう一度確認</button>
              <button className="btn btn-sm btn-primary" onClick={()=>{setConfirmLeave(false); go('viewer');}}>保存済みです</button>
            </div>
          </div>
        </div>
      )}
    </PhoneShell>
  );
}

function Viewer({ go }) {
  const pages = [
    { n:'01', t:'集合写真', s1:'みんなで集まって、', s2:'パシャリ。今日のはじまり!', v:'a' },
    { n:'02', t:'クラブでダンス!', s1:'音楽にあわせてノリノリで!', s2:'最高の時間を一緒に。', v:'c' },
    { n:'03', t:'まったりタイム', s1:'ちょっと一息。', s2:'おしゃべりも楽しいよね。', v:'b' },
  ];
  return (
    <PhoneShell>
      <TopBar/>
      <div className="pb-scroll" style={{paddingBottom: 20}}>
        {/* Hero card */}
        <div style={{padding: '16px 16px 0'}}>
          <div className="card" style={{padding: 12, display:'grid', gridTemplateColumns:'1.3fr 1fr', gap: 12, alignItems:'center'}}>
            <Photo variant="a" style={{aspectRatio:'4/3', borderRadius: 10}} label="cover"/>
            <div>
              <span className="chip" style={{marginBottom: 8}}>Event Photo Book</span>
              <div style={{fontSize: 16, fontWeight: 800, lineHeight: 1.2, marginTop: 8}}>ミッドナイト<br/>ソーシャルクラブ</div>
              <div style={{fontSize: 11, color:'var(--fg-3)', marginTop: 10, fontFamily:'var(--font-num)'}}>2026.04.24</div>
              <div style={{display:'flex', alignItems:'center', gap: 6, marginTop: 6}}>
                <Av label="N" size="sm" variant="a"/>
                <span style={{fontSize: 11.5, fontWeight: 600}}>nekoma</span>
              </div>
            </div>
          </div>
        </div>

        {/* Memories card */}
        <div style={{padding: '12px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Book size={14}/> この日の思い出</div></div>
            <div style={{fontSize: 13, color:'var(--fg-2)', lineHeight: 1.6}}>
              真夜中の街で集まった、特別なひととき。<br/>
              笑って、踊って、語り合った、かけがえのない思い出をこの一冊に。
            </div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 12, paddingTop: 12, borderTop:'1px dashed var(--border)'}}>
              <div style={{display:'flex', alignItems:'center', gap: 8}}>
                <div style={{width: 32, height: 32, borderRadius: 8, background:'var(--teal-soft)', color:'var(--teal)', display:'grid', placeItems:'center'}}><Icon.Image size={14}/></div>
                <div>
                  <div style={{fontSize: 10.5, color:'var(--fg-3)'}}>写真枚数</div>
                  <div style={{fontSize: 13, fontWeight: 700}}>128枚</div>
                </div>
              </div>
              <div style={{display:'flex', alignItems:'center', gap: 8}}>
                <div style={{width: 32, height: 32, borderRadius: 8, background:'var(--teal-soft)', color:'var(--teal)', display:'grid', placeItems:'center'}}><Icon.World size={14}/></div>
                <div style={{minWidth: 0}}>
                  <div style={{fontSize: 10.5, color:'var(--fg-3)'}}>撮影ワールド</div>
                  <div style={{fontSize: 12.5, fontWeight: 700, overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap'}}>Midnight Social Club</div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Pages list */}
        <div style={{padding: '12px 16px 0', display:'grid', gap: 10}}>
          {pages.map(p=>(
            <div key={p.n} className="card" style={{padding: 10, display:'grid', gridTemplateColumns:'1fr 1.1fr', gap: 12, alignItems:'center'}}>
              <Photo variant={p.v} style={{aspectRatio:'4/3', borderRadius: 8}}/>
              <div>
                <div className="page-num">Page {p.n}</div>
                <div style={{fontSize: 15, fontWeight: 800, marginTop: 4}}>{p.t}</div>
                <div style={{fontSize: 11.5, color:'var(--fg-3)', marginTop: 8, lineHeight: 1.5}}>
                  {p.s1}<br/>{p.s2}
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Share */}
        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Send size={13}/> みんなにシェアしよう</div></div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8}}>
              <button className="btn" style={{height: 42, background:'#111827', color:'#fff', borderColor:'#111827'}}><Icon.X size={13}/> Xで共有</button>
              <button className="btn" style={{height: 42, color:'var(--teal)', borderColor:'var(--teal)'}}><Icon.Copy size={13}/> URLをコピー</button>
            </div>
          </div>
        </div>

        {/* Create CTA */}
        <div style={{padding: '14px 16px 0'}}>
          <div className="card" style={{background:'var(--teal-soft)', borderColor:'rgba(20,184,166,0.2)'}}>
            <div style={{display:'flex', alignItems:'center', gap:6, fontSize: 13, fontWeight:800, marginBottom: 4}}>
              <Icon.Sparkle size={14} style={{color:'var(--teal)'}}/> ログイン不要でフォトブックを作る
            </div>
            <div style={{fontSize: 11.5, color:'var(--fg-2)', lineHeight: 1.55, marginBottom: 12}}>
              思い出の写真をまとめて、かんたんにフォトブックを作成。<br/>すぐにシェアできます。
            </div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8}}>
              <button className="btn btn-primary btn-sm" onClick={()=>go('create')}><Icon.Sparkle size={13}/> 今すぐ作る</button>
              <button className="btn btn-sm"><Icon.Image size={13}/> 作例を見る</button>
            </div>
          </div>
        </div>

        {/* Trust strip */}
        <div style={{padding: '8px 10px 8px'}}>
          <div className="trust-strip">
            <div className="cell"><span className="ico"><Icon.Check size={14}/></span>完全無料</div>
            <div className="cell"><span className="ico"><Icon.Camera size={14}/></span>スマホで完結</div>
            <div className="cell"><span className="ico"><Icon.Lock size={14}/></span>安全・安心</div>
            <div className="cell"><span className="ico"><Icon.Sparkle size={14}/></span>VRCユーザー向け</div>
          </div>
        </div>

        <div style={{textAlign:'center', padding: '8px 0 16px', fontSize: 10.5, color:'var(--fg-4)'}}>
          <span className="tap" onClick={()=>go('report')}>問題を報告</span>
        </div>
      </div>
    </PhoneShell>
  );
}

function Manage({ go }) {
  const [sensitive, setSensitive] = React.useState(false);
  const [confirmDel, setConfirmDel] = React.useState(false);
  return (
    <PhoneShell>
      <TopBar back={()=>go('complete')} title="管理ページ" right={<div className="icon-btn tap"><Icon.Menu/></div>}/>
      <div className="pb-scroll" style={{paddingBottom: 30}}>
        <div style={{padding: '14px 16px 0'}}>
          <div className="card" style={{padding: 14}}>
            <div style={{display:'flex', gap: 12}}>
              <Photo variant="a" style={{width: 86, height: 86, borderRadius: 10, flexShrink: 0}}/>
              <div style={{flex:1, minWidth:0}}>
                <div style={{fontSize: 15, fontWeight: 800, lineHeight: 1.2}}>ミッドナイト<br/>ソーシャルクラブ</div>
                <span className="chip" style={{marginTop: 8}}><Icon.Lock size={10}/> 限定公開</span>
              </div>
            </div>
            <div style={{fontSize: 12, color:'var(--fg-3)', marginTop: 10, lineHeight: 1.5}}>最高の夜を、もう一度。</div>
            <div style={{display:'flex', gap: 14, marginTop: 10, paddingTop: 10, borderTop:'1px solid var(--border-2)', fontSize: 11, color:'var(--fg-3)'}}>
              <span><Icon.Users size={11}/> 作成者 nekoma</span>
              <span><Icon.Calendar size={11}/> 2026.04.24</span>
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Globe size={14}/> 公開URL</div></div>
            <div style={{fontSize: 11, color:'var(--fg-3)', marginBottom: 8}}>このURLからフォトブックを閲覧できます。</div>
            <div style={{display:'flex', gap: 8, alignItems:'center'}}>
              <div className="url-pill" style={{flex: 1}}>
                <span style={{overflow:'hidden', textOverflow:'ellipsis'}}>vrc-photobook.com/p/midnight-social-club</span>
              </div>
              <button className="btn btn-sm btn-ghost-cyan" style={{height: 40}}><Icon.Copy size={13}/></button>
            </div>

            <div className="divider"/>

            <div className="sect-head"><div className="title" style={{color:'var(--violet)'}}><Icon.Link size={14}/> 管理URL</div></div>
            <div style={{fontSize: 11, color:'var(--fg-3)', marginBottom: 8}}>このURLからこのフォトブックを管理できます。</div>
            <div style={{display:'flex', gap: 8, alignItems:'center'}}>
              <div className="url-pill" style={{flex: 1, color:'var(--violet)'}}>
                <span style={{overflow:'hidden', textOverflow:'ellipsis'}}>vrc-photobook.com/m/ab12cd34ef56</span>
              </div>
              <button className="btn btn-sm" style={{height: 40, color:'var(--violet)', borderColor:'#E9DDFD'}}><Icon.Copy size={13}/></button>
            </div>
            <div style={{fontSize: 10.5, color:'var(--fg-4)', marginTop: 8}}>
              <Icon.Lock size={11}/> このURLを知っている人は編集・削除できます
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0', display:'grid', gap: 8}}>
          <div className="row tap" onClick={()=>go('edit')}>
            <div className="ico-wrap"><Icon.Pencil size={16}/></div>
            <div className="meta"><div className="t">編集する</div><div className="s">ページや写真の追加・編集を行います</div></div>
            <Icon.Chev/>
          </div>
          <div className="row tap">
            <div className="ico-wrap"><Icon.Users size={16}/></div>
            <div className="meta"><div className="t">公開範囲を変更</div><div className="s">公開状態やURLの設定を変更します</div></div>
            <Icon.Chev/>
          </div>
          <div className="row tap">
            <div className="ico-wrap"><Icon.Refresh size={16}/></div>
            <div className="meta"><div className="t">X共有文を再生成</div><div className="s">フォトブックの共有文を新しく作成します</div></div>
            <Icon.Chev/>
          </div>
          <div className="row danger tap" onClick={()=>setConfirmDel(true)}>
            <div className="ico-wrap"><Icon.Trash size={16}/></div>
            <div className="meta"><div className="t" style={{color:'var(--red)'}}>削除する</div><div className="s">このフォトブックを削除します(復元できません)</div></div>
            <Icon.Chev/>
          </div>
        </div>

        <div style={{padding: '12px 16px 0'}}>
          <div className="card" style={{display:'flex', alignItems:'center', gap: 12, padding: 14}}>
            <Icon.Shield/>
            <div style={{flex:1}}>
              <div style={{fontSize: 13, fontWeight: 700}}>センシティブな内容を含む</div>
              <div style={{fontSize: 10.5, color:'var(--fg-3)'}}>ONにすると閲覧時に注意メッセージが表示されます。</div>
            </div>
            <div className={`switch tap ${sensitive?'on':''}`} onClick={()=>setSensitive(!sensitive)}/>
          </div>
        </div>

        <div style={{padding: '12px 16px 0'}}>
          <div className="info-card" style={{background:'#FAF7FF', borderColor:'#E9DDFD', color:'var(--violet)'}}>
            <Icon.Shield/>
            <div style={{color:'var(--fg-2)'}}>この管理URLを知っている人は編集・削除できます。他人に共有しないでください。</div>
          </div>
        </div>

        <div style={{padding: '14px 16px 20px'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Info size={14}/> 詳細情報</div></div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 10, fontSize: 12}}>
              <div>
                <div style={{color:'var(--fg-3)', fontSize: 10.5, marginBottom: 3}}><Icon.Refresh size={11}/> 最終更新</div>
                <div style={{fontFamily:'var(--font-num)', fontWeight: 600}}>2026.04.24 23:47</div>
              </div>
              <div>
                <div style={{color:'var(--fg-3)', fontSize: 10.5, marginBottom: 3}}><Icon.Calendar size={11}/> 作成</div>
                <div style={{fontFamily:'var(--font-num)', fontWeight: 600}}>2026.04.24</div>
              </div>
              <div>
                <div style={{color:'var(--fg-3)', fontSize: 10.5, marginBottom: 3}}><Icon.Book size={11}/> ページ数</div>
                <div style={{fontFamily:'var(--font-num)', fontWeight: 600}}>3 ページ</div>
              </div>
              <div>
                <div style={{color:'var(--fg-3)', fontSize: 10.5, marginBottom: 3}}><Icon.Image size={11}/> 写真数</div>
                <div style={{fontFamily:'var(--font-num)', fontWeight: 600}}>23 枚</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {confirmDel && (
        <div className="modal-backdrop">
          <div className="modal">
            <div style={{display:'flex', gap:10, alignItems:'flex-start', marginBottom: 14}}>
              <div style={{width: 36, height:36, borderRadius: 10, background:'var(--red-soft)', display:'grid', placeItems:'center', color:'var(--red)', flexShrink:0}}>
                <Icon.Trash size={18}/>
              </div>
              <div>
                <div style={{fontSize: 16, fontWeight: 800}}>このフォトブックを削除しますか?</div>
                <div style={{fontSize: 12, color:'var(--fg-3)', marginTop: 6, lineHeight: 1.5}}>削除すると <strong style={{color:'var(--red)'}}>復元できません</strong>。</div>
              </div>
            </div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 8, marginTop: 14}}>
              <button className="btn btn-sm" onClick={()=>setConfirmDel(false)}>キャンセル</button>
              <button className="btn btn-sm btn-danger" onClick={()=>setConfirmDel(false)}>削除する</button>
            </div>
          </div>
        </div>
      )}
    </PhoneShell>
  );
}

function Report({ go }) {
  const [reason, setReason] = React.useState('unauthorized');
  const reasons = [
    { k:'self', t:'自分が写っているため削除してほしい', i:<Icon.Users/> },
    { k:'unauthorized', t:'無断転載の可能性', i:<Icon.Shield/> },
    { k:'sensitive', t:'センシティブ設定が不足している', i:<Icon.EyeOff/> },
    { k:'harass', t:'嫌がらせ・晒し', i:<Icon.Warn/> },
    { k:'other', t:'その他', i:<Icon.Info/> },
  ];
  return (
    <PhoneShell>
      <TopBar back={()=>go('viewer')} title="問題を報告" right={<div className="icon-btn tap"><Icon.Menu/></div>}/>
      <div className="pb-scroll" style={{paddingBottom: 110}}>
        <div style={{padding: '16px 20px 0', textAlign:'center'}}>
          <div style={{fontSize: 12.5, color:'var(--fg-2)', lineHeight: 1.65}}>
            権利侵害や不適切な内容を見つけた場合は、<br/>こちらからご連絡ください。
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <label className="field-label"><Icon.Link size={11}/> 対象のURL <span style={{color:'var(--red)'}}>*</span></label>
            <input className="input input-height" placeholder="例: https://vrc-photobook.com/p/xxxxx"/>
          </div>
        </div>

        <div style={{padding: '12px 16px 0'}}>
          <div className="card">
            <div className="field-label" style={{marginBottom: 10}}><Icon.Flag size={11}/> 報告理由 <span style={{color:'var(--red)'}}>*</span></div>
            <div style={{display:'grid', gap: 8}}>
              {reasons.map(r=>(
                <div key={r.k} className={`radio-card ${reason===r.k?'active':''}`} onClick={()=>setReason(r.k)} style={{padding: 11}}>
                  <div className="radio-dot"/>
                  <div style={{color:'var(--fg-3)'}}>{r.i}</div>
                  <div style={{flex:1, fontSize: 13}}>{r.t}</div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div style={{padding: '12px 16px 0'}}>
          <div className="card">
            <label className="field-label">詳細</label>
            <textarea className="textarea" placeholder="詳しい状況や理由をご記入ください(任意)" style={{minHeight: 100}}/>
            <div style={{textAlign:'right', fontSize: 10.5, color:'var(--fg-4)', marginTop: 4}}>0 / 500</div>
          </div>
        </div>

        <div style={{padding: '12px 16px 0'}}>
          <div className="card">
            <label className="field-label">連絡先(任意)</label>
            <input className="input input-height" placeholder="メールアドレスまたはX ID"/>
            <div style={{fontSize: 10.5, color:'var(--fg-4)', marginTop: 6, lineHeight: 1.5}}>
              ご記入いただいた場合、必要に応じてご連絡することがあります。
            </div>
          </div>
        </div>
      </div>

      <div className="bottom-bar">
        <button className="btn btn-primary btn-lg btn-block tap" onClick={()=>go('viewer')}>
          <Icon.Send size={14}/> 送信する
        </button>
        <div className="sub">いただいた報告は運営チームが内容を確認し、ガイドラインに基づいて適切に対応します。</div>
      </div>
    </PhoneShell>
  );
}

Object.assign(window, { Complete, Viewer, Manage, Report });
