#!/usr/bin/env python3
"""
track-quality-execution.py — afterShellExecution フック
build/lint/test コマンドの実行を検知し、成否に基づいて
品質パイプラインの状態を更新する。
"""

import json
import os
import re
import sys
from pathlib import Path

STATE_DIR = Path("/tmp/ai-driven-quality-state")
STATE_FILE = STATE_DIR / "quality-state.json"

# コマンド検出パターン
BUILD_PATTERNS = [
    r"make\s+build", r"go\s+build", r"npm\s+run\s+build", r"pnpm\s+build",
    r"yarn\s+build", r"cargo\s+build", r"mvn\s+compile", r"gradle\s+build",
    r"tsc\b", r"npx\s+astro\s+build",
]

LINT_PATTERNS = [
    r"make\s+lint", r"golangci-lint", r"eslint", r"prettier",
    r"npm\s+run\s+lint", r"pnpm\s+lint", r"yarn\s+lint",
    r"cargo\s+clippy", r"ruff\s+check", r"flake8", r"pylint",
    r"stylelint", r"shellcheck", r"sqlfluff",
]

TEST_PATTERNS = [
    r"make\s+test", r"go\s+test", r"npm\s+test", r"pnpm\s+test",
    r"yarn\s+test", r"pytest", r"cargo\s+test", r"jest", r"vitest",
    r"make\s+test-modules", r"make\s+test-all", r"make\s+test-footonly",
]

# 失敗検出パターン
FAILURE_PATTERNS = [
    r"FAIL", r"FAILED", r"ERROR", r"error:", r"ERRORS?:",
    r"build failed", r"compilation failed", r"exit status [1-9]",
]

# 成功検出パターン
SUCCESS_PATTERNS = [
    r"PASS", r"ok\s+\S+", r"All checks passed", r"Successfully compiled",
    r"Build complete", r"0 error", r"passed", r"✓",
]


def matches_any(text: str, patterns: list[str]) -> bool:
    """テキストがいずれかのパターンにマッチするか"""
    for pattern in patterns:
        if re.search(pattern, text, re.IGNORECASE):
            return True
    return False


def detect_command_type(command: str) -> str | None:
    """コマンドの種類を判定"""
    if matches_any(command, TEST_PATTERNS):
        return "test"
    if matches_any(command, LINT_PATTERNS):
        return "lint"
    if matches_any(command, BUILD_PATTERNS):
        return "build"
    return None


def detect_success(exit_code: int, output: str) -> bool:
    """コマンドの成否を判定"""
    if exit_code == 0:
        return True
    # exit_code != 0 でも出力にFAILがなければ成功扱いしない
    return False


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
    }


def save_state(state: dict) -> None:
    """状態ファイルを保存"""
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    STATE_FILE.write_text(json.dumps(state, indent=2, ensure_ascii=False))


def main():
    exit_code = int(os.environ.get("EXIT_CODE", "-1"))
    command = os.environ.get("COMMAND", "")
    output = os.environ.get("OUTPUT", "")

    # stdinからJSON入力を試行
    if not command:
        try:
            input_data = json.loads(sys.stdin.read())
            exit_code = input_data.get("exit_code", exit_code)
            command = input_data.get("command", "")
            output = input_data.get("output", "")
        except (json.JSONDecodeError, IOError):
            pass

    if not command:
        return

    cmd_type = detect_command_type(command)
    if cmd_type is None:
        return

    state = load_state()

    # 追跡中のファイルがなければスキップ
    if not state.get("touched_files"):
        return

    success = detect_success(exit_code, output)

    if cmd_type == "build":
        state["build_ok"] = success
    elif cmd_type == "lint":
        state["lint_ok"] = success
    elif cmd_type == "test":
        state["test_ok"] = success

    save_state(state)

    if not success:
        print(f"⚠️ {cmd_type} が失敗しました。修正後に再実行してください。")


if __name__ == "__main__":
    main()
