// prepareLocalLabels の unit test。
//
// 観点:
//   - rememberLabel / lookupLabel の往復
//   - photobook 単位の名前空間（cross-photobook leak がない）
//   - TTL 超過 entry が GC される
//   - 上限 50 entry を超えると古いものから削除
//   - localStorage 不在環境（SSR / private mode）でも throw しない

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  clearLabels,
  lookupLabel,
  rememberLabel,
} from "@/lib/prepareLocalLabels";

type Store = Map<string, string>;

function installFakeLocalStorage(store: Store): void {
  vi.stubGlobal("window", {
    localStorage: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => {
        store.set(k, v);
      },
      removeItem: (k: string) => {
        store.delete(k);
      },
      clear: () => {
        store.clear();
      },
      get length() {
        return store.size;
      },
      key: (i: number) => Array.from(store.keys())[i] ?? null,
    },
  });
}

describe("prepareLocalLabels", () => {
  let store: Store;

  beforeEach(() => {
    store = new Map();
    installFakeLocalStorage(store);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("正常_rememberLabel 後に lookupLabel で取り出せる", () => {
    rememberLabel("pb-1", "img-aaa", "vacation.jpg", 1_000_000);
    expect(lookupLabel("pb-1", "img-aaa", 1_000_000)).toBe("vacation.jpg");
  });

  it("正常_別 photobook には漏れない（cross-photobook 名前空間）", () => {
    rememberLabel("pb-1", "img-aaa", "vacation.jpg", 1_000_000);
    expect(lookupLabel("pb-2", "img-aaa", 1_000_000)).toBeNull();
  });

  it("正常_TTL 超過 entry は null を返す", () => {
    rememberLabel("pb-1", "img-aaa", "old.jpg", 1_000_000);
    const after31days = 1_000_000 + 31 * 24 * 60 * 60 * 1000;
    expect(lookupLabel("pb-1", "img-aaa", after31days)).toBeNull();
  });

  it("正常_50 entry 超過時は古いものから削除", () => {
    for (let i = 0; i < 60; i++) {
      rememberLabel("pb-1", `img-${i}`, `f-${i}.jpg`, 1_000_000 + i);
    }
    // 古い順に削られているため img-0..9 は消える、img-10..59 は残る
    expect(lookupLabel("pb-1", "img-0", 1_000_100)).toBeNull();
    expect(lookupLabel("pb-1", "img-9", 1_000_100)).toBeNull();
    expect(lookupLabel("pb-1", "img-10", 1_000_100)).toBe("f-10.jpg");
    expect(lookupLabel("pb-1", "img-59", 1_000_100)).toBe("f-59.jpg");
  });

  it("正常_imageId 空 / filename 空は no-op", () => {
    rememberLabel("pb-1", "", "x.jpg", 1_000_000);
    rememberLabel("pb-1", "img-x", "", 1_000_000);
    expect(lookupLabel("pb-1", "img-x", 1_000_000)).toBeNull();
  });

  it("正常_clearLabels で photobook の全 entry が消える", () => {
    rememberLabel("pb-1", "img-aaa", "f1.jpg", 1_000_000);
    rememberLabel("pb-1", "img-bbb", "f2.jpg", 1_000_000);
    clearLabels("pb-1");
    expect(lookupLabel("pb-1", "img-aaa", 1_000_000)).toBeNull();
    expect(lookupLabel("pb-1", "img-bbb", 1_000_000)).toBeNull();
  });

  it("正常_window 不在（SSR）でも throw しない", () => {
    vi.unstubAllGlobals(); // window を消す
    expect(() => rememberLabel("pb-1", "img-x", "x.jpg", 1)).not.toThrow();
    expect(lookupLabel("pb-1", "img-x", 1)).toBeNull();
    expect(() => clearLabels("pb-1")).not.toThrow();
  });

  it("正常_localStorage 例外（QuotaExceeded など）でも throw しない", () => {
    vi.stubGlobal("window", {
      localStorage: {
        getItem: () => null,
        setItem: () => {
          throw new Error("QuotaExceeded");
        },
        removeItem: () => {
          throw new Error("Disabled");
        },
      },
    });
    expect(() => rememberLabel("pb-1", "img-x", "x.jpg", 1)).not.toThrow();
    expect(() => clearLabels("pb-1")).not.toThrow();
  });

  it("正常_lookupLabel が raw image_id 等を一切 return しない（filename のみ）", () => {
    const SECRET_IMG_ID = "img-secret-zzz1234";
    rememberLabel("pb-1", SECRET_IMG_ID, "label-only.jpg", 1_000_000);
    const got = lookupLabel("pb-1", SECRET_IMG_ID, 1_000_000);
    expect(got).toBe("label-only.jpg");
    expect(got).not.toContain(SECRET_IMG_ID);
  });
});
