package domain_test

import (
	"errors"
	"testing"
	"time"

	"vrcpb/backend/internal/image/domain"
	imagetests "vrcpb/backend/internal/image/domain/tests"
	"vrcpb/backend/internal/image/domain/vo/byte_size"
	"vrcpb/backend/internal/image/domain/vo/failure_reason"
	"vrcpb/backend/internal/image/domain/vo/image_dimensions"
	"vrcpb/backend/internal/image/domain/vo/image_format"
	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/image/domain/vo/image_status"
	"vrcpb/backend/internal/image/domain/vo/image_usage_kind"
	"vrcpb/backend/internal/image/domain/vo/normalized_format"
	"vrcpb/backend/internal/image/domain/vo/variant_kind"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

func newID(t *testing.T) image_id.ImageID {
	t.Helper()
	id, err := image_id.New()
	if err != nil {
		t.Fatalf("image_id.New: %v", err)
	}
	return id
}

func newPhotobookID(t *testing.T) photobook_id.PhotobookID {
	t.Helper()
	id, err := photobook_id.New()
	if err != nil {
		t.Fatalf("photobook_id.New: %v", err)
	}
	return id
}

func TestNewUploadingImage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		description string
		params      func(t *testing.T) domain.NewUploadingImageParams
		wantErr     bool
	}{
		{
			name:        "正常_最小必須項目で作成",
			description: "Given: 最小必須項目, When: NewUploadingImage, Then: status=uploading / uploaded_at=Now",
			params: func(t *testing.T) domain.NewUploadingImageParams {
				return domain.NewUploadingImageParams{
					ID:               newID(t),
					OwnerPhotobookID: newPhotobookID(t),
					UsageKind:        image_usage_kind.Photo(),
					SourceFormat:     image_format.Jpg(),
					Now:              now,
				}
			},
		},
		{
			name:        "正常_cover usage_kindで作成",
			description: "Given: usage_kind=cover, When: 作成, Then: 成功",
			params: func(t *testing.T) domain.NewUploadingImageParams {
				return domain.NewUploadingImageParams{
					ID:               newID(t),
					OwnerPhotobookID: newPhotobookID(t),
					UsageKind:        image_usage_kind.Cover(),
					SourceFormat:     image_format.Heic(),
					Now:              now,
				}
			},
		},
		{
			name:        "異常_now=zeroで失敗",
			description: "Given: Now がゼロ値, When: 作成, Then: ErrInvalidStateForRestore",
			params: func(t *testing.T) domain.NewUploadingImageParams {
				return domain.NewUploadingImageParams{
					ID:               newID(t),
					OwnerPhotobookID: newPhotobookID(t),
					UsageKind:        image_usage_kind.Photo(),
					SourceFormat:     image_format.Jpg(),
				}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.params(t)
			img, err := domain.NewUploadingImage(p)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !img.IsUploading() {
				t.Errorf("status must be uploading, got %s", img.Status().String())
			}
			if !img.UploadedAt().Equal(p.Now) {
				t.Errorf("uploaded_at mismatch")
			}
			if img.OwnerPhotobookID().UUID() != p.OwnerPhotobookID.UUID() {
				t.Errorf("owner_photobook_id mismatch")
			}
			if img.NormalizedFormat() != nil {
				t.Errorf("normalized_format should be nil before processing")
			}
			if img.AvailableAt() != nil || img.FailedAt() != nil || img.DeletedAt() != nil {
				t.Errorf("timestamps should be nil")
			}
		})
	}
}

