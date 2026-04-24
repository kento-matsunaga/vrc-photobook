---
description: "テスト実装統合ガイド — テストファイル全般に適用"
globs: ["**/*_test.go", "**/*.test.ts", "**/*.test.tsx", "**/*.spec.ts", "**/*_test.py"]
---

# テスト実装統合ガイド

## 必須パターン

### 1. テーブル駆動テスト
すべてのテストはテーブル駆動で記述する。フラットな `t.Run()` の列挙は禁止。

```go
// ✅ 正しい: テーブル駆動
tests := []struct {
    name        string
    description string // BDD: Given-When-Then
    setup       func()
    input       InputType
    want        OutputType
    wantErr     bool
}{
    {
        name:        "正常_基本ケース",
        description: "Given: 有効な入力, When: 処理実行, Then: 期待結果を返す",
        input:       validInput,
        want:        expectedOutput,
    },
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // ...
    })
}

// ❌ 禁止: フラット列挙
t.Run("ケース1", func(t *testing.T) { /* ... */ })
t.Run("ケース2", func(t *testing.T) { /* ... */ })
```

### 2. description フィールド必須
テストケースには `description` フィールドで Given/When/Then を記述する。
テスト名（`name`）は短い識別子、`description` がテストの意図を伝える。

### 3. Builder パターン（メソッドテスト用）
テスト対象メソッドの前提条件構築には Builder を使用する。

```go
// ✅ 正しい: Builder でメソッドテストの前提条件を構築
task := tests.NewTaskBuilder().
    WithStatus(StatusActive).
    WithAssignee(userID).
    Build(t)
result := task.Complete()

// ❌ 禁止: メソッドテストで50行の手動セットアップ
```

### 4. コンストラクタテストは直接構築
`New*()` 関数のテストでは Builder を使わず、引数を直接指定する。

```go
// ✅ 正しい: New関数テストは直接引数
result, err := NewTask(id, name, status)
require.NoError(t, err)

// ❌ 禁止: New関数テストでBuilder使用（検証対象を隠蔽する）
```

## 禁止事項

1. **テストファイル内のヘルパー関数** — 前提条件が隠蔽される。Builder に集約する
2. **fixture.go ファイル** — 暗黙のテストデータは追跡不能。Builder で明示的に生成
3. **Builder に `t` を保持** — テーブルテストで再利用不能になる。`Build(t)` で受け取る
4. **テスト観点による関数分割** — 1つの概念は1つのテスト関数でテーブル駆動
5. **ループ内のインデックス条件分岐** — テーブルの各ケースが自己完結すべき

## テスト階層

| レイヤー | テスト内容 | DB必要 |
|---------|----------|-------|
| ドメインモデル | コンストラクタ、ビジネスメソッド、境界値 | No |
| VO (値オブジェクト) | コンストラクタ、等価性、不変性 | No |
| ユースケース | ビジネスフロー（Repository mock可） | No |
| リポジトリ | データ永続化・取得（実DB必須） | Yes |
| コントローラー | API統合（ユースケースは実物必須） | Depends |

## Why

エージェントがテスト実装時に以下の問題を繰り返したため:
- フラット `t.Run()` → テスト構造が崩壊し、追加・修正が困難に
- Builder の `t` 保持 → テーブルテストの各ケースが独立せず相互干渉
- ヘルパー関数 → テストの意図が読み取れず、レビュー不能に
- fixture.go → テストデータの出自が不明で、変更時の影響範囲が特定不能
