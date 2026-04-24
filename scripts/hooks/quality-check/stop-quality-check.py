#!/usr/bin/env python3
"""
stop-quality-check.py — stop フック
エージェント停止時に品質パイプライン（build/lint/test）の
完了状態を確認し、未完了があればリマインダーを出力する。
"""

import json
import sys
from pathlib import Path

STATE_DIR = Path("/tmp/ai-driven-quality-state")
STATE_FILE = STATE_DIR / "quality-state.json"
MAX_LOOP_COUNT = 4  # 無限ループ防止


def load_state() -> dict:
    """状態ファイルを読み込み"""
    if STATE_FILE.exists():
        try:
            return json.loads(STATE_FILE.read_text())
        except (json.JSONDecodeError, IOError):
            pass
    return {}


def main():
    state = load_state()

    # 追跡中のファイルがなければスキップ
    touched_files = state.get("touched_files", {})
    if not touched_files:
        return

    build_ok = state.get("build_ok", False)
    lint_ok = state.get("lint_ok", False)
    test_ok = state.get("test_ok", False)

    # 全チェック完了していればスキップ
    if build_ok and lint_ok and test_ok:
        # 状態をクリア
        STATE_FILE.unlink(missing_ok=True)
        return

    # ループカウント管理
    loop_count = state.get("loop_count", 0) + 1
    state["loop_count"] = loop_count

    if loop_count > MAX_LOOP_COUNT:
        print("⚠️ 品質チェックリマインダーが上限に達しました。手動で確認してください。")
        STATE_FILE.unlink(missing_ok=True)
        return

    # 未完了チェックのリスト作成
    missing = []
    if not build_ok:
        missing.append("build")
    if not lint_ok:
        missing.append("lint")
    if not test_ok:
        missing.append("test")

    # リマインダー出力
    print("")
    print("==========================================")
    print("⚠️  品質パイプラインが未完了です")
    print("==========================================")
    print("")
    print(f"変更ファイル数: {len(touched_files)}")
    for filepath in list(touched_files.keys())[:10]:
        print(f"  - {filepath}")
    if len(touched_files) > 10:
        print(f"  ... 他 {len(touched_files) - 10} ファイル")
    print("")
    print("未実行チェック:")
    for check in missing:
        print(f"  ❌ {check}")
    print("")
    print("完了チェック:")
    for check in ["build", "lint", "test"]:
        if state.get(f"{check}_ok", False):
            print(f"  ✅ {check}")
    print("")
    print("品質チェックを実行してから停止してください。")
    print("==========================================")

    # 状態を保存（ループカウント更新）
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    STATE_FILE.write_text(json.dumps(state, indent=2, ensure_ascii=False))

    # 非ゼロ終了でリマインダーとして機能
    sys.exit(1)


if __name__ == "__main__":
    main()
