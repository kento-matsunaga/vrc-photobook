// UploadQueue 純関数の unit test。
//
// 観点:
//   - addFiles: queued tile を追加
//   - selectNextRunnable: concurrency 制御
//   - markStatus: 状態遷移
//   - reconcileWithServer: imageId placed / processingCount=0 での failed 推定
//   - isAllSettled / canProceedToEdit: enable 判定
//   - pollDelaySeconds: backoff sequence

import { describe, expect, it } from "vitest";

import {
  addFiles,
  activeUploadCount,
  canProceedToEdit,
  emptyQueue,
  imageIdOf,
  isAllSettled,
  markStatus,
  mergeServerImages,
  pollDelaySeconds,
  reconcileWithServer,
  selectNextRunnable,
  summary,
  type QueueTile,
  type ServerImageForMerge,
} from "@/components/Prepare/UploadQueue";

// File を test 用に組み立てる helper。
function fakeFile(name: string, size = 1024): File {
  return new File([new Uint8Array(size)], name, { type: "image/jpeg" });
}

function idGen(prefix = "t"): () => string {
  let n = 0;
  return () => `${prefix}-${++n}`;
}

describe("addFiles", () => {
  it("正常_空 queue に複数 File を queued tile として追加", () => {
    const s0 = emptyQueue();
    const s1 = addFiles(s0, [fakeFile("a.jpg"), fakeFile("b.jpg")], idGen());
    expect(s1.tiles).toHaveLength(2);
    expect(s1.tiles[0].id).toBe("t-1");
    expect(s1.tiles[0].status.kind).toBe("queued");
    expect(s1.tiles[1].id).toBe("t-2");
    expect(s1.tiles[1].status.kind).toBe("queued");
  });

  it("正常_既存 tile に追加で末尾に append", () => {
    const s0 = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen("a"));
    const s1 = addFiles(s0, [fakeFile("b.jpg")], idGen("b"));
    expect(s1.tiles.map((t) => t.id)).toEqual(["a-1", "b-1"]);
  });

  it("正常_空配列 add は no-op", () => {
    const s0 = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    const s1 = addFiles(s0, [], idGen());
    expect(s1.tiles).toHaveLength(1);
  });
});

describe("selectNextRunnable / activeUploadCount", () => {
  it("正常_active=0 で queued がある場合は最初の queued を返す", () => {
    const s = addFiles(
      emptyQueue(),
      [fakeFile("a.jpg"), fakeFile("b.jpg")],
      idGen(),
    );
    const t = selectNextRunnable(s, 2);
    expect(t?.id).toBe("t-1");
  });

  it("正常_concurrency 上限到達で null", () => {
    let s = addFiles(
      emptyQueue(),
      [fakeFile("a.jpg"), fakeFile("b.jpg"), fakeFile("c.jpg")],
      idGen(),
    );
    s = markStatus(s, "t-1", { kind: "uploading" });
    s = markStatus(s, "t-2", { kind: "verifying" });
    expect(activeUploadCount(s)).toBe(2);
    expect(selectNextRunnable(s, 2)).toBeNull();
  });

  it("正常_active が concurrency 未満なら次の queued を返す", () => {
    let s = addFiles(
      emptyQueue(),
      [fakeFile("a.jpg"), fakeFile("b.jpg"), fakeFile("c.jpg")],
      idGen(),
    );
    s = markStatus(s, "t-1", { kind: "uploading" });
    expect(activeUploadCount(s)).toBe(1);
    expect(selectNextRunnable(s, 2)?.id).toBe("t-2");
  });

  it("正常_全 tile が terminal なら null", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "available", imageId: "img-1" });
    expect(selectNextRunnable(s, 2)).toBeNull();
  });
});

describe("markStatus / 状態遷移", () => {
  it("正常_queued → verifying → uploading → completing → processing → available", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "verifying" });
    expect(s.tiles[0].status.kind).toBe("verifying");
    s = markStatus(s, "t-1", { kind: "uploading" });
    expect(s.tiles[0].status.kind).toBe("uploading");
    s = markStatus(s, "t-1", { kind: "completing" });
    expect(s.tiles[0].status.kind).toBe("completing");
    s = markStatus(s, "t-1", { kind: "processing", imageId: "img-1" });
    expect(s.tiles[0].status.kind).toBe("processing");
    s = markStatus(s, "t-1", { kind: "available", imageId: "img-1" });
    expect(s.tiles[0].status.kind).toBe("available");
  });

  it("正常_他 tile は変化しない", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg"), fakeFile("b.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "uploading" });
    expect(s.tiles[0].status.kind).toBe("uploading");
    expect(s.tiles[1].status.kind).toBe("queued");
  });

  it("正常_failed 遷移に reason を含める", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "failed", reason: "verification_failed" });
    const status = s.tiles[0].status;
    expect(status.kind).toBe("failed");
    expect(status.kind === "failed" ? status.reason : "").toBe("verification_failed");
  });
});

