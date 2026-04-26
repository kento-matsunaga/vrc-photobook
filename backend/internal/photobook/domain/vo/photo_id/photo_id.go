// Package photo_id は photobook_photos.id の VO（UUIDv7）。
package photo_id

import (
	"errors"

	"github.com/google/uuid"
)

var ErrInvalidPhotoID = errors.New("invalid photo id")

type PhotoID struct {
	v uuid.UUID
}

func New() (PhotoID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return PhotoID{}, err
	}
	return PhotoID{v: v}, nil
}

func FromUUID(v uuid.UUID) (PhotoID, error) {
	if v == uuid.Nil {
		return PhotoID{}, ErrInvalidPhotoID
	}
	return PhotoID{v: v}, nil
}

func MustParse(s string) PhotoID {
	return PhotoID{v: uuid.MustParse(s)}
}

func (p PhotoID) UUID() uuid.UUID        { return p.v }
func (p PhotoID) Equal(o PhotoID) bool   { return p.v == o.v }
func (p PhotoID) String() string         { return p.v.String() }
