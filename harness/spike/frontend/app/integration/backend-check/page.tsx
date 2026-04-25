"use client";

/**
 * M1 PoC: Frontend / Backend 結合検証ページ
 *
 * 検証目的:
 *  - Frontend (OpenNext preview, Cloudflare Workers 互換ローカル) から
 *    Backend (Cloud Run 互換ローカル) へクロスオリジン fetch できるか
 *  - credentials: "include" で Cookie が引き渡せるか
 *  - CORS（Access-Control-Allow-Origin / Allow-Credentials）が成立するか
 *  - Origin ヘッダ検証が成立するか
 *  - SameSite=Strict + 異なるホストでの Cookie 引き渡し挙動
 *
 * 重要:
 *  - Cookie 値そのものは画面に表示しない（Backend からも値を返さない）
 *  - 表示するのは API レスポンスの「存在/不存在」「allowed」など分類値のみ
 *  - エラー詳細はサーバ側で追跡、画面はラフに status と body を表示
 */

import { useEffect, useState } from "react";

const DEFAULT_API_BASE = "http://localhost:8090";

type Outcome = {
  ok: boolean;
  status: number;
  body: unknown;
  error?: string;
};

export default function BackendCheckPage() {
  const [apiBase, setApiBase] = useState<string>(DEFAULT_API_BASE);
  const [results, setResults] = useState<Record<string, Outcome>>({});

  useEffect(() => {
    // クライアント側で参照する環境変数（NEXT_PUBLIC_* のみ反映される）
    const fromEnv = process.env.NEXT_PUBLIC_API_BASE_URL;
    if (fromEnv && fromEnv.trim() !== "") {
      setApiBase(fromEnv);
    }
  }, []);

  async function call(name: string, url: string, init: RequestInit) {
    try {
      const res = await fetch(url, init);
      const text = await res.text();
      let body: unknown = text;
      try {
        body = JSON.parse(text);
      } catch {
        // JSON でなければそのまま文字列
      }
      setResults((prev) => ({
        ...prev,
        [name]: { ok: res.ok, status: res.status, body },
      }));
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : String(e);
      setResults((prev) => ({
        ...prev,
        [name]: { ok: false, status: 0, body: null, error: message },
      }));
    }
  }

  const tests: Array<{
    name: string;
    label: string;
    run: () => Promise<void>;
    note?: string;
  }> = [
    {
      name: "healthz",
      label: "GET /healthz (no credentials)",
      run: () => call("healthz", `${apiBase}/healthz`, { credentials: "omit" }),
      note: "Backend 単純疎通確認。CORS 不要（Cookie 送らない）",
    },
    {
      name: "session_include",
      label: "GET /sandbox/session-check (credentials: include)",
      run: () =>
        call("session_include", `${apiBase}/sandbox/session-check`, {
          credentials: "include",
        }),
      note:
        "Cookie 引き渡し検証。draft/manage Cookie を Backend が見えるか。Backend の ALLOWED_ORIGINS に Frontend オリジンが含まれている必要がある",
    },
    {
      name: "session_omit",
      label: "GET /sandbox/session-check (credentials: omit)",
      run: () =>
        call("session_omit", `${apiBase}/sandbox/session-check`, {
          credentials: "omit",
        }),
      note: "比較用: Cookie が送られないので両方 false になるはず",
    },
    {
      name: "origin_post",
      label: "POST /sandbox/origin-check (credentials: include)",
      run: () =>
        call("origin_post", `${apiBase}/sandbox/origin-check`, {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({}),
        }),
      note:
        "Origin チェック検証。Frontend オリジンが ALLOWED_ORIGINS に含まれていれば 200、なければ 403",
    },
  ];

  return (
    <main>
      <h1>Frontend / Backend 結合検証 PoC</h1>
      <p>
        API base URL: <code>{apiBase}</code>
      </p>
      <p>
        <small>
          NEXT_PUBLIC_API_BASE_URL 未設定なら <code>{DEFAULT_API_BASE}</code> を
          使う。Cookie 値は表示しない。
        </small>
      </p>

      <h2>事前準備</h2>
      <ol>
        <li>
          Backend を起動しておく（例: <code>PORT=8090
          ALLOWED_ORIGINS=http://localhost:8787 /tmp/spike-api</code>）。
        </li>
        <li>
          先に <a href="/draft/sample-draft-token">/draft/sample-draft-token</a>{" "}
          / <a href="/manage/token/sample-manage-token">
            /manage/token/sample-manage-token
          </a>{" "}
          にアクセスし、Cookie を発行しておく（同一 Frontend オリジン内）。
        </li>
        <li>
          ただし <strong>localhost を異なるポートで分けると別オリジン</strong>
          になり、Frontend で発行された Cookie は Backend 側ホストには付かない。
          結合検証では同一オリジンで動かすか、または独自ドメイン経由で揃える必要がある（U2、ADR-0003）。
          詳細は本ページ末尾「ローカル HTTP の限界」を参照。
        </li>
      </ol>

      <h2>テスト実行</h2>
      <ul style={{ paddingLeft: 20 }}>
        {tests.map((t) => (
          <li key={t.name} style={{ marginBottom: "1rem" }}>
            <button
              onClick={() => {
                void t.run();
              }}
              style={{ marginRight: "0.5rem" }}
            >
              {t.label}
            </button>
            {t.note && (
              <span style={{ color: "#555" }}>
                <small> — {t.note}</small>
              </span>
            )}
            <Result outcome={results[t.name]} />
          </li>
        ))}
      </ul>

      <h2>結果 (生 JSON)</h2>
      <pre
        style={{
          background: "#f6f8fa",
          padding: "1rem",
          borderRadius: 4,
          overflowX: "auto",
        }}
      >
        {JSON.stringify(results, null, 2)}
      </pre>

      <h2>ローカル HTTP の限界</h2>
      <ul>
        <li>
          Frontend (port 8787) と Backend (port 8090) は <strong>別オリジン</strong>
          。SameSite=Strict + ホスト分離で Cookie は引き渡せない。
        </li>
        <li>
          結合検証で「session_include の Cookie 存在」が <code>true</code> に
          ならない場合、それは設計失敗ではなく ローカル HTTP + 異なるポートの
          仕様。実環境（共通の親ドメイン or 同一ホストプロキシ経由）で再確認する。
        </li>
        <li>
          <code>Secure</code> 属性付き Cookie は HTTP 経由では送られない。
          本 PoC は localhost を許容として動かしているが、本番では HTTPS 必須。
        </li>
      </ul>

      <h2>Safari で確認すべき項目</h2>
      <ul>
        <li>
          Web Inspector → Storage → Cookies で <code>vrcpb_draft_*</code> /
          <code>vrcpb_manage_*</code> が Frontend ホストに紐付いている
        </li>
        <li>
          別ホストの Backend に対して fetch すると ITP が cross-site Cookie
          として扱わないか
        </li>
        <li>同一オリジンなら credentials: "include" で Cookie が付くか</li>
      </ul>

      <h2>実環境デプロイ後に再確認する項目</h2>
      <ul>
        <li>共通親ドメイン下の `*.example.com` で Cookie Domain 動作</li>
        <li>HTTPS 下での Secure Cookie 引き渡し</li>
        <li>Cloudflare Workers + Cloud Run 構成での CORS / Origin 検証</li>
        <li>Safari ITP の長期影響（24h / 7 日）</li>
      </ul>

      <p>
        <a href="/">← トップへ戻る</a>
      </p>
    </main>
  );
}

function Result({ outcome }: { outcome?: Outcome }) {
  if (!outcome) {
    return (
      <div style={{ marginTop: "0.5rem", color: "#999" }}>
        <small>(未実行)</small>
      </div>
    );
  }
  const color = outcome.error
    ? "#c62828"
    : outcome.ok
    ? "#2e7d32"
    : "#ef6c00";
  return (
    <div
      style={{
        marginTop: "0.5rem",
        padding: "0.5rem 0.75rem",
        background: "#f6f8fa",
        borderLeft: `3px solid ${color}`,
        fontSize: "0.85rem",
      }}
    >
      {outcome.error ? (
        <span style={{ color: "#c62828" }}>error: {outcome.error}</span>
      ) : (
        <>
          <span>status: {outcome.status}</span>
          <pre style={{ margin: "0.25rem 0 0", fontSize: "0.8rem" }}>
            {JSON.stringify(outcome.body, null, 2)}
          </pre>
        </>
      )}
    </div>
  );
}
