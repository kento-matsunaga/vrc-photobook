"use client";

// SensitiveGate: センシティブ設定 ON 時のワンクッション (同意するまで本文非表示)。
//
// 採用元: プロンプト §1「センシティブ設定 ON 時のワンクッション UI」
//
// 設計判断 (v2):
//   - Backend が isSensitive を返すまで dead path だが component は用意 (wiring だけで
//     済むよう先回り、Q4 確認)
//   - 永続化は state のみ (reload で再表示)。MVP では sessionStorage 不採用、
//     セキュリティ上「閉じてリロードで再警告」が安全側
//   - children は同意するまで render しない (DOM に出さない、SSR では isSensitive=false
//     扱いで dev / production 動作を変えない)
//
// セキュリティ:
//   - 同意状態は memory のみ。Cookie / localStorage に書かない (敵対者観測抑止)

import { useState, type ReactNode } from "react";

type Props = {
  isSensitive: boolean;
  children: ReactNode;
};

export function SensitiveGate({ isSensitive, children }: Props) {
  const [agreed, setAgreed] = useState<boolean>(false);

  if (!isSensitive) {
    return <>{children}</>;
  }
  if (agreed) {
    return <>{children}</>;
  }
  return (
    <div
      data-testid="viewer-sensitive-gate"
      role="region"
      aria-label="センシティブな内容の警告"
      className="mx-auto my-12 max-w-screen-md rounded-lg border border-amber-200 bg-amber-50 p-6 text-center shadow-sm sm:p-10"
    >
      <span
        aria-hidden="true"
        className="grid h-12 w-12 mx-auto place-items-center rounded-full bg-amber-200 font-serif text-xl font-bold italic text-amber-800"
      >
        !
      </span>
      <h2 className="mt-4 font-serif text-lg font-bold text-ink sm:text-xl">
        センシティブな内容を含む可能性があります
      </h2>
      <p className="mt-2 text-sm leading-relaxed text-ink-medium">
        このフォトブックには、創作者によりセンシティブ設定が付与されています。
        閲覧する場合は下の「同意して見る」を押してください。
      </p>
      <button
        type="button"
        data-testid="viewer-sensitive-agree"
        onClick={() => setAgreed(true)}
        className="mt-5 inline-flex h-11 items-center justify-center rounded-[10px] bg-amber-600 px-6 text-sm font-bold text-white shadow-sm transition-colors hover:bg-amber-700"
      >
        同意して見る
      </button>
    </div>
  );
}