func TestImageStateTransitions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)

	dims, err := image_dimensions.New(1920, 1080)
	if err != nil {
		t.Fatalf("dims: %v", err)
	}
	bs, err := byte_size.New(2_000_000)
	if err != nil {
		t.Fatalf("bs: %v", err)
	}
	availParams := domain.MarkAvailableParams{
		NormalizedFormat:   normalized_format.Webp(),
		OriginalDimensions: dims,
		OriginalByteSize:   bs,
		MetadataStrippedAt: now.Add(2 * time.Second),
		Now:                now.Add(3 * time.Second),
	}

	tests := []struct {
		name        string
		description string
		run         func(t *testing.T) error
	}{
		{
			name:        "正常_uploading→processing→available",
			description: "Given: uploading, When: MarkProcessing → MarkAvailable, Then: 全フィールド設定",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().WithNow(now).Build(t)
				p, err := img.MarkProcessing(now.Add(time.Second))
				if err != nil {
					return err
				}
				if !p.IsProcessing() {
					t.Fatalf("status must be processing")
				}
				a, err := p.MarkAvailable(availParams)
				if err != nil {
					return err
				}
				if !a.IsAvailable() {
					t.Fatalf("status must be available")
				}
				if a.NormalizedFormat() == nil || a.OriginalDimensions() == nil ||
					a.OriginalByteSize() == nil || a.MetadataStrippedAt() == nil {
					t.Fatalf("required fields must be set")
				}
				if a.AvailableAt() == nil {
					t.Fatalf("available_at must be set")
				}
				if !a.CanAttachToPhotobook() {
					t.Fatalf("available image must be attachable")
				}
				return nil
			},
		},
		{
			name:        "正常_uploading→failed",
			description: "Given: uploading, When: MarkFailed, Then: failure_reason / failed_at が設定",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().WithNow(now).Build(t)
				f, err := img.MarkFailed(failure_reason.FileTooLarge(), now.Add(time.Second))
				if err != nil {
					return err
				}
				if !f.IsFailed() {
					t.Fatalf("status must be failed")
				}
				if f.FailureReason() == nil || f.FailureReason().String() != "file_too_large" {
					t.Fatalf("failure_reason mismatch")
				}
				if f.FailedAt() == nil {
					t.Fatalf("failed_at must be set")
				}
				if f.CanAttachToPhotobook() {
					t.Fatalf("failed image must NOT be attachable")
				}
				return nil
			},
		},
		{
			name:        "正常_available→deleted",
			description: "Given: available, When: MarkDeleted, Then: deleted_at が設定",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().WithNow(now).Build(t)
				p, _ := img.MarkProcessing(now.Add(time.Second))
				a, _ := p.MarkAvailable(availParams)
				d, err := a.MarkDeleted(now.Add(10 * time.Second))
				if err != nil {
					return err
				}
				if !d.IsDeleted() {
					t.Fatalf("status must be deleted")
				}
				if d.DeletedAt() == nil {
					t.Fatalf("deleted_at must be set")
				}
				return nil
			},
		},
		{
			name:        "異常_available→processingは不可",
			description: "Given: available, When: MarkProcessing, Then: ErrIllegalStatusTransition",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().WithNow(now).Build(t)
				p, _ := img.MarkProcessing(now.Add(time.Second))
				a, _ := p.MarkAvailable(availParams)
				_, err := a.MarkProcessing(now.Add(20 * time.Second))
				if !errors.Is(err, domain.ErrIllegalStatusTransition) {
					t.Fatalf("err = %v, want ErrIllegalStatusTransition", err)
				}
				return nil
			},
		},
		{
			name:        "異常_failed→availableは不可",
			description: "Given: failed, When: MarkAvailable, Then: ErrIllegalStatusTransition",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().WithNow(now).Build(t)
				f, _ := img.MarkFailed(failure_reason.DecodeFailed(), now.Add(time.Second))
				_, err := f.MarkAvailable(availParams)
				if !errors.Is(err, domain.ErrIllegalStatusTransition) {
					t.Fatalf("err = %v, want ErrIllegalStatusTransition", err)
				}
				return nil
			},
		},
		{
			name:        "異常_uploading→deletedは不可",
			description: "Given: uploading, When: MarkDeleted, Then: ErrIllegalStatusTransition",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().WithNow(now).Build(t)
				_, err := img.MarkDeleted(now.Add(time.Second))
				if !errors.Is(err, domain.ErrIllegalStatusTransition) {
					t.Fatalf("err = %v, want ErrIllegalStatusTransition", err)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(t); err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	}
}

func TestImageAttachVariant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		run         func(t *testing.T) error
	}{
		{
			name:        "正常_displayをattach",
			description: "Given: 新規 image, When: AttachVariant(display), Then: variants に追加",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().Build(t)
				v := imagetests.MakeDisplayVariant(t, img)
				out, err := img.AttachVariant(v)
				if err != nil {
					return err
				}
				if len(out.Variants()) != 1 {
					t.Fatalf("variants len mismatch")
				}
				got, ok := out.VariantByKind(variant_kind.Display())
				if !ok || !got.Kind().Equal(variant_kind.Display()) {
					t.Fatalf("display variant not found")
				}
				return nil
			},
		},
		{
			name:        "異常_同kind重複はErrDuplicateVariantKind",
			description: "Given: display 済み image, When: 再度 display を attach, Then: 重複エラー",
			run: func(t *testing.T) error {
				img := imagetests.NewImageBuilder().Build(t)
				v := imagetests.MakeDisplayVariant(t, img)
				out1, _ := img.AttachVariant(v)
				v2 := imagetests.MakeDisplayVariant(t, img)
				_, err := out1.AttachVariant(v2)
				if !errors.Is(err, domain.ErrDuplicateVariantKind) {
					t.Fatalf("err = %v, want ErrDuplicateVariantKind", err)
				}
				return nil
			},
		},
		{
			name:        "異常_別imageのvariantをattachするとErrInvalidVariant",
			description: "Given: image A の variant, When: image B に attach, Then: ErrInvalidVariant",
			run: func(t *testing.T) error {
				imgA := imagetests.NewImageBuilder().Build(t)
				imgB := imagetests.NewImageBuilder().Build(t)
				v := imagetests.MakeDisplayVariant(t, imgA)
				_, err := imgB.AttachVariant(v)
				if !errors.Is(err, domain.ErrInvalidVariant) {
					t.Fatalf("err = %v, want ErrInvalidVariant", err)
				}
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(t); err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	}
}

