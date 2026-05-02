// uploadVerificationCache のユニットテスト。
//
// 観点（Issue A hotfix の核となる挙動）:
//   - 並列 ensure 呼び出しでも issue は **1 回だけ**実行される（race condition 回避）
//   - 取得済の token は次回以降そのまま返される（複数 upload-intent 共有）
//   - 失敗時は inflight / token が破棄され、次の ensure で再試行可能
//   - reset で明示破棄できる
//   - issue 関数に渡される turnstileToken は最初の呼び出し分のみ（後続は in-flight に合流）

import { describe, expect, it } from "vitest";

import {
  createUploadVerificationCache,
  type IssueVerificationFn,
} from "@/lib/uploadVerificationCache";

/** 制御可能な issue 関数を作る helper。 */
function makeControlledIssue(): {
  fn: IssueVerificationFn;
  callCount: () => number;
  receivedTokens: () => string[];
  resolveOnce: (vtok: string) => void;
  rejectOnce: (err: unknown) => void;
} {
  let count = 0;
  const tokens: string[] = [];
  let resolver: ((v: string) => void) | null = null;
  let rejecter: ((e: unknown) => void) | null = null;

  const fn: IssueVerificationFn = async (turnstileToken) => {
    count += 1;
    tokens.push(turnstileToken);
    return await new Promise<{ uploadVerificationToken: string }>((resolve, reject) => {
      resolver = (v: string) => resolve({ uploadVerificationToken: v });
      rejecter = (e: unknown) => reject(e);
    });
  };

  return {
    fn,
    callCount: () => count,
    receivedTokens: () => [...tokens],
    resolveOnce: (vtok: string) => {
      const r = resolver;
      resolver = null;
      rejecter = null;
      r?.(vtok);
    },
    rejectOnce: (err: unknown) => {
      const r = rejecter;
      resolver = null;
      rejecter = null;
      r?.(err);
    },
  };
}

describe("createUploadVerificationCache concurrency", () => {
  it("正常_並列ensure_2件で issue は 1 回だけ呼ばれる_両方が同 token を得る", async () => {
    const ctrl = makeControlledIssue();
    const cache = createUploadVerificationCache(ctrl.fn);

    // 同期に 2 回 ensure を呼ぶ（concurrency=2 並列 upload を simulate）
    const p1 = cache.ensure("ts-token-1st");
    const p2 = cache.ensure("ts-token-2nd-ignored");

    // microtask が動くのを 1 度だけ進めても、issue 内の await はまだ pending
    await Promise.resolve();
    expect(ctrl.callCount()).toBe(1); // ← 重要: 二重実行回避
    expect(ctrl.receivedTokens()).toEqual(["ts-token-1st"]);

    // 1 回の issue が解決すると両 ensure が同じ token を得る
    ctrl.resolveOnce("vtok-shared");
    const [r1, r2] = await Promise.all([p1, p2]);
    expect(r1).toBe("vtok-shared");
    expect(r2).toBe("vtok-shared");
    expect(cache.current).toBe("vtok-shared");
    expect(ctrl.callCount()).toBe(1); // 最後まで 1 回
  });

  it("正常_3件並列 ensure でも issue は 1 回だけ", async () => {
    const ctrl = makeControlledIssue();
    const cache = createUploadVerificationCache(ctrl.fn);

    const ps = [
      cache.ensure("ts-1"),
      cache.ensure("ts-2"),
      cache.ensure("ts-3"),
    ];
    await Promise.resolve();
    expect(ctrl.callCount()).toBe(1);

    ctrl.resolveOnce("vtok-x");
    const results = await Promise.all(ps);
    expect(results).toEqual(["vtok-x", "vtok-x", "vtok-x"]);
  });

  it("正常_取得済 token がある状態で ensure すると issue は呼ばれない", async () => {
    const ctrl = makeControlledIssue();
    const cache = createUploadVerificationCache(ctrl.fn);

    const p1 = cache.ensure("ts-1");
    await Promise.resolve();
    ctrl.resolveOnce("vtok-cached");
    await p1;

    expect(ctrl.callCount()).toBe(1);

    // 取得後の追加 ensure は即座に同 token を返す（issue は再度呼ばれない）
    const r2 = await cache.ensure("ts-2");
    const r3 = await cache.ensure("ts-3");
    expect(r2).toBe("vtok-cached");
    expect(r3).toBe("vtok-cached");
    expect(ctrl.callCount()).toBe(1);
  });
});

