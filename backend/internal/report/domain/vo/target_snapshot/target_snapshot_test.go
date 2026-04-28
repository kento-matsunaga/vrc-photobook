package target_snapshot_test

import (
	"errors"
	"strings"
	"testing"

	"vrcpb/backend/internal/report/domain/vo/target_snapshot"
)

func ptr(s string) *string { return &s }

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		description string
		slug        string
		title       string
		creator     *string
		wantErrIs   error
	}{
		{name: "正常_全項目埋まる", description: "slug + title + creator あり", slug: "uqfwfti7glarva5saj", title: "Test", creator: ptr("Tester")},
		{name: "正常_creator_nil", description: "匿名 photobook", slug: "uqfwfti7glarva5saj", title: "Test", creator: nil},
		{name: "正常_creator_空文字", description: "空文字は nil 扱い", slug: "uqfwfti7glarva5saj", title: "Test", creator: ptr("")},
		{name: "正常_境界_slug100", description: "100 文字 slug 境界", slug: strings.Repeat("a", 100), title: "Test"},
		{name: "正常_境界_title200", description: "200 文字 title 境界", slug: "ok12pp34zz56gh78", title: strings.Repeat("a", 200)},
		{name: "正常_境界_creator100", description: "100 文字 creator 境界", slug: "ok12pp34zz56gh78", title: "Test", creator: ptr(strings.Repeat("a", 100))},
		{name: "異常_slug_空", description: "slug 必須", slug: "", title: "Test", wantErrIs: target_snapshot.ErrInvalidPublicURLSlug},
		{name: "異常_slug_101", description: "slug 上限超過", slug: strings.Repeat("a", 101), title: "Test", wantErrIs: target_snapshot.ErrInvalidPublicURLSlug},
		{name: "異常_title_空", description: "title 必須", slug: "ok12pp34zz56gh78", title: "", wantErrIs: target_snapshot.ErrInvalidTitle},
		{name: "異常_title_201", description: "title 上限超過", slug: "ok12pp34zz56gh78", title: strings.Repeat("a", 201), wantErrIs: target_snapshot.ErrInvalidTitle},
		{name: "異常_creator_101", description: "creator 上限超過", slug: "ok12pp34zz56gh78", title: "Test", creator: ptr(strings.Repeat("a", 101)), wantErrIs: target_snapshot.ErrInvalidCreatorDisplayName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := target_snapshot.New(tt.slug, tt.title, tt.creator)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Errorf("expected %v, got %v", tt.wantErrIs, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got.PublicURLSlug() != tt.slug {
				t.Errorf("slug mismatch")
			}
			if got.Title() != tt.title {
				t.Errorf("title mismatch")
			}
			// creator が nil または空文字なら CreatorDisplayName() は nil
			if tt.creator == nil || *tt.creator == "" {
				if got.CreatorDisplayName() != nil {
					t.Errorf("CreatorDisplayName should be nil")
				}
			} else {
				if got.CreatorDisplayName() == nil || *got.CreatorDisplayName() != *tt.creator {
					t.Errorf("CreatorDisplayName mismatch")
				}
			}
		})
	}
}