func TestRestoreImage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	dims, _ := image_dimensions.New(800, 600)
	bs, _ := byte_size.New(50_000)
	nf := normalized_format.Webp()
	stripped := now.Add(2 * time.Second)
	availAt := now.Add(3 * time.Second)
	failedAt := now.Add(4 * time.Second)
	deletedAt := now.Add(5 * time.Second)
	reason := failure_reason.DecodeFailed()

	tests := []struct {
		name        string
		description string
		params      RestoreCase
		wantErr     error
	}{
		{
			name:        "正常_uploadingをrestore",
			description: "Given: uploading row, When: Restore, Then: 成功",
			params: RestoreCase{
				Status: image_status.Uploading(),
			},
		},
		{
			name:        "正常_availableをrestore",
			description: "Given: available row, When: Restore, Then: 必須フィールドがそろっている",
			params: RestoreCase{
				Status:             image_status.Available(),
				NormalizedFormat:   &nf,
				OriginalDimensions: &dims,
				OriginalByteSize:   &bs,
				MetadataStrippedAt: &stripped,
				AvailableAt:        &availAt,
			},
		},
		{
			name:        "異常_availableなのにnormalized_formatなし",
			description: "Given: status=available + normalized_format nil, When: Restore, Then: ErrAvailableMissingFields",
			params: RestoreCase{
				Status:             image_status.Available(),
				OriginalDimensions: &dims,
				OriginalByteSize:   &bs,
				MetadataStrippedAt: &stripped,
				AvailableAt:        &availAt,
			},
			wantErr: domain.ErrAvailableMissingFields,
		},
		{
			name:        "異常_failedなのにfailure_reasonなし",
			description: "Given: status=failed + failure_reason nil, When: Restore, Then: ErrFailedMissingReason",
			params: RestoreCase{
				Status:   image_status.Failed(),
				FailedAt: &failedAt,
			},
			wantErr: domain.ErrFailedMissingReason,
		},
		{
			name:        "異常_deletedなのにdeleted_atなし",
			description: "Given: status=deleted + deleted_at nil, When: Restore, Then: ErrDeletedMissingDeletedAt",
			params: RestoreCase{
				Status:             image_status.Deleted(),
				NormalizedFormat:   &nf,
				OriginalDimensions: &dims,
				OriginalByteSize:   &bs,
				MetadataStrippedAt: &stripped,
				AvailableAt:        &availAt,
			},
			wantErr: domain.ErrDeletedMissingDeletedAt,
		},
		{
			name:        "正常_deletedをrestore",
			description: "Given: deleted row, When: Restore, Then: deleted_at がある",
			params: RestoreCase{
				Status:             image_status.Deleted(),
				NormalizedFormat:   &nf,
				OriginalDimensions: &dims,
				OriginalByteSize:   &bs,
				MetadataStrippedAt: &stripped,
				AvailableAt:        &availAt,
				DeletedAt:          &deletedAt,
			},
		},
		{
			name:        "正常_failedをrestore",
			description: "Given: failed row + failure_reason, When: Restore, Then: 成功",
			params: RestoreCase{
				Status:        image_status.Failed(),
				FailedAt:      &failedAt,
				FailureReason: &reason,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := newID(t)
			pid := newPhotobookID(t)
			p := domain.RestoreImageParams{
				ID:                 id,
				OwnerPhotobookID:   pid,
				UsageKind:          image_usage_kind.Photo(),
				SourceFormat:       image_format.Jpg(),
				NormalizedFormat:   tt.params.NormalizedFormat,
				OriginalDimensions: tt.params.OriginalDimensions,
				OriginalByteSize:   tt.params.OriginalByteSize,
				MetadataStrippedAt: tt.params.MetadataStrippedAt,
				Status:             tt.params.Status,
				UploadedAt:         now,
				AvailableAt:        tt.params.AvailableAt,
				FailedAt:           tt.params.FailedAt,
				FailureReason:      tt.params.FailureReason,
				DeletedAt:          tt.params.DeletedAt,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			_, err := domain.RestoreImage(p)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RestoreImage: %v", err)
			}
		})
	}
}

// RestoreCase は TestRestoreImage 用のテーブルケース。
type RestoreCase struct {
	Status             image_status.ImageStatus
	NormalizedFormat   *normalized_format.NormalizedFormat
	OriginalDimensions *image_dimensions.ImageDimensions
	OriginalByteSize   *byte_size.ByteSize
	MetadataStrippedAt *time.Time
	AvailableAt        *time.Time
	FailedAt           *time.Time
	FailureReason      *failure_reason.FailureReason
	DeletedAt          *time.Time
}
