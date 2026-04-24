// Screens 1-4: LP, CreateStart, Edit, PublishSettings (Light theme)

function LP({ go }) {
  return (
    <PhoneShell>
      <TopBar />
      <div className="pb-scroll bottom-pad">
        <div style={{padding: '20px 20px 0'}}>
          <div className="hero-title">
            VRC写真を、<br/>
            Webフォトブックに。
          </div>
          <div className="t-body" style={{marginTop: 12, color: 'var(--fg-2)'}}>
            ログイン不要で、だれでもかんたんに。<br/>
            思い出を、きれいにまとめて、すぐにシェア。
          </div>

          <div style={{display:'grid', gap: 10, marginTop: 18}}>
            <button className="btn btn-primary btn-lg btn-block tap" onClick={()=>go('create')}>
              <Icon.Sparkle size={14}/> 今すぐ作る
            </button>
            <button className="btn btn-lg btn-block tap" onClick={()=>go('viewer')} style={{color:'var(--teal)', borderColor:'var(--teal)'}}>
              <Icon.Image size={14}/> 作例を見る
            </button>
          </div>

          {/* Hero book mock */}
          <div style={{marginTop: 20, position:'relative'}}>
            <div className="mock-book">
              <div className="left">
                <div className="t">ミッドナイト<br/>ソーシャルクラブ</div>
                <div className="d">2026.04.24</div>
                <div className="w">Midnight Social Club</div>
              </div>
              <Photo variant="a" style={{aspectRatio:'3/4', minHeight: 130}} label="cover"/>
            </div>
          </div>

          {/* Caption */}
          <div style={{textAlign:'center', marginTop: 18, fontSize: 11.5, color:'var(--teal)', display:'flex', alignItems:'center', justifyContent:'center', gap:6}}>
            <Icon.Sparkle size={12}/> 美しいレイアウトで、あなたの思い出を残そう
          </div>

          {/* Thumbnails strip */}
          <div style={{display:'grid', gridTemplateColumns:'1fr 1fr 1fr 1fr', gap: 6, marginTop: 10}}>
            {['a','c','b','d'].map((v,i)=>(
              <Photo key={i} variant={v} style={{aspectRatio:'1/1', borderRadius: 8}}/>
            ))}
          </div>
        </div>

        {/* Features */}
        <div style={{padding: '22px 18px 0'}}>
          <div className="card" style={{padding: 16}}>
            <div className="sect-head" style={{paddingBottom: 12}}>
              <div className="title"><Icon.Book size={14}/> VRC PhotoBookの特徴</div>
            </div>
            <div className="feature-grid">
              {[
                { i: <Icon.Users size={18}/>, t:'ログイン不要', s:'アカウント登録なしですぐフォトブックを作成できます。'},
                { i: <Icon.Link size={18}/>, t:'URLで共有', s:'生成されたURLを共有するだけで、みんなが写真を楽しめます。'},
                { i: <Icon.Pencil size={18}/>, t:'管理URLで編集', s:'管理用URLからいつでも編集・追加・並べ替えが可能です。'},
                { i: <Icon.Calendar size={18}/>, t:'イベント・おはツイ・作品集', s:'イベントの記録やおはツイ、作品集など様々な用途に。'},
              ].map((f,i)=>(
                <div key={i} className="feature-cell">
                  <div className="ico">{f.i}</div>
                  <div className="t">{f.t}</div>
                  <div className="s">{f.s}</div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Final CTA */}
        <div style={{padding: '18px 18px 0'}}>
          <div className="cta-block">
            <div className="eyebrow">さあ、あなたの思い出をカタチにしよう</div>
            <button className="btn btn-primary btn-lg btn-block tap" onClick={()=>go('create')}>
              <Icon.Sparkle size={14}/> 無料でフォトブックを作る
            </button>
            <div style={{textAlign:'center', marginTop: 8, fontSize: 10.5, color:'var(--fg-3)'}}>ログイン不要・完全無料</div>
          </div>
        </div>

        {/* Trust strip */}
        <div style={{padding: '16px 10px 8px'}}>
          <div className="trust-strip">
            <div className="cell"><span className="ico"><Icon.Check size={14}/></span>完全無料</div>
            <div className="cell"><span className="ico"><Icon.Camera size={14}/></span>スマホで完結</div>
            <div className="cell"><span className="ico"><Icon.Lock size={14}/></span>安全・安心</div>
            <div className="cell"><span className="ico"><Icon.Sparkle size={14}/></span>VRCユーザー向け</div>
          </div>
        </div>
      </div>
    </PhoneShell>
  );
}

function CreateStart({ go }) {
  const [type, setType] = React.useState('event');
  const [tpl, setTpl] = React.useState('simple');
  const types = [
    { k:'event', t:'イベント', s:'イベントの記録や、思い出のシーンをまとめます。', v:'a' },
    { k:'morning', t:'おはツイ', s:'おはツイの記録や、日々の交流をまとめます。', v:'d' },
    { k:'portfolio', t:'作品集', s:'ワールドや写真作品を、美しくまとめます。', v:'c' },
    { k:'avatar', t:'アバター紹介', s:'あなたのアバターを、魅力的に紹介します。', v:'e' },
    { k:'free', t:'自由作成', s:'自由にレイアウトを組んで、オリジナルの一冊を作ります。', v:'f' },
  ];
  return (
    <PhoneShell>
      <TopBar back={()=>go('lp')} title="" right={<Logo/>}/>
      <div className="pb-scroll" style={{paddingBottom: 100}}>
        <div style={{padding: '18px 20px 14px'}}>
          <div className="t-h1">どんなフォトブックを作りますか?</div>
          <div className="t-sm" style={{marginTop: 6}}>まずはフォトブックのタイプを選んでください。<br/>あとから変更することもできます。</div>
        </div>

        <div style={{padding: '0 16px', display:'grid', gap: 10}}>
          {types.map(x=>(
            <div key={x.k} className={`type-card tap ${type===x.k?'active':''}`} onClick={()=>setType(x.k)}>
              <Photo variant={x.v} className="thumb" label="ph"/>
              <div style={{flex:1, minWidth:0}}>
                <div style={{fontSize: 15, fontWeight: 800, color:'var(--fg)'}}>{x.t}</div>
                <div style={{fontSize: 11.5, color:'var(--fg-3)', marginTop: 4, lineHeight: 1.45}}>{x.s}</div>
              </div>
              <Icon.Chev size={16}/>
            </div>
          ))}
        </div>

        <div style={{padding: '18px 16px 0'}}>
          <div style={{display:'flex', alignItems:'baseline', justifyContent:'space-between', padding:'0 2px 10px'}}>
            <div style={{fontSize: 13, fontWeight:700}}>テンプレートのスタイルを選ぶ</div>
            <span style={{fontSize: 10, color: 'var(--fg-4)'}}>※あとから変更できます</span>
          </div>
          <div style={{display:'grid', gridTemplateColumns:'1fr 1fr 1fr', gap: 8}}>
            {[
              { k:'simple', t:'シンプル', s:'すっきり見やすい定番レイアウト', v:'e' },
              { k:'card', t:'カード型', s:'写真をカード風に並べるデザイン', v:'c' },
              { k:'large', t:'大判', s:'写真を大きく魅せる迫力レイアウト', v:'a' },
            ].map(s=>(
              <div key={s.k} className={`type-card tap ${tpl===s.k?'active':''}`}
                style={{flexDirection:'column', padding: 8, alignItems:'stretch', gap: 8}}
                onClick={()=>setTpl(s.k)}>
                <Photo variant={s.v} style={{aspectRatio:'4/3', borderRadius: 6}}/>
                <div style={{padding:'0 2px'}}>
                  <div style={{fontSize: 12, fontWeight:700}}>{s.t}</div>
                  <div style={{fontSize: 10, color:'var(--fg-3)', marginTop: 2, lineHeight: 1.3}}>{s.s}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="bottom-bar">
        <button className="btn btn-primary btn-lg btn-block tap" onClick={()=>go('edit')}>
          <Icon.Sparkle size={14}/> このタイプではじめる
        </button>
      </div>
    </PhoneShell>
  );
}

function Edit({ go }) {
  const [layout, setLayout] = React.useState('full');
  const layouts = [
    { k:'full', t:'フルページ', i:<Icon.Full/> },
    { k:'spread', t:'見開き', i:<Icon.Spread/> },
    { k:'grid', t:'グリッド', i:<Icon.Grid/> },
    { k:'collage', t:'コラージュ', i:<Icon.Collage/> },
  ];
  const pages = [
    { n:'01', t:'集合写真', v:'a' },
    { n:'02', t:'クラブでダンス', v:'c' },
    { n:'03', t:'まったりタイム', v:'b' },
    { n:'04', t:'ラスト', v:'d' },
  ];
  return (
    <PhoneShell>
      <div className="topbar">
        <div style={{display:'flex',alignItems:'center',gap:10}}>
          <div className="icon-btn tap" onClick={()=>go('create')} style={{color:'var(--teal)'}}><Icon.ArrowLeft/></div>
          <div style={{fontWeight:700, fontSize: 16}}>編集</div>
        </div>
        <button className="btn btn-sm" style={{color:'var(--teal)', borderColor:'rgba(20,184,166,0.3)'}}><Icon.Eye size={13}/> プレビュー</button>
      </div>
      <Steps active="edit"/>
      <div className="pb-scroll" style={{paddingBottom: 110}}>
        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Book size={14}/> 基本情報</div></div>
            <label className="field-label">ブックタイトル</label>
            <input className="input input-height" defaultValue="ミッドナイト ソーシャルクラブ"/>
            <div style={{textAlign:'right', fontSize: 10.5, color:'var(--fg-4)', marginTop: 4}}>14 / 60</div>
            <label className="field-label" style={{marginTop: 8}}>説明</label>
            <textarea className="textarea" defaultValue="2026.04.24 に集まったミッドナイト ソーシャルクラブの思い出をまとめたフォトブックです。"/>
            <div style={{textAlign:'right', fontSize: 10.5, color:'var(--fg-4)'}}>41 / 200</div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Image size={14}/> 写真を追加</div></div>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 12}}>JPG / PNG / WEBPに対応、最大200枚まで追加できます。</div>
            <div style={{
              border:'1.5px dashed #CBD5E1',
              borderRadius: 12,
              padding: '16px 14px',
              textAlign:'center',
              background: 'var(--bg-2)',
            }}>
              <button className="btn btn-primary btn-sm" style={{height: 40}}><Icon.Plus size={14}/> 写真を追加</button>
              <div style={{fontSize: 10.5, color:'var(--fg-3)', marginTop: 8}}>または、ここにドラッグ&ドロップ</div>
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div style={{display:'flex', justifyContent:'space-between', alignItems:'baseline', marginBottom: 10}}>
              <div style={{fontSize: 14, fontWeight:700, display:'flex', alignItems:'center', gap:6}}>
                <Icon.Book size={14} style={{color:'var(--teal)'}}/> ページ一覧 <span style={{color:'var(--fg-4)', fontSize:10, fontWeight:500}}>(ドラッグで並び替え)</span>
              </div>
              <span style={{fontSize: 11, color:'var(--fg-3)'}}>全 {pages.length} ページ</span>
            </div>
            <div style={{display:'flex', gap: 8, overflowX:'auto', margin:'0 -4px', padding: '0 4px 4px'}}>
              {pages.map((p,i)=>(
                <div key={p.n} style={{flex:'0 0 auto', width: 120, border: i===0?'2px solid var(--teal)':'1px solid var(--border)', borderRadius: 10, background:'#fff', padding: 4, position:'relative'}}>
                  <div style={{position:'absolute', top: 4, left: 8, background:'#fff', borderRadius: 4, padding:'2px 6px', fontSize: 10, fontWeight:700, color:'var(--fg-2)', zIndex:2, border:'1px solid var(--border)'}}>Page {p.n}</div>
                  <div style={{position:'absolute', top: 4, right: 8, fontSize: 14, color:'var(--fg-4)', fontWeight:700, zIndex:2}}>⋯</div>
                  <Photo variant={p.v} style={{aspectRatio:'3/4', borderRadius: 6}}/>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Spread size={14}/> レイアウトを選択</div></div>
            <div className="layout-grid">
              {layouts.map(l=>(
                <div key={l.k} className={`layout-chip ${layout===l.k?'active':''}`} onClick={()=>setLayout(l.k)}>
                  <div className="ico">{l.i}</div>
                  <div>{l.t}</div>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Calendar size={14}/> メタデータ</div></div>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr 1fr', gap: 8}}>
              <div>
                <label className="field-label" style={{fontSize: 10.5}}>World</label>
                <input className="input" style={{height: 40, fontSize: 12, padding: '8px 10px'}} defaultValue="Midnight Rooftop"/>
              </div>
              <div>
                <label className="field-label" style={{fontSize: 10.5}}>Cast</label>
                <input className="input" style={{height: 40, fontSize: 12, padding: '8px 10px'}} defaultValue="Luna, Noir, Miko"/>
              </div>
              <div>
                <label className="field-label" style={{fontSize: 10.5}}>Photographer</label>
                <input className="input" style={{height: 40, fontSize: 12, padding: '8px 10px'}} defaultValue="あなたの名前"/>
              </div>
            </div>
            <div style={{textAlign:'right', fontSize: 10, color:'var(--fg-4)', marginTop: 6}}>0 / 40</div>
          </div>
        </div>
      </div>

      <div className="bottom-bar">
        <button className="btn btn-primary btn-lg btn-block tap" onClick={()=>go('publish')}>
          <Icon.Sparkle size={14}/> 公開設定へ進む
        </button>
        <div style={{display:'flex', justifyContent:'space-between', marginTop: 8, fontSize: 10.5, color:'var(--fg-3)'}}>
          <span>いつでも下書き保存できます</span>
          <span style={{color:'var(--teal)'}}><Icon.Book size={11}/> 下書きを保存</span>
        </div>
      </div>
    </PhoneShell>
  );
}

function PublishSettings({ go }) {
  const [visibility, setVis] = React.useState('unlisted');
  const [sensitive, setSensitive] = React.useState(false);
  const [agree1, setA1] = React.useState(true);
  const [agree2, setA2] = React.useState(true);
  const [bot, setBot] = React.useState(false);
  return (
    <PhoneShell>
      <TopBar back={()=>go('edit')} title="公開設定" right={<div className="icon-btn tap"><Icon.Menu/></div>}/>
      <Steps active="pub"/>
      <div className="pb-scroll" style={{paddingBottom: 110}}>
        <div style={{padding: '14px 16px 0'}}>
          <div className="card" style={{display:'flex', gap: 12, alignItems:'center'}}>
            <Photo variant="a" style={{width: 72, height: 72, borderRadius: 10, flexShrink: 0}}/>
            <div style={{flex:1, minWidth:0}}>
              <div className="t-xs">フォトブックのタイトル</div>
              <div style={{fontSize: 15, fontWeight: 800, marginTop: 2}}>ミッドナイトソーシャルクラブ</div>
              <div style={{display:'flex', gap: 10, marginTop: 6, fontSize: 11, color:'var(--fg-3)'}}>
                <span>全 3 ページ</span><span>23 枚</span>
              </div>
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Globe size={14}/> 公開範囲</div></div>
            <div style={{display:'grid', gap: 8}}>
              {[
                { k:'public', t:'公開', s:'誰でも検索・閲覧できます。', i:<Icon.Globe/> },
                { k:'unlisted', t:'限定公開', s:'URLを知っている人のみ閲覧できます。', i:<Icon.Users/>, rec: true },
                { k:'private', t:'非公開', s:'管理URLからのみ閲覧できます。', i:<Icon.Lock/> },
              ].map(v=>(
                <div key={v.k} className={`radio-card ${visibility===v.k?'active':''}`} onClick={()=>setVis(v.k)}>
                  <div style={{color:'var(--teal)'}}>{v.i}</div>
                  <div style={{flex:1}}>
                    <div style={{fontSize: 13.5, fontWeight: 700, display:'flex', gap: 6, alignItems:'center'}}>
                      {v.t}
                      {v.rec && <span className="chip" style={{height: 18, fontSize: 9.5, padding:'0 6px'}}>推奨</span>}
                    </div>
                    <div style={{fontSize: 11.5, color:'var(--fg-3)', marginTop: 2}}>{v.s}</div>
                  </div>
                  <div className="radio-dot"/>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card" style={{display:'flex', alignItems:'center', gap: 12}}>
            <Icon.Shield/>
            <div style={{flex:1}}>
              <div style={{fontSize: 13.5, fontWeight: 700}}>センシティブ設定</div>
              <div style={{fontSize: 11, color:'var(--fg-3)', marginTop: 2}}>ONにすると閲覧前に注意が表示されます。</div>
            </div>
            <div className={`switch tap ${sensitive?'on':''}`} onClick={()=>setSensitive(!sensitive)}/>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <div className="sect-head"><div className="title"><Icon.Users size={14}/> 作成者情報</div></div>
            <label className="field-label">表示名</label>
            <input className="input input-height" defaultValue="nekoma"/>
            <label className="field-label" style={{marginTop: 10}}>X ID(任意)</label>
            <input className="input input-height" defaultValue="@nekoma_vrc"/>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card">
            <label className="field-label" style={{marginBottom: 10, display:'flex', alignItems:'center', gap:6}}><Icon.Shield size={12}/> 権利・同意</label>
            <div style={{display:'flex', gap: 10, marginBottom: 10}} className="tap" onClick={()=>setA1(!agree1)}>
              <div className={`check ${agree1?'on':''}`}/>
              <div style={{fontSize: 12, lineHeight: 1.5, color:'var(--fg-2)'}}>写っている人やワールド等に配慮し、公開して問題ない内容であることを確認しました。</div>
            </div>
            <div style={{display:'flex', gap: 10}} className="tap" onClick={()=>setA2(!agree2)}>
              <div className={`check ${agree2?'on':''}`}/>
              <div style={{fontSize: 12, lineHeight: 1.5, color:'var(--fg-2)'}}>VRC PhotoBookの <span style={{color:'var(--teal)', fontWeight:600}}>利用規約・ガイドライン</span> に同意します。</div>
            </div>
          </div>
        </div>

        <div style={{padding: '14px 16px 0'}}>
          <div className="card" style={{display:'flex', gap: 12, alignItems:'center'}}>
            <div className={`check ${bot?'on':''} tap`} onClick={()=>setBot(!bot)}/>
            <div style={{flex:1, fontSize: 13, fontWeight: 600}}>私はロボットではありません</div>
            <div style={{fontSize: 9.5, color:'var(--fg-4)', textAlign:'right', lineHeight: 1.2}}>
              <div style={{fontWeight:700, fontSize: 10, color: 'var(--fg-3)'}}>CLOUDFLARE</div>
              Turnstile
            </div>
          </div>
        </div>
      </div>

      <div className="bottom-bar">
        <button className="btn btn-primary btn-lg btn-block tap" onClick={()=>go('complete')}>
          <Icon.Send size={14}/> 公開する
        </button>
        <div className="sub">ログイン不要。公開後に管理URLを発行します。</div>
      </div>
    </PhoneShell>
  );
}

Object.assign(window, { LP, CreateStart, Edit, PublishSettings });
