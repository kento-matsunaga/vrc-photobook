package usecase

import (
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token"
	"vrcpb/backend/internal/uploadverification/domain/vo/verification_session_token_hash"
)

// uploadVerificationTokenWithHash は raw token と hash の組（intermediate）。
type uploadVerificationTokenWithHash struct {
	tok  verification_session_token.VerificationSessionToken
	hash verification_session_token_hash.VerificationSessionTokenHash
}

// Hash は SHA-256(token) を返す。
func (t uploadVerificationTokenWithHash) Hash() verification_session_token_hash.VerificationSessionTokenHash {
	return t.hash
}

// parseUploadVerificationToken は base64url 43 文字を VO に変換し、hash を併せて返す。
func parseUploadVerificationToken(raw string) (uploadVerificationTokenWithHash, error) {
	tok, err := verification_session_token.Parse(raw)
	if err != nil {
		return uploadVerificationTokenWithHash{}, err
	}
	return uploadVerificationTokenWithHash{
		tok:  tok,
		hash: verification_session_token_hash.Of(tok),
	}, nil
}
