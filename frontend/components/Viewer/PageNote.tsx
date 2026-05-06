// PageNote: ページの引用ノート (creator の手記) をクリーム色背景で表示する。
//
// 採用元: TESTImage 完成イメージ「Note」ブロック
//
// 設計判断 (v2):
//   - note 空 / undefined → null (空 div を出さない)
//   - 改行を尊重 (\n → 行送り)、Markdown は採用しない (装飾過多回避)
//   - 装飾のための左の縦線 (teal-300) で「引用」アフォーダンス

type Props = {
  note?: string;
};

export function PageNote({ note }: Props) {
  const trimmed = (note ?? "").trim();
  if (trimmed === "") return null;
  return (
    <blockquote
      data-testid="page-note"
      className="relative whitespace-pre-line rounded-md border-l-[3px] border-teal-300 bg-[#fdf8ee] px-5 py-4 text-[14px] leading-[1.85] text-ink-strong shadow-sm sm:px-7 sm:py-5 sm:text-[15px]"
    >
      <span aria-hidden="true" className="absolute left-3 top-2 font-serif text-3xl leading-none text-teal-300/70">
        “
      </span>
      <span className="block pl-4 sm:pl-5">{trimmed}</span>
    </blockquote>
  );
}
