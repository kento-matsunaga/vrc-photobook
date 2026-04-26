package http

import (
	"github.com/google/uuid"

	"vrcpb/backend/internal/image/domain/vo/image_id"
	"vrcpb/backend/internal/photobook/domain/vo/photobook_id"
)

func parsePhotobookID(s string) (photobook_id.PhotobookID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return photobook_id.PhotobookID{}, err
	}
	return photobook_id.FromUUID(u)
}

func parseImageID(s string) (image_id.ImageID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return image_id.ImageID{}, err
	}
	return image_id.FromUUID(u)
}
