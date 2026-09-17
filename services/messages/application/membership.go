package application

// MembershipReader 只提供消息和文件授权所需的成员检查。
// 装配层决定使用数据库适配器还是远程服务客户端。
type MembershipReader interface {
	CheckIsMember(roomID, userID int64) (bool, error)
}
