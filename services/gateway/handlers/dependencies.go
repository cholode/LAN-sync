package api

import (
	"context"
	usersmodel "lan-im-go/services/users/models"
)

// UserStore 是接入层登录、注册及连接所需的用户能力。
type UserStore interface {
	GetByUsernameContext(context.Context, string) (*usersmodel.User, error)
	GetByUsername(string) (*usersmodel.User, error)
	GetByID(int64) (*usersmodel.User, error)
	CreateUser(*usersmodel.User) error
}

type MembershipReader interface {
	GetUserRoomIDs(int64) ([]int64, error)
}

type Handler struct {
	Users      UserStore
	Membership MembershipReader
}
