package application

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
	"lan-im-go/repository"
)

var (
	ErrRoomNotFound      = errors.New("群聊不存在")
	ErrNotRoomMember     = errors.New("您不是该群成员")
	ErrTargetNotMember   = errors.New("用户不在群聊中")
	ErrPermissionDenied  = errors.New("权限不足")
	ErrCannotRemoveOwner = errors.New("不能移除群主")
	ErrInvalidRoomName   = errors.New("群聊名称不能为空")
)

// RuntimeNotifier 只描述房间变更对实时连接层产生的影响。
// 本地运行时可由 Hub 实现，独立部署时由事件发布器实现。
type RuntimeNotifier interface {
	JoinRoom(userID, roomID int64)
	LeaveRoom(userID, roomID int64)
	DisbandRoom(roomID int64)
}

type Service struct {
	rooms   repository.RoomRepository
	members repository.RoomMemberRepository
	runtime RuntimeNotifier
}

func NewService(rooms repository.RoomRepository, members repository.RoomMemberRepository, runtime RuntimeNotifier) *Service {
	return &Service{rooms: rooms, members: members, runtime: runtime}
}

func (s *Service) CreateRoom(name string, creatorID int64) (*models.Room, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidRoomName
	}
	room := &models.Room{Name: name, CreatorID: creatorID, Type: 2, LastActiveAt: time.Now().UTC()}
	if err := s.rooms.CreateRoomWithCreator(room, creatorID); err != nil {
		return nil, err
	}
	if s.runtime != nil {
		s.runtime.JoinRoom(creatorID, room.ID)
	}
	return room, nil
}

func (s *Service) JoinRoom(roomID, userID int64) error {
	if _, err := s.rooms.GetRoomByID(roomID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoomNotFound
		}
		return err
	}
	if err := s.members.AddMember(roomID, userID, 1); err != nil {
		return err
	}
	if s.runtime != nil {
		s.runtime.JoinRoom(userID, roomID)
	}
	return nil
}

func (s *Service) RemoveMember(roomID, operatorID, targetUserID int64, globalRole int8) error {
	room, err := s.rooms.GetRoomByID(roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoomNotFound
		}
		return err
	}

	canRemove := globalRole == 1 || operatorID == targetUserID || room.CreatorID == operatorID
	if !canRemove {
		role, ok, roleErr := s.members.GetMemberRole(roomID, operatorID)
		if roleErr != nil {
			return roleErr
		}
		canRemove = ok && role >= 2
	}
	if !canRemove {
		return ErrPermissionDenied
	}
	if targetUserID == room.CreatorID && operatorID != targetUserID && globalRole != 1 {
		return ErrCannotRemoveOwner
	}
	if err := s.members.RemoveMember(roomID, targetUserID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTargetNotMember
		}
		return err
	}
	if s.runtime != nil {
		s.runtime.LeaveRoom(targetUserID, roomID)
	}
	return nil
}

func (s *Service) RoomMembers(roomID, requesterID int64) ([]repository.RoomMemberWithRole, int64, error) {
	ok, err := s.members.CheckIsMember(roomID, requesterID)
	if err != nil {
		return nil, 0, err
	}
	if !ok {
		return nil, 0, ErrNotRoomMember
	}
	room, err := s.rooms.GetRoomByID(roomID)
	if err != nil {
		return nil, 0, err
	}
	members, err := s.members.GetRoomMembersWithRoles(roomID)
	return members, room.CreatorID, err
}

func (s *Service) JoinedRooms(userID int64) ([]repository.JoinedRoom, error) {
	return s.rooms.GetJoinedRoomsWithRole(userID)
}

func (s *Service) SearchRooms(keyword string, offset, limit int) ([]*models.Room, int64, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.rooms.SearchRooms(keyword, offset, limit)
}

func (s *Service) DisbandRoom(roomID, operatorID int64) error {
	room, err := s.rooms.GetRoomByID(roomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoomNotFound
		}
		return err
	}
	role, ok, err := s.members.GetMemberRole(roomID, operatorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotRoomMember
	}
	if room.CreatorID != operatorID && role != 3 {
		return ErrPermissionDenied
	}
	if err := s.rooms.SoftDeleteRoom(roomID); err != nil {
		return err
	}
	if s.runtime != nil {
		s.runtime.DisbandRoom(roomID)
	}
	return nil
}
