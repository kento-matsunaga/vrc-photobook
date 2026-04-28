// HiddenByOperatorBanner: hidden_by_operator=true の photobook 管理画面に表示する banner。
//
// 設計参照:
//   - docs/plan/m2-moderation-ops-plan.md §8.2 案 b（manage UI に「運営により非公開中」表示）
//
// 方針:
//   - 編集はブロックしない（作成者は内容を直せる）
//   - 管理 URL 再発行はトリガしない
//   - 公開ページ / SNS プレビューに表示されない旨を伝える
//   - 強い視覚的注意を与えるが、エラーではない（status-warning 系トーン）

type Props = {
  testId?: string;
};

export function HiddenByOperatorBanner({ testId = "hidden-by-operator-banner" }: Props) {
  return (
    <section
      role="status"
      aria-live="polite"
      data-testid={testId}
      className="rounded-lg border border-status-warning bg-status-warning-soft px-4 py-3 text-sm text-status-warning"
    >
      <p className="font-medium">このフォトブックは運営により一時的に非公開になっています。</p>
      <p className="mt-1 text-ink-medium">
        公開ページと SNS プレビューには表示されません。内容を修正したい場合は編集を
        続けられます。再公開のタイミングは運営の判断によります。
      </p>
    </section>
  );
}
