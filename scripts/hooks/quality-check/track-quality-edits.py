#!/usr/bin/env python3
"""
track-quality-edits.py — afterFileEdit フック
ソースファイルの変更を追跡し、品質パイプライン（build/lint/test）の
実行状態を管理する。
"""

import json
import os
import sys
import hashlib
from pathlib import Path

STATE_DIR = Path("/tmp/ai-driven-quality-state")
STATE_FILE = STATE_DIR / "quality-state.json"

# 追跡対象の拡張子（テストファイルは除外）
SOURCE_EXTENSIONS = {
    ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt",
    ".rb", ".php", ".cs", ".swift", ".dart",
}

# テストファイルパターン
TEST_PATTERNS = [
    "_test.go", ".test.ts", ".test.tsx", ".test.js", ".test.jsx",
    ".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx",
    "_test.py", "_test.rs",
]


def is_source_file(filepath: str) -> bool:
    """追跡対象のソースファイルか判定"""
    path = Path(filepath)
    if path.suffix not in SOURCE_EXTENSIONS:
        return False
    # テストファイルは除外
    for pattern in TEST_PATTERNS:
        if filepath.endswith(pattern):
            return False
    return True


def file_hash(filepath: str) -> str:
    """ファイルのハッシュを計算"""
    try:
        content = Path(filepath).read_bytes()
        return hashlib.md5(content).hexdigest()[:12]
    except (FileNotFoundError, PermissionError):
        return "unknown"


def load_state() -> dict:
    """状態ファイルを読み込み"""
    if STATE_FILE.exists():
        try:
            return json.loads(STATE_FILE.read_text())
        except (json.JSONDecodeError, IOError):
            pass
    return {
        "touched_files": {},
        "build_ok": False,
        "lint_ok": False,
        "test_ok": False,
        "build_hash": "",
        "lint_hash": "",
        "test_hash": "",
    }


def save_state(state: dict) -> None:
    """状態ファイルを保存"""
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    STATE_FILE.write_text(json.dumps(state, indent=2, ensure_ascii=False))


def main():
    # 環境変数またはstdinからファイルパスを取得
    filepath = os.environ.get("FILEPATH", "")

    if not filepath:
        # stdinからJSON入力を試行（Claude Codeフック形式）
        try:
            input_data = json.loads(sys.stdin.read())
            filepath = input_data.get("filepath", input_data.get("file_path", ""))
        except (json.JSONDecodeError, IOError):
            pass

    if not filepath or not is_source_file(filepath):
        return

    state = load_state()

    # ファイル変更を記録
    fhash = file_hash(filepath)
    state["touched_files"][filepath] = fhash

    # ファイルが変更されたので品質チェックをリセット
    state["build_ok"] = False
    state["lint_ok"] = False
    state["test_ok"] = False

    save_state(state)


if __name__ == "__main__":
    main()
