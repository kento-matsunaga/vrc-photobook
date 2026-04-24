// PC Screens 1-4

function PCLP({ go }) {
  return (
    <PCBrowser url="https://vrc-photobook.com">
      <PCHeader go={go}/>

      <div className="pc-container">
        <div className="pc-hero">
          <div>
            <h1>VRC写真を、<br/><span className="accent">Web</span>フォトブックに。</h1>
            <div className="sub">ログイン不要で、だれでもかんたんに。<br/>思い出を、きれいにまとめて、すぐにシェア。</div>
            <div className="cta-row">
              <button className="btn btn-primary btn-lg" onClick={()=>go('pc-create')}>
                <Icon.Sparkle size={14}/> 今すぐ作る
              </button>
              <button className="btn btn-lg" style={{color:'var(--teal)', borderColor:'var(--teal)'}} onClick={()=>go('pc-viewer')}>
                <Icon.Image size={14}/> 作例を見る
              </button>
            </div>
          </div>
          <div className="pc-book-mock">
            <div className="pc-book-pages">
              <div className="pg-left">
                <div className="t">ミッドナイト<br/>ソーシャルクラブ</div>
                <div className="d">2026.04.24</div>
                <div className="w">Midnight Social Club</div>
              </div>
              <Photo variant="a" style={{width:'100%', height:'100%', borderRadius: 0}} label="right page"/>
            </div>
            <div className="pc-book-card c1" style={{background:'linear-gradient(135deg,#C4B5FD,#F9A8D4)'}}/>
            <div className="pc-book-card c2" style={{background:'linear-gradient(135deg,#A7F3D0,#BAE6FD)'}}/>
          </div>
        </div>

        {/* thumb strip */}
        <div className="pc-thumb-strip">
          {['a','c','b','d','e'].map((v,i)=> <Photo key={i} variant={v}/> )}
        </div>

        {/* Features */}
        <div className="pc-card" style={{marginTop: 32}}>
          <h3 className="pc-sect-title"><span className="ico"><Icon.Book size={16}/></span> VRC PhotoBookの特徴</h3>
          <div className="pc-features-grid">
            {[
              { i:<Icon.Users size={18}/>, t:'ログイン不要', s:'アカウント登録なしですぐフォトブックを作成できます。' },
              { i:<Icon.Link size={18}/>, t:'URLで共有', s:'生成されたURLを共有するだけで、みんなが写真を楽しめます。' },
              { i:<Icon.Pencil size={18}/>, t:'管理URLで編集', s:'管理用URLがあれば、いつでも編集・追加・並べ替えが可能です。' },
              { i:<Icon.Calendar size={18}/>, t:'イベント・おはツイ・作品集', s:'イベントの記録やおはツイ、作品集など様々な用途に活用できます。' },
            ].map((f,i)=>(
              <div key={i} className="pc-feature">
                <div className="ico">{f.i}</div>
                <div className="t">{f.t}</div>
                <div className="s">{f.s}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Usecases */}
        <div style={{marginTop: 24}}>
          <h3 className="pc-sect-title" style={{color:'var(--fg-2)'}}>こんなシーンで活用できます</h3>
          <div className="pc-usecase-grid">
            {[
              { i:<Icon.Calendar size={18}/>, t:'イベント', s:'イベントの記録や、思い出のシーンをまとめます。', v:'c' },
              { i:<Icon.Book size={18}/>, t:'おはツイ', s:'おはツイの記録や、日々の交流をまとめます。', v:'d' },
              { i:<Icon.Image size={18}/>, t:'作品集', s:'ワールドや写真作品を、美しくまとめます。', v:'a' },
            ].map((u,i)=>(
              <div key={i} className="pc-usecase">
                <div className="ico">{u.i}</div>
                <div>
                  <div className="t">{u.t}</div>
                  <div className="s">{u.s}</div>
                </div>
                <Photo variant={u.v} className="ph"/>
              </div>
            ))}
          </div>
        </div>

        {/* CTA */}
        <div className="pc-cta" style={{marginTop: 28}}>
          <div className="eyebrow">さあ、あなたの思い出をカタチにしよう</div>
          <button className="btn btn-primary btn-lg" onClick={()=>go('pc-create')} style={{minWidth: 320}}>
            <Icon.Sparkle size={14}/> 無料でフォトブックを作る
          </button>
          <div style={{marginTop: 10, fontSize: 11, color:'var(--fg-3)'}}>ログイン不要・完全無料</div>
        </div>

        <PCTrust/>
      </div>
    </PCBrowser>
  );
}

function PCCreateStart({ go }) {
  const [type, setType] = React.useState('event');
  const [tpl, setTpl] = React.useState('simple');
  const types = [
    { k:'event',     t:'イベント',     i:<Icon.Calendar size={16}/>, s:'オフ会やライブなどの思い出を1冊にまとめます。', v:'a' },
    { k:'morning',   t:'おはツイ',     i:<Icon.Sparkle size={16}/>,  s:'日々の「おはようツイート」をかわいく残します。', v:'d' },
    { k:'portfolio', t:'作品集',       i:<Icon.Book size={16}/>,     s:'ワールドや写真作品をまとめて紹介します。', v:'c' },
    { k:'avatar',    t:'アバター紹介', i:<Icon.Users size={16}/>,    s:'アバターの魅力をまとめてプロフィールブックに。', v:'e' },
    { k:'free',      t:'自由作成',     i:<Icon.Pencil size={16}/>,   s:'用途を決めずに、自由にページを作成できます。', v:'f' },
  ];
  return (
    <PCBrowser url="https://vrc-photobook.com/create">
      <PCHeader go={go}/>
      <PCSteps active="type"/>
      <div className="pc-container">
        <div style={{textAlign:'center', padding:'8px 0 24px'}}>
          <h1 style={{fontSize: 32, fontWeight: 800, letterSpacing:'-0.015em', margin:0}}>どんなフォトブックを作りますか?</h1>
          <div style={{fontSize: 13.5, color:'var(--fg-3)', marginTop: 10}}>まずは用途を選ぶだけ。あとから変更もできます。</div>
        </div>

        <div className="pc-type-grid">
          {types.map(x=>(
            <div key={x.k} className={`pc-type-card ${type===x.k?'active':''}`} onClick={()=>setType(x.k)}>
              <Photo variant={x.v} className="thumb"/>
              <div className="t"><span className="ico">{x.i}</span> {x.t}</div>
              <div className="s">{x.s}</div>
            </div>
          ))}
        </div>

        <div className="pc-card" style={{marginTop: 20}}>
          <div style={{display:'flex', alignItems:'baseline', justifyContent:'space-between', marginBottom: 14}}>
            <h3 className="pc-sect-title" style={{margin:0}}><span className="ico"><Icon.Sparkle size={14}/></span> テンプレートのスタイルを選ぶ</h3>
            <span style={{fontSize: 11, color:'var(--fg-4)'}}>※あとから変更できます</span>
          </div>
          <div style={{display:'grid', gridTemplateColumns:'repeat(3, 1fr)', gap: 12}}>
            {[
              { k:'simple', t:'シンプル', s:'すっきり見やすい定番レイアウト', v:'e' },
              { k:'card', t:'カード型', s:'写真をカード風に並べるデザイン', v:'c' },
              { k:'large', t:'大判', s:'写真を大きく魅せる迫力レイアウト', v:'a' },
            ].map(s=>(
              <div key={s.k} className={`pc-type-card ${tpl===s.k?'active':''}`} onClick={()=>setTpl(s.k)}>
                <Photo variant={s.v} className="thumb" style={{aspectRatio:'16/7'}}/>
                <div className="t" style={{fontSize: 14}}>{s.t}</div>
                <div className="s">{s.s}</div>
              </div>
            ))}
          </div>
        </div>

        <div style={{display:'flex', justifyContent:'center', marginTop: 28}}>
          <button className="btn btn-primary btn-lg" onClick={()=>go('pc-edit')} style={{minWidth: 360}}>
            <Icon.Sparkle size={14}/> このタイプではじめる
          </button>
        </div>
        <div style={{textAlign:'center', marginTop: 10, fontSize: 11, color:'var(--fg-4)'}}>
          ログイン不要・保存は公開後に管理URLで行えます。
        </div>
      </div>
    </PCBrowser>
  );
}

function PCEdit({ go }) {
  const [layout, setLayout] = React.useState('full');
  const [activePage, setActivePage] = React.useState(0);
  const layouts = [
    { k:'full', t:'フルページ', i:<Icon.Full size={14}/> },
    { k:'card', t:'カード型', i:<Icon.Spread size={14}/> },
    { k:'large', t:'大判', i:<Icon.Grid size={14}/> },
    { k:'collage', t:'コラージュ', i:<Icon.Collage size={14}/> },
  ];
  const pages = Array.from({length:8}).map((_,i)=>({
    n: String(i+1).padStart(2,'0'),
    v: ['a','c','b','d','e','f','g','a'][i],
  }));
  return (
    <PCBrowser url="https://vrc-photobook.com/editor">
      <PCHeader go={go}/>
      <PCSteps active="edit"/>

      <div className="pc-editor">
        {/* LEFT PANEL */}
        <div style={{display:'flex', flexDirection:'column', gap: 16}}>
          <div className="pc-card">
            <h3 className="pc-panel-title"><span className="ico"><Icon.Book size={14}/></span> 基本情報</h3>
            <label className="field-label">ブックタイトル</label>
            <input className="input" defaultValue="ミッドナイト ソーシャルクラブ"/>
            <div style={{textAlign:'right', fontSize: 10.5, color:'var(--fg-4)', marginTop: 3}}>14 / 60</div>
            <label className="field-label" style={{marginTop: 10}}>説明</label>
            <textarea className="textarea" defaultValue="2026.04.24 に集まったミッドナイト ソーシャルクラブの思い出をまとめたフォトブックです。"/>
            <div style={{textAlign:'right', fontSize: 10.5, color:'var(--fg-4)'}}>41 / 200</div>
          </div>

          <div className="pc-card">
            <div style={{display:'flex', justifyContent:'space-between', alignItems:'center', marginBottom: 10}}>
              <div style={{fontSize: 13, fontWeight: 800}}>テンプレートのスタイルを選ぶ</div>
              <span style={{fontSize: 10, color:'var(--fg-4)'}}>※あとから変更できます</span>
            </div>
            <div style={{display:'flex', flexDirection:'column', gap: 8}}>
              {[
                { k:'simple', t:'シンプル', s:'すっきり見やすい定番レイアウト', v:'e' },
                { k:'card', t:'カード型', s:'写真をカード風に並べるデザイン', v:'c' },
                { k:'large', t:'大判', s:'写真を大きく魅せる迫力レイアウト', v:'a' },
              ].map((s,i)=>(
                <div key={s.k} className={`type-card ${i===0?'active':''}`}>
                  <Photo variant={s.v} className="thumb" style={{width: 68, height: 52}}/>
                  <div>
                    <div style={{fontSize: 13, fontWeight: 800}}>{s.t}</div>
                    <div style={{fontSize: 10.5, color:'var(--fg-3)', marginTop: 2}}>{s.s}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* MIDDLE PANEL */}
        <div style={{display:'flex', flexDirection:'column', gap: 16}}>
          <div className="pc-card">
            <h3 className="pc-panel-title"><span className="ico"><Icon.Image size={14}/></span> 写真を追加</h3>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginBottom: 12}}>JPG / PNG / WEBPに対応。最大200枚まで追加できます。</div>
            <div style={{
              border: '1.5px dashed #CBD5E1',
              borderRadius: 12,
              padding: '22px 16px',
              textAlign: 'center',
              background: 'var(--bg-2)',
            }}>
              <button className="btn btn-primary btn-sm" style={{height: 40}}><Icon.Plus size={14}/> 写真を追加</button>
              <div style={{fontSize: 11, color:'var(--fg-3)', marginTop: 10}}>または、ここにドラッグ&ドロップ</div>
            </div>
          </div>

          <div className="pc-card">
            <div style={{display:'flex', justifyContent:'space-between', alignItems:'baseline', marginBottom: 12}}>
              <h3 className="pc-panel-title" style={{margin:0}}><span className="ico"><Icon.Book size={14}/></span> ページ一覧 <span style={{color:'var(--fg-4)', fontWeight: 500, fontSize: 11, marginLeft: 4}}>(ドラッグで並び替え)</span></h3>
              <span style={{fontSize: 11, color:'var(--fg-3)'}}>全 {pages.length} ページ</span>
            </div>
            <div className="pc-page-grid">
              {pages.map((p,i)=>(
                <div key={p.n} className={`page-tile ${i===activePage?'active':''}`} onClick={()=>setActivePage(i)}>
                  <div className="label">Page {p.n}</div>
                  <div className="dots">⋯</div>
                  <Photo variant={p.v} className="ph"/>
                </div>
              ))}
            </div>
          </div>

          <div className="pc-card">
            <h3 className="pc-panel-title"><span className="ico"><Icon.Spread size={14}/></span> レイアウトを選択</h3>
            <div className="pc-layout-row">
              {layouts.map(l=>(
                <div key={l.k} className={`pc-layout-card ${layout===l.k?'active':''}`} onClick={()=>setLayout(l.k)}>
                  {l.i} {l.t}
                </div>
              ))}
            </div>
          </div>

          <div className="pc-card">
            <h3 className="pc-panel-title"><span className="ico"><Icon.Calendar size={14}/></span> メタデータ</h3>
            <div style={{display:'grid', gridTemplateColumns:'1fr 1fr 1fr', gap: 10}}>
              <div>
                <label className="field-label">World</label>
                <input className="input" defaultValue="Midnight Rooftop"/>
                <div style={{textAlign:'right', fontSize: 10, color:'var(--fg-4)', marginTop: 3}}>16 / 40</div>
              </div>
              <div>
                <label className="field-label">Cast</label>
                <input className="input" defaultValue="Luna, Noir, Miko, Shiro"/>
                <div style={{textAlign:'right', fontSize: 10, color:'var(--fg-4)', marginTop: 3}}>22 / 40</div>
              </div>
              <div>
                <label className="field-label">Photographer</label>
                <input className="input" placeholder="あなたの名前"/>
                <div style={{textAlign:'right', fontSize: 10, color:'var(--fg-4)', marginTop: 3}}>0 / 40</div>
              </div>
            </div>
          </div>
        </div>

        {/* RIGHT PREVIEW PANE */}
        <div style={{display:'flex', flexDirection:'column', gap: 16, position:'sticky', top: 82, alignSelf: 'start'}}>
          <div className="pc-card">
            <h3 className="pc-panel-title"><span className="ico"><Icon.Eye size={14}/></span> プレビュー</h3>
            <div style={{fontSize: 11, color:'var(--fg-3)', marginBottom: 8}}>Page {String(activePage+1).padStart(2,'0')} ({activePage+1}ページ目)</div>
            <div className="pc-preview-book">
              <div className="pg-left">
                <div className="t">ミッドナイト<br/>ソーシャルクラブ</div>
                <div className="d">2026.04.24<br/>Midnight Social Club</div>
              </div>
              <Photo variant={pages[activePage].v} style={{width:'100%', height:'100%', borderRadius: 0}}/>
            </div>
            <div className="pc-nav-pills">
              <div className="pill-btn" onClick={()=>setActivePage(Math.max(0, activePage-1))}><Icon.ArrowLeft size={14}/></div>
              <div className="page-of">{String(activePage+1)} / {pages.length}</div>
              <div className="pill-btn" onClick={()=>setActivePage(Math.min(pages.length-1, activePage+1))}><Icon.ArrowRight size={14}/></div>
              <button className="btn btn-sm" style={{marginLeft: 8, height: 30, fontSize: 11}}><Icon.Full size={12}/> 全体プレビュー</button>
            </div>
          </div>

          <div className="pc-card">
            <div style={{display:'flex', justifyContent:'space-between', alignItems:'baseline', marginBottom: 10}}>
              <div style={{fontSize: 12.5, fontWeight: 800}}>このページの写真</div>
              <span style={{fontSize: 11, color:'var(--fg-3)'}}>4 枚</span>
            </div>
            <div className="pc-minis">
              {['a','c','d','b'].map((v,i)=>(
                <Photo key={i} variant={v} className="m"/>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="pc-action-bar">
        <button className="btn btn-primary btn-lg" style={{minWidth: 380}} onClick={()=>go('pc-publish')}>
          <Icon.Sparkle size={14}/> 公開設定へ進む
        </button>
        <button className="btn btn-lg" style={{color:'var(--teal)', borderColor:'var(--teal)'}}>
          <Icon.Book size={14}/> 下書きを保存
        </button>
        <div className="sub">いつでも下書きを保存できます</div>
      </div>
    </PCBrowser>
  );
}

function PCPublishSettings({ go }) {
  const [vis, setVis] = React.useState('unlisted');
  const [sen, setSen] = React.useState(false);
  const [a1, setA1] = React.useState(true);
  const [a2, setA2] = React.useState(true);
  const [bot, setBot] = React.useState(false);
  return (
    <PCBrowser url="https://vrc-photobook.com/publish">
      <PCHeader go={go}/>
      <PCSteps active="pub"/>

      <div className="pc-container narrow">
        <div style={{textAlign:'center', padding:'8px 0 22px'}}>
          <h1 style={{fontSize: 30, fontWeight: 800, letterSpacing:'-0.015em', margin:0}}>公開設定</h1>
          <div style={{fontSize: 13, color:'var(--fg-3)', marginTop: 8}}>公開範囲・作成者情報を確認してから公開しましょう。</div>
        </div>

        <div className="pc-card" style={{display:'flex', alignItems:'center', gap: 18, marginBottom: 18}}>
          <Photo variant="a" style={{width: 100, height: 100, borderRadius: 10, flexShrink: 0}}/>
          <div style={{flex: 1, minWidth: 0}}>
            <div className="t-xs" style={{marginBottom: 4}}>フォトブックのタイトル</div>
            <div style={{fontSize: 19, fontWeight: 800, letterSpacing:'-0.01em'}}>ミッドナイト ソーシャルクラブ</div>
            <div style={{display:'flex', gap: 16, marginTop: 10, fontSize: 12, color:'var(--fg-3)'}}>
              <span><Icon.Book size={12}/> 全 3 ページ</span>
              <span><Icon.Image size={12}/> 23 枚</span>
              <span><Icon.Users size={12}/> 作成者 nekoma</span>
            </div>
          </div>
        </div>

        <div className="pc-card" style={{marginBottom: 18}}>
          <h3 className="pc-panel-title"><span className="ico"><Icon.Globe size={14}/></span> 公開範囲</h3>
          <div style={{display:'grid', gridTemplateColumns:'1fr 1fr 1fr', gap: 10}}>
            {[
              { k:'public', t:'公開', s:'誰でも検索・閲覧できます。', i:<Icon.Globe size={18}/> },
              { k:'unlisted', t:'限定公開', s:'URLを知っている人のみ。', i:<Icon.Users size={18}/>, rec: true },
              { k:'private', t:'非公開', s:'管理URLからのみ閲覧。', i:<Icon.Lock size={18}/> },
            ].map(v=>(
              <div key={v.k} className={`radio-card ${vis===v.k?'active':''}`} onClick={()=>setVis(v.k)} style={{flexDirection:'column', alignItems:'flex-start', padding: 14, gap: 8}}>
                <div style={{display:'flex', width:'100%', justifyContent:'space-between', alignItems:'center'}}>
                  <div style={{color:'var(--teal)'}}>{v.i}</div>
                  <div className="radio-dot"/>
                </div>
                <div>
                  <div style={{fontSize: 14, fontWeight: 800, display:'flex', gap: 6, alignItems:'center'}}>
                    {v.t}
                    {v.rec && <span className="chip" style={{fontSize: 9.5, height: 18, padding:'0 6px'}}>推奨</span>}
                  </div>
                  <div style={{fontSize: 11.5, color:'var(--fg-3)', marginTop: 4}}>{v.s}</div>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="pc-card" style={{display:'flex', alignItems:'center', gap: 14, marginBottom: 18}}>
          <div style={{width: 40, height: 40, borderRadius: 10, background:'var(--teal-soft)', color:'var(--teal)', display:'grid', placeItems:'center'}}><Icon.Shield size={18}/></div>
          <div style={{flex:1}}>
            <div style={{fontSize: 14, fontWeight: 800}}>センシティブ設定</div>
            <div style={{fontSize: 11.5, color:'var(--fg-3)', marginTop: 2}}>ONにすると閲覧前に注意が表示されます。</div>
          </div>
          <div className={`switch tap ${sen?'on':''}`} onClick={()=>setSen(!sen)}/>
        </div>

        <div className="pc-card" style={{marginBottom: 18}}>
          <h3 className="pc-panel-title"><span className="ico"><Icon.Users size={14}/></span> 作成者情報</h3>
          <div style={{display:'grid', gridTemplateColumns:'1fr 1fr', gap: 14}}>
            <div>
              <label className="field-label">表示名</label>
              <input className="input" defaultValue="nekoma"/>
            </div>
            <div>
              <label className="field-label">X ID(任意)</label>
              <input className="input" defaultValue="@nekoma_vrc"/>
            </div>
          </div>
        </div>

        <div className="pc-card" style={{marginBottom: 18}}>
          <h3 className="pc-panel-title"><span className="ico"><Icon.Shield size={14}/></span> 権利・同意</h3>
          <div style={{display:'flex', gap: 10, marginBottom: 10}} className="tap" onClick={()=>setA1(!a1)}>
            <div className={`check ${a1?'on':''}`}/>
            <div style={{fontSize: 13, lineHeight: 1.55, color:'var(--fg-2)'}}>写っている人やワールド等に配慮し、公開して問題ない内容であることを確認しました。</div>
          </div>
          <div style={{display:'flex', gap: 10}} className="tap" onClick={()=>setA2(!a2)}>
            <div className={`check ${a2?'on':''}`}/>
            <div style={{fontSize: 13, lineHeight: 1.55, color:'var(--fg-2)'}}>VRC PhotoBookの <span style={{color:'var(--teal)', fontWeight:600}}>利用規約・ガイドライン</span> に同意します。</div>
          </div>
        </div>

        <div className="pc-card" style={{display:'flex', gap: 14, alignItems:'center', marginBottom: 22}}>
          <div className={`check ${bot?'on':''} tap`} onClick={()=>setBot(!bot)}/>
          <div style={{flex:1, fontSize: 13.5, fontWeight: 600}}>私はロボットではありません</div>
          <div style={{fontSize: 10, color:'var(--fg-4)', textAlign:'right', lineHeight: 1.3}}>
            <div style={{fontWeight: 700, fontSize: 10.5, color:'var(--fg-3)'}}>CLOUDFLARE</div>
            Turnstile
          </div>
        </div>

        <div style={{display:'flex', justifyContent:'center'}}>
          <button className="btn btn-primary btn-lg" style={{minWidth: 360}} onClick={()=>go('pc-complete')}>
            <Icon.Send size={14}/> 公開する
          </button>
        </div>
        <div style={{textAlign:'center', marginTop: 10, fontSize: 11, color:'var(--fg-4)', paddingBottom: 40}}>
          ログイン不要。公開後に管理URLを発行します。
        </div>
      </div>
    </PCBrowser>
  );
}

Object.assign(window, { PCLP, PCCreateStart, PCEdit, PCPublishSettings });
