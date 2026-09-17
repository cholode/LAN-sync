package repository

import (
	"context"
	usersmodel "lan-im-go/services/users/models"
)

// UserRepository 用户数据访问接口
type UserRepository interface {
	// 基础用户操作
	CreateUser(user *usersmodel.User) error
	GetByUsername(username string) (*usersmodel.User, error)
	GetByUsernameContext(ctx context.Context, username string) (*usersmodel.User, error)
	GetByID(id int64) (*usersmodel.User, error)
	// 按ID软删除用户
	SoftDeleteUser(id int64) error
}
