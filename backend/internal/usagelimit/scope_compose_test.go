// scope_compose のテスト。テーブル駆動 + description（`.agents/rules/testing.md`）。
//
// セキュリティ: ip_hash_hex / pid_hex の実値はテストでもダミー固定値を使い、
// 出力 hash 完全値もテスト assert 経由でしか参照しない。
package usagelimit

import (
	"strings"
	"testing"
)

func TestComposeIPHashAndPhotobookID(t *testing.T) {
	tests := []struct {
		name        string
		description string
		ip          string
		pid         string
	}{
		{
			name:        "正常_異なるIP同photobook",
			description: "ip 違いで結果も違う",
			ip:          strings.Repeat("a", 64),
			pid:         "019dd1bb774f73419a1afd0fbd279320",
		},
		{
			name:        "正常_同IP異なるphotobook",
			description: "pid 違いで結果も違う",
			ip:          strings.Repeat("a", 64),
			pid:         "019dd2cccccccccc11119876543210ab",
		},
		{
			name:        "正常_空文字でも一意性保たれる",
			description: "空文字を含んでも区切り文字で衝突しない",
			ip:          "",
			pid:         strings.Repeat("c", 32),
		},
	}
	seen := map[string]string{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComposeIPHashAndPhotobookID(tt.ip, tt.pid)
			if len(got) != 64 {
				t.Errorf("len = %d want 64 (sha256 hex)", len(got))
			}
			// 異なる入力で異なる出力（衝突回避テスト）
			if prev, ok := seen[got]; ok && prev != tt.name {
				t.Errorf("collision with %q", prev)
			}
			seen[got] = tt.name
		})
	}
}

func TestComposeIPHashAndPhotobookID_Deterministic(t *testing.T) {
	// 同入力で同出力（決定性）
	a := ComposeIPHashAndPhotobookID("ip-1", "pid-1")
	b := ComposeIPHashAndPhotobookID("ip-1", "pid-1")
	if a != b {
		t.Errorf("non-deterministic")
	}
}

func TestComposeIPHashAndPhotobookID_DelimiterMatters(t *testing.T) {
	// "ab" + "" と "" + "ab" / "a" + "b" と "ab" + "" などが衝突しない
	a := ComposeIPHashAndPhotobookID("a", "b")
	b := ComposeIPHashAndPhotobookID("ab", "")
	c := ComposeIPHashAndPhotobookID("", "ab")
	if a == b || a == c || b == c {
		t.Errorf("delimiter collision detected (a=%s b=%s c=%s)", a[:8], b[:8], c[:8])
	}
}