describe("reconcileWithServer", () => {
  it("正常_imageId が placed に含まれる processing tile は available に遷移", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "processing", imageId: "img-1" });
    const reconciled = reconcileWithServer(s, new Set(["img-1"]), 0);
    const status = reconciled.tiles[0].status;
    expect(status.kind).toBe("available");
    expect(status.kind === "available" ? status.imageId : "").toBe("img-1");
  });

  it("正常_processingCount=0 で未配置 → failed (processing_failed)", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "processing", imageId: "img-1" });
    const reconciled = reconcileWithServer(s, new Set(), 0);
    const status = reconciled.tiles[0].status;
    expect(status.kind).toBe("failed");
    expect(status.kind === "failed" ? status.reason : "").toBe("processing_failed");
  });

  it("正常_processingCount>0 で未配置はそのまま processing 据え置き", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "processing", imageId: "img-1" });
    const reconciled = reconcileWithServer(s, new Set(), 3);
    expect(reconciled.tiles[0].status.kind).toBe("processing");
  });

  it("正常_processing 以外の status は touchしない", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "uploading" });
    const reconciled = reconcileWithServer(s, new Set(["img-1"]), 0);
    expect(reconciled.tiles[0].status.kind).toBe("uploading");
  });
});

describe("isAllSettled / canProceedToEdit", () => {
  it("正常_空 queue は isAllSettled=false", () => {
    expect(isAllSettled(emptyQueue())).toBe(false);
  });

  it("正常_全 available なら isAllSettled=true", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg"), fakeFile("b.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "available", imageId: "i1" });
    s = markStatus(s, "t-2", { kind: "available", imageId: "i2" });
    expect(isAllSettled(s)).toBe(true);
  });

  it("正常_available + failed 混在も isAllSettled=true", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg"), fakeFile("b.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "available", imageId: "i1" });
    s = markStatus(s, "t-2", { kind: "failed", reason: "upload_failed" });
    expect(isAllSettled(s)).toBe(true);
  });

  it("正常_processing 残ありなら isAllSettled=false", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg"), fakeFile("b.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "available", imageId: "i1" });
    s = markStatus(s, "t-2", { kind: "processing", imageId: "i2" });
    expect(isAllSettled(s)).toBe(false);
  });

  it("正常_canProceedToEdit_全 available + serverProcessing=0 + 1 件以上 = true", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "available", imageId: "i1" });
    expect(canProceedToEdit(s, 0, 1)).toBe(true);
  });

  it("異常_canProceedToEdit_serverProcessing>0 で false", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "available", imageId: "i1" });
    expect(canProceedToEdit(s, 1, 1)).toBe(false);
  });

  it("異常_canProceedToEdit_全 failed では先に進ませない", () => {
    let s = addFiles(emptyQueue(), [fakeFile("a.jpg")], idGen());
    s = markStatus(s, "t-1", { kind: "failed", reason: "upload_failed" });
    expect(canProceedToEdit(s, 0, 0)).toBe(false);
  });

  it("正常_canProceedToEdit_空 queue でも server に photo があれば true（戻ってきたユーザ）", () => {
    expect(canProceedToEdit(emptyQueue(), 0, 3)).toBe(true);
  });
});

describe("summary", () => {
  it("正常_各 status の集計", () => {
    let s = addFiles(
      emptyQueue(),
      [
        fakeFile("a.jpg"),
        fakeFile("b.jpg"),
        fakeFile("c.jpg"),
        fakeFile("d.jpg"),
        fakeFile("e.jpg"),
        fakeFile("f.jpg"),
      ],
      idGen(),
    );
    s = markStatus(s, "t-1", { kind: "queued" });
    s = markStatus(s, "t-2", { kind: "uploading" });
    s = markStatus(s, "t-3", { kind: "verifying" });
    s = markStatus(s, "t-4", { kind: "processing", imageId: "i4" });
    s = markStatus(s, "t-5", { kind: "available", imageId: "i5" });
    s = markStatus(s, "t-6", { kind: "failed", reason: "network" });
    const sum = summary(s);
    expect(sum).toEqual({
      total: 6,
      queued: 1,
      active: 2,
      processing: 1,
      available: 1,
      failed: 1,
    });
  });
});

