package usecase_test

import (
	"crypto/sha256"
	"testing"

	"vrcpb/backend/internal/report/internal/usecase"
)

func TestHashSourceIP(t *testing.T) {
	tests := []struct {
		name        string
		description string
		saltVersion string
		salt        string
		ip          string
	}{
		{
			name:        "正常_v1_典型値",
			description: "Given: salt + IP, When: HashSourceIP, Then: sha256 32 bytes",
			saltVersion: usecase.SaltVersionV1,
			salt:        "test-salt-32-bytes-AAAAAAAAAAAAAA",
			ip:          "192.0.2.1",
		},
		{
			name:        "正常_IPv6",
			description: "Given: IPv6, When: HashSourceIP, Then: sha256 32 bytes",
			saltVersion: usecase.SaltVersionV1,
			salt:        "salt-x",
			ip:          "2001:db8::1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usecase.HashSourceIP(tt.saltVersion, tt.salt, tt.ip)
			if len(got) != sha256.Size {
				t.Errorf("hash length=%d want %d", len(got), sha256.Size)
			}
			// 同じ入力で hash が決定論的であることを確認
			again := usecase.HashSourceIP(tt.saltVersion, tt.salt, tt.ip)
			for i := range got {
				if got[i] != again[i] {
					t.Errorf("hash should be deterministic, mismatch at byte %d", i)
					break
				}
			}
		})
	}
}

func TestHashSourceIPDifferentSaltVersion(t *testing.T) {
	salt := "common-salt"
	ip := "192.0.2.99"
	v1 := usecase.HashSourceIP("v1", salt, ip)
	v2 := usecase.HashSourceIP("v2", salt, ip)
	identical := true
	for i := range v1 {
		if v1[i] != v2[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("different salt version should produce different hash (v1 vs v2)")
	}
}

func TestHashSourceIPDifferentSalt(t *testing.T) {
	a := usecase.HashSourceIP("v1", "salt-A", "192.0.2.1")
	b := usecase.HashSourceIP("v1", "salt-B", "192.0.2.1")
	identical := true
	for i := range a {
		if a[i] != b[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("different salt should produce different hash")
	}
}
