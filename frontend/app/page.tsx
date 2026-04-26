// PR4: トップページ最小実装。
// 確認したいこと:
//   - Next.js App Router が起動して SSR でページが返る
//   - Tailwind ユーティリティクラスが適用される
//   - layout.tsx の Metadata が反映される
// PR5 以降で本格 LP / OGP / 公開ページを整備する。
export default function HomePage() {
  return (
    <main className="mx-auto max-w-2xl px-4 py-12">
      <h1 className="text-3xl font-bold tracking-tight">VRC PhotoBook</h1>
      <p className="mt-4 text-base text-gray-600">
        VRChat 向けフォトブックサービス（非公式ファンメイド）の本実装スケルトンです。
      </p>
      <p className="mt-2 text-sm text-gray-500">
        現在 PR4: Next.js 15 + Tailwind の最小骨格。後続 PR でヘッダ制御 / OGP / 各画面 / Workers
        デプロイを段階的に追加します。
      </p>
    </main>
  );
}
