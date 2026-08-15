package admin

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	"lan-im-go/models"
)

// MessageStatsStore ?????????Dashboard ??????? MySQL ?? MongoDB?
type MessageStatsStore interface {
	CountMessages(ctx context.Context, start, end time.Time) (int64, error)
	CountMessagesByType(ctx context.Context, start, end time.Time) (map[int8]int64, error)
	CountPrivateGroupMessages(ctx context.Context, start, end time.Time) (private, group int64, err error)
	CountAgentMentions(ctx context.Context, start, end time.Time) (int64, error)
	ActiveSenders(ctx context.Context, since time.Time) (int64, error)
}

type mysqlMessageStats struct {
	db *gorm.DB
}

func newMySQLMessageStats(db *gorm.DB) MessageStatsStore {
	return &mysqlMessageStats{db: db}
}

func (s *mysqlMessageStats) notDeleted(db *gorm.DB) *gorm.DB {
	return db.Model(&models.Message{}).Where("deleted_at = 0")
}

func (s *mysqlMessageStats) CountMessages(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := s.notDeleted(s.db.WithContext(ctx)).
		Where("created_at >= ? AND created_at < ?", start, end).
		Count(&count).Error
	return count, err
}

func (s *mysqlMessageStats) CountMessagesByType(ctx context.Context, start, end time.Time) (map[int8]int64, error) {
	type row struct {
		Type  int8
		Count int64
	}
	var rows []row
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("type, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("type").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int8]int64, len(rows))
	for _, r := range rows {
		out[r.Type] = r.Count
	}
	return out, nil
}

func (s *mysqlMessageStats) CountPrivateGroupMessages(ctx context.Context, start, end time.Time) (int64, int64, error) {
	type row struct {
		RoomType int8
		Count    int64
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Model(&models.Message{}).
		Select("rooms.type AS room_type, COUNT(*) AS count").
		Joins("INNER JOIN rooms ON rooms.id = messages.room_id AND rooms.deleted_at = 0").
		Where("messages.deleted_at = 0 AND messages.created_at >= ? AND messages.created_at < ?", start, end).
		Group("rooms.type").
		Scan(&rows).Error
	if err != nil {
		return 0, 0, err
	}
	var private, group int64
	for _, r := range rows {
		if r.RoomType == 1 {
			private += r.Count
		} else {
			group += r.Count
		}
	}
	return private, group, nil
}

func (s *mysqlMessageStats) CountAgentMentions(ctx context.Context, start, end time.Time) (int64, error) {
	var count int64
	err := s.notDeleted(s.db.WithContext(ctx)).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("content LIKE ? OR content LIKE ? OR content LIKE ?", "%@agent%", "%@AI??%", "%@AI??_?%").
		Count(&count).Error
	return count, err
}

func (s *mysqlMessageStats) ActiveSenders(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	err := s.notDeleted(s.db.WithContext(ctx)).
		Where("created_at >= ?", since).
		Select("COUNT(DISTINCT sender_id)").
		Scan(&count).Error
	return count, err
}

type mongoMessageStats struct {
	coll *mongo.Collection
	db   *gorm.DB
}

func newMongoMessageStats(coll *mongo.Collection, db *gorm.DB) MessageStatsStore {
	return &mongoMessageStats{coll: coll, db: db}
}

func notDeletedFilter() bson.M {
	return bson.M{
		"$or": bson.A{
			bson.M{"deleted_at": bson.M{"$exists": false}},
			bson.M{"deleted_at": 0},
		},
	}
}

func (s *mongoMessageStats) baseFilter(start, end time.Time) bson.M {
	f := notDeletedFilter()
	f["created_at"] = bson.M{"$gte": start, "$lt": end}
	return f
}

func (s *mongoMessageStats) CountMessages(ctx context.Context, start, end time.Time) (int64, error) {
	return s.coll.CountDocuments(ctx, s.baseFilter(start, end))
}

func (s *mongoMessageStats) CountMessagesByType(ctx context.Context, start, end time.Time) (map[int8]int64, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: s.baseFilter(start, end)}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$type", "count": bson.M{"$sum": 1}}}},
	}
	cursor, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Type  int8  `bson:"_id"`
		Count int64 `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[int8]int64, len(rows))
	for _, r := range rows {
		out[r.Type] = r.Count
	}
	return out, nil
}

func (s *mongoMessageStats) roomIDsByType(ctx context.Context, roomType int8) ([]int64, error) {
	var ids []int64
	err := s.db.WithContext(ctx).
		Model(&models.Room{}).
		Where("type = ?", roomType).
		Pluck("id", &ids).Error
	return ids, err
}

func (s *mongoMessageStats) CountPrivateGroupMessages(ctx context.Context, start, end time.Time) (int64, int64, error) {
	privateIDs, err := s.roomIDsByType(ctx, 1)
	if err != nil {
		return 0, 0, err
	}
	groupIDs, err := s.roomIDsByType(ctx, 2)
	if err != nil {
		return 0, 0, err
	}

	private, err := s.countByRoomIDs(ctx, start, end, privateIDs)
	if err != nil {
		return 0, 0, err
	}
	group, err := s.countByRoomIDs(ctx, start, end, groupIDs)
	if err != nil {
		return 0, 0, err
	}
	return private, group, nil
}

func (s *mongoMessageStats) countByRoomIDs(ctx context.Context, start, end time.Time, roomIDs []int64) (int64, error) {
	if len(roomIDs) == 0 {
		return 0, nil
	}
	f := s.baseFilter(start, end)
	f["room_id"] = bson.M{"$in": roomIDs}
	return s.coll.CountDocuments(ctx, f)
}

func (s *mongoMessageStats) CountAgentMentions(ctx context.Context, start, end time.Time) (int64, error) {
	f := s.baseFilter(start, end)
	f["$or"] = bson.A{
		bson.M{"content": bson.M{"$regex": "@agent", "$options": "i"}},
		bson.M{"content": bson.M{"$regex": "@AI??", "$options": "i"}},
	}
	return s.coll.CountDocuments(ctx, f)
}

func (s *mongoMessageStats) ActiveSenders(ctx context.Context, since time.Time) (int64, error) {
	f := notDeletedFilter()
	f["created_at"] = bson.M{"$gte": since}
	result := s.coll.Distinct(ctx, "sender_id", f)
	var senders []int64
	if err := result.Decode(&senders); err != nil {
		return 0, err
	}
	return int64(len(senders)), nil
}
