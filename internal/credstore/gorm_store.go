package credstore

import "gorm.io/gorm"

// UserCredential 是存在 DB 的學籍憑證資料表。
type UserCredential struct {
	UserID   string `gorm:"primaryKey"`
	Username string
	Password string
}

// GormStore 是 Store 介面的 DB 實作。
type GormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

func (s *GormStore) Set(userID string, creds Credentials) {
	s.db.Save(&UserCredential{
		UserID:   userID,
		Username: creds.Username,
		Password: creds.Password,
	})
}

func (s *GormStore) Get(userID string) (Credentials, bool) {
	var uc UserCredential
	if err := s.db.First(&uc, "user_id = ?", userID).Error; err != nil {
		return Credentials{}, false
	}
	return Credentials{Username: uc.Username, Password: uc.Password}, true
}

func (s *GormStore) Delete(userID string) {
	s.db.Delete(&UserCredential{}, "user_id = ?", userID)
}
