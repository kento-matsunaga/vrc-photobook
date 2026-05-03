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
//
// m2-design-refresh STOP β-4 (本 commit、visual のみ):
//   - design `wf-screens-b.jsx:329-331` (M) / `:375-377` (PC) `wf-note.warn` 視覚整合
//     (`wireframe-styles.css:420-425` border-l + bg + ! icon)
//   - role / data-testid prop は **触らない**
//   - 既存文言「一時的に非公開」「公開ページと SNS プレビューには表示されません」「編集を」は維持

type Props = {
  testId?: string;
};

export function HiddenByOperatorBanner({ testId = "hidden-by-operator-banner" }: Props) {
  return (
    <section
      role="status"
      aria-live="polite"
      data-testid={testId}
      className="flex items-start gap-2.5 rounded-lg border-l-[3px] border-status-warning bg-status-warning-soft p-3.5"
    >
      <span
        aria-hidden="true"
        className="grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full bg-status-warning font-serif text-xs font-bold italic leading-none text-white"
      >
        !
      </span>
      <div className="flex-1 text-xs leading-[1.6]">
        <p className="font-bold text-status-warning">
          このフォトブックは運営により一時的に非公開になっています。
        </p>
        <p className="mt-1 text-ink-medium">
          公開ページと SNS プレビューには表示されません。内容を修正したい場合は編集を
          続けられます。再公開のタイミングは運営の判断によります。
        </p>
      </div>
    </section>
  );
}