describe("pollDelaySeconds (exponential backoff)", () => {
  it("正常_tick 0,1 = 5s / tick 2 = 10s / tick 3 = 20s / tick 4+ = 60s", () => {
    expect(pollDelaySeconds(0)).toBe(5);
    expect(pollDelaySeconds(1)).toBe(5);
    expect(pollDelaySeconds(2)).toBe(10);
    expect(pollDelaySeconds(3)).toBe(20);
    expect(pollDelaySeconds(4)).toBe(60);
    expect(pollDelaySeconds(20)).toBe(60);
  });
});

describe("mergeServerImages (β-3 reload restore)", () => {
  type Case = {
    name: string;
    description: string;
    initialQueue: () => ReturnType<typeof emptyQueue>;
    serverImages: ServerImageForMerge[];
    placedImageIds: Set<string>;
    labelLookup?: (imageId: string) => string | null;
    assert: (tiles: QueueTile[]) => void;
  };

  const tests: Case[] = [
    {
      name: "正常_空 queue に server processing image を新規 server-restored tile として追加",
      description: "Given: 空 queue + server に processing image 1 件, When: merge, Then: 1 tile が origin=server status=processing で追加",
      initialQueue: () => emptyQueue(),
      serverImages: [
        {
          imageId: "img-aaa",
          status: "processing",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
      ],
      placedImageIds: new Set(),
      assert: (tiles) => {
        expect(tiles).toHaveLength(1);
        expect(tiles[0].origin).toBe("server");
        expect(tiles[0].status.kind).toBe("processing");
        expect(tiles[0].file).toBeUndefined();
        expect(tiles[0].byteSize).toBe(1_000_000);
        expect(tiles[0].displayLabel).toBe("復元された画像");
      },
    },
    {
      name: "正常_labelLookup から filename を復元",
      description: "Given: localStorage に filename がある, When: merge, Then: displayLabel が filename になる",
      initialQueue: () => emptyQueue(),
      serverImages: [
        {
          imageId: "img-bbb",
          status: "available",
          originalByteSize: 2_000_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
      ],
      placedImageIds: new Set(),
      labelLookup: (id) => (id === "img-bbb" ? "vacation.jpg" : null),
      assert: (tiles) => {
        expect(tiles).toHaveLength(1);
        expect(tiles[0].displayLabel).toBe("vacation.jpg");
        expect(tiles[0].status.kind).toBe("available");
      },
    },
    {
      name: "正常_既に placed の image は queue から除外",
      description: "Given: server image が placedImageIds 含む, When: merge, Then: tile に追加されない",
      initialQueue: () => emptyQueue(),
      serverImages: [
        {
          imageId: "img-ccc",
          status: "available",
          originalByteSize: 1_500_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
      ],
      placedImageIds: new Set(["img-ccc"]),
      assert: (tiles) => {
        expect(tiles).toHaveLength(0);
      },
    },
    {
      name: "正常_既存 local tile (imageId 一致) の status を server で更新",
      description: "Given: queue に processing tile, When: server で available, Then: tile.status が available に",
      initialQueue: () => {
        let s = addFiles(emptyQueue(), [new File([new Uint8Array(8)], "x.jpg", { type: "image/jpeg" })], () => "t-1");
        s = markStatus(s, "t-1", { kind: "processing", imageId: "img-ddd" });
        return s;
      },
      serverImages: [
        {
          imageId: "img-ddd",
          status: "available",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
      ],
      placedImageIds: new Set(),
      assert: (tiles) => {
        expect(tiles).toHaveLength(1);
        expect(tiles[0].id).toBe("t-1");
        expect(tiles[0].origin).toBe("local"); // 既存 tile を維持
        expect(tiles[0].status.kind).toBe("available");
      },
    },
    {
      name: "正常_failed image を tile failed (processing_failed) で復元、raw failure_reason は出さない",
      description: "Given: server に failed image, When: merge, Then: tile.status.reason='processing_failed'",
      initialQueue: () => emptyQueue(),
      serverImages: [
        {
          imageId: "img-eee",
          status: "failed",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
      ],
      placedImageIds: new Set(),
      assert: (tiles) => {
        expect(tiles).toHaveLength(1);
        expect(tiles[0].status.kind).toBe("failed");
        if (tiles[0].status.kind === "failed") {
          expect(tiles[0].status.reason).toBe("processing_failed");
        }
      },
    },
    {
      name: "正常_uploading image は tile uploading で復元",
      description: "Given: server に uploading image, When: merge, Then: tile.status.kind='uploading'",
      initialQueue: () => emptyQueue(),
      serverImages: [
        {
          imageId: "img-fff",
          status: "uploading",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
      ],
      placedImageIds: new Set(),
      assert: (tiles) => {
        expect(tiles).toHaveLength(1);
        expect(tiles[0].status.kind).toBe("uploading");
      },
    },
    {
      name: "正常_local upload chain 中 tile (imageId 未割当) は影響しない",
      description: "Given: queue に queued tile (imageId なし), When: merge, Then: そのまま保持",
      initialQueue: () => addFiles(emptyQueue(), [new File([new Uint8Array(8)], "y.jpg", { type: "image/jpeg" })], () => "t-1"),
      serverImages: [],
      placedImageIds: new Set(),
      assert: (tiles) => {
        expect(tiles).toHaveLength(1);
        expect(tiles[0].id).toBe("t-1");
        expect(tiles[0].status.kind).toBe("queued");
      },
    },
    {
      name: "正常_既存 tile の imageId が placedImageIds に含まれたら tile を queue から削除",
      description: "Given: queue に processing tile, When: server が placed に移動, Then: tile が削除される",
      initialQueue: () => {
        let s = addFiles(emptyQueue(), [new File([new Uint8Array(8)], "z.jpg", { type: "image/jpeg" })], () => "t-1");
        s = markStatus(s, "t-1", { kind: "processing", imageId: "img-ggg" });
        return s;
      },
      serverImages: [
        {
          imageId: "img-ggg",
          status: "available",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:00Z",
        },
      ],
      placedImageIds: new Set(["img-ggg"]),
      assert: (tiles) => {
        expect(tiles).toHaveLength(0);
      },
    },
    {
      name: "正常_複数 server images が createdAt 順で追加される",
      description: "Given: 3 server images (createdAt 異なる), When: merge, Then: 古い順に並ぶ",
      initialQueue: () => emptyQueue(),
      serverImages: [
        {
          imageId: "img-third",
          status: "available",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:30Z",
        },
        {
          imageId: "img-first",
          status: "available",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:10Z",
        },
        {
          imageId: "img-second",
          status: "available",
          originalByteSize: 1_000_000,
          createdAt: "2026-05-02T00:00:20Z",
        },
      ],
      placedImageIds: new Set(),
      assert: (tiles) => {
        expect(tiles).toHaveLength(3);
        // 各 tile.id は idGen 注入で順に t-1, t-2, t-3 になる（追加順 = createdAt 順）
        expect(tiles.map((t) => t.id)).toEqual(["t-1", "t-2", "t-3"]);
      },
    },
  ];

  for (const tt of tests) {
    it(tt.name, () => {
      const idGen = (): (() => string) => {
        let n = 0;
        return () => `t-${++n}`;
      };
      const lookup = tt.labelLookup ?? (() => null);
      const merged = mergeServerImages(
        tt.initialQueue(),
        tt.serverImages,
        tt.placedImageIds,
        lookup,
        idGen(),
      );
      tt.assert(merged.tiles);
    });
  }
});

describe("imageIdOf", () => {
  it("正常_processing tile の imageId を返す", () => {
    const t: QueueTile = {
      id: "t-1",
      displayLabel: "x",
      byteSize: 0,
      origin: "local",
      status: { kind: "processing", imageId: "img-x" },
    };
    expect(imageIdOf(t)).toBe("img-x");
  });

  it("正常_queued tile は null", () => {
    const t: QueueTile = {
      id: "t-1",
      displayLabel: "x",
      byteSize: 0,
      origin: "local",
      status: { kind: "queued" },
    };
    expect(imageIdOf(t)).toBeNull();
  });

  it("正常_failed tile (imageId なし) は null", () => {
    const t: QueueTile = {
      id: "t-1",
      displayLabel: "x",
      byteSize: 0,
      origin: "local",
      status: { kind: "failed", reason: "network" },
    };
    expect(imageIdOf(t)).toBeNull();
  });
});
