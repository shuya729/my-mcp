package service

type Identity struct {
	UserID string `json:"user_id"`
}

type IdentityService struct{}

// UserID を返す
func (IdentityService) Current(userID string) Identity {
	return Identity{UserID: userID}
}
