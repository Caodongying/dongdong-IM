package model

import (
	"time"
	"gorm.io/gorm"
)

type User struct {
	ID        string `gorm:"column:id;primaryKey;type:varchar(64)" json:"id"`
	Username  string `gorm:"column:username;type:varchar(128)" json:"username"`
	Email     string `gorm:"column:email;unique;not null;type:varchar(128)" json:"email"`
	Password  string `gorm:"column:password;type:varchar(128);not null" json:"password"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at" json:"deleted_at"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (u *User) TableName() string {
	return "im_user"
}