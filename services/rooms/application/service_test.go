package application

import (
	"testing"

	roomsmodel "lan-im-go/services/rooms/models"
	"lan-im-go/services/rooms/repository"
	usersmodel "lan-im-go/services/users/models"
)

type roomRepoStub struct {
	room *roomsmodel.Room
}

func (s *roomRepoStub) CreateRoomWithCreator(room *roomsmodel.Room, _ int64) error {
	room.ID = 101
	s.room = room
	return nil
}
func (s *roomRepoStub) GetRoomByID(_ int64) (*roomsmodel.Room, error)         { return s.room, nil }
func (s *roomRepoStub) SoftDeleteRoom(_ int64) error                          { return nil }
func (s *roomRepoStub) GetJoinedRooms(_ int64) ([]*roomsmodel.Room, error)    { return nil, nil }
func (s *roomRepoStub) GetRoomByExactName(_ string) (*roomsmodel.Room, error) { return nil, nil }
func (s *roomRepoStub) GetJoinedRoomsWithRole(_ int64) ([]repository.JoinedRoom, error) {
	return nil, nil
}
func (s *roomRepoStub) SearchRooms(_ string, _, _ int) ([]*roomsmodel.Room, int64, error) {
	return nil, 0, nil
}

type memberRepoStub struct {
	role int8
	ok   bool
}

func (s *memberRepoStub) AddMember(_, _ int64, _ int8) error                 { return nil }
func (s *memberRepoStub) RemoveMember(_, _ int64) error                      { return nil }
func (s *memberRepoStub) GetUserRoomIDs(_ int64) ([]int64, error)            { return nil, nil }
func (s *memberRepoStub) CheckIsMember(_, _ int64) (bool, error)             { return s.ok, nil }
func (s *memberRepoStub) GetMemberRole(_, _ int64) (int8, bool, error)       { return s.role, s.ok, nil }
func (s *memberRepoStub) GetRoomMembers(_ int64) ([]*usersmodel.User, error) { return nil, nil }
func (s *memberRepoStub) GetRoomMembersWithRoles(_ int64) ([]repository.RoomMemberWithRole, error) {
	return nil, nil
}

type runtimeStub struct {
	joinedUser int64
	joinedRoom int64
}

func (s *runtimeStub) JoinRoom(userID, roomID int64) { s.joinedUser, s.joinedRoom = userID, roomID }
func (s *runtimeStub) LeaveRoom(_, _ int64)          {}
func (s *runtimeStub) DisbandRoom(_ int64)           {}

func TestCreateRoomPersistsAndNotifiesRuntime(t *testing.T) {
	rooms := &roomRepoStub{}
	members := &memberRepoStub{}
	runtime := &runtimeStub{}
	service := NewService(rooms, members, runtime)

	room, err := service.CreateRoom(" 测试群 ", 7)
	if err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	if room.ID != 101 || room.Name != "测试群" {
		t.Fatalf("unexpected room: %#v", room)
	}
	if runtime.joinedUser != 7 || runtime.joinedRoom != 101 {
		t.Fatalf("runtime notification = (%d, %d)", runtime.joinedUser, runtime.joinedRoom)
	}
}

func TestRemoveMemberRejectsOrdinaryMember(t *testing.T) {
	rooms := &roomRepoStub{room: &roomsmodel.Room{ID: 9, CreatorID: 1}}
	members := &memberRepoStub{role: 1, ok: true}
	service := NewService(rooms, members, nil)

	err := service.RemoveMember(9, 2, 3, 0)
	if err != ErrPermissionDenied {
		t.Fatalf("RemoveMember() error = %v, want %v", err, ErrPermissionDenied)
	}
}