describe("createUploadVerificationCache failure / retry", () => {
  it("正常_失敗時は inflight / token が破棄され、次の ensure で再試行可能", async () => {
    const ctrl = makeControlledIssue();
    const cache = createUploadVerificationCache(ctrl.fn);

    const p1 = cache.ensure("ts-1");
    await Promise.resolve();
    expect(ctrl.callCount()).toBe(1);

    const err1 = { kind: "verification_failed" };
    ctrl.rejectOnce(err1);
    await expect(p1).rejects.toEqual(err1);

    expect(cache.current).toBe(""); // 破棄されている

    // 別 turnstileToken で再試行 → issue が再度呼ばれる
    const p2 = cache.ensure("ts-fresh");
    await Promise.resolve();
    expect(ctrl.callCount()).toBe(2);
    expect(ctrl.receivedTokens()).toEqual(["ts-1", "ts-fresh"]);

    ctrl.resolveOnce("vtok-after-retry");
    expect(await p2).toBe("vtok-after-retry");
    expect(cache.current).toBe("vtok-after-retry");
  });

  it("正常_失敗中の並列 ensure はどちらも同じ rejection を受ける", async () => {
    const ctrl = makeControlledIssue();
    const cache = createUploadVerificationCache(ctrl.fn);

    const p1 = cache.ensure("ts-1");
    const p2 = cache.ensure("ts-2");
    await Promise.resolve();
    expect(ctrl.callCount()).toBe(1);

    const err = { kind: "rate_limited", retryAfterSeconds: 60 };
    ctrl.rejectOnce(err);

    await expect(p1).rejects.toEqual(err);
    await expect(p2).rejects.toEqual(err);
  });
});

describe("createUploadVerificationCache reset", () => {
  it("正常_reset 後の ensure は issue を再度呼ぶ", async () => {
    const ctrl = makeControlledIssue();
    const cache = createUploadVerificationCache(ctrl.fn);

    const p1 = cache.ensure("ts-1");
    await Promise.resolve();
    ctrl.resolveOnce("vtok-1");
    await p1;
    expect(cache.current).toBe("vtok-1");

    cache.reset();
    expect(cache.current).toBe("");

    const p2 = cache.ensure("ts-2");
    await Promise.resolve();
    expect(ctrl.callCount()).toBe(2);
    ctrl.resolveOnce("vtok-2");
    expect(await p2).toBe("vtok-2");
    expect(cache.current).toBe("vtok-2");
  });

  // 注: in-flight 中の reset の corner case は本 hotfix の対象外（現実の使用 path では
  // verification 完了後に reset を呼ぶ。並列 reset + ensure の race は別途検討）。
});

describe("createUploadVerificationCache turnstileToken passing", () => {
  it("正常_最初の ensure 呼び出しで渡された turnstileToken のみ issue に渡る", async () => {
    const ctrl = makeControlledIssue();
    const cache = createUploadVerificationCache(ctrl.fn);

    const p1 = cache.ensure("ts-FIRST");
    const p2 = cache.ensure("ts-SECOND");
    const p3 = cache.ensure("ts-THIRD");
    await Promise.resolve();

    // 後続 2 件は in-flight に合流するため、turnstileToken は捨てられる（race fix の必然）
    expect(ctrl.receivedTokens()).toEqual(["ts-FIRST"]);

    ctrl.resolveOnce("vtok-x");
    await Promise.all([p1, p2, p3]);
    expect(cache.current).toBe("vtok-x");
  });
});
