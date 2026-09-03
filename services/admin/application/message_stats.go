package admin

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	"lan-im-go/models"
)

// MessageStatsStore 为 Dashboard 提供消息统计，支持 MySQL 与 MongoDB。
type MessageStatsStore interface {
	CountMessages(ctx context.Context, start, end time.Time) (int64, error)
	CountMessagesByType(ctx context.Context, start, end time.Time) (map[int8]int64, error)
	CountPrivateGroupMessages(ctx context.Context, start, end time.Time) (private, group int64, err error)
	CountAgentMentions(ctx context.Context, start, end time.Time) (int64, error)
	ActiveSenders(ctx context.Context, since time.Time) (int64, error)
	HourlyCounts(ctx context.Context, start, end time.Time) ([]TimeCount, error)
	DailyCounts(ctx context.Context, start, end time.Time) ([]TimeCount, error)
	TopRooms(ctx context.Context, start, end time.Time, limit int) ([]KeyCount, error)
	TopSenders(ctx context.Context, start, end time.Time, limit int) ([]KeyCount, error)
	CountBySender(ctx context.Context, senderID int64) (int64, error)
	CountByRoom(ctx context.Context, roomID int64, start, end time.Time) (int64, error)
	CountByRoomTotal(ctx context.Context, roomID int64) (int64, error)
	CountsBySenderIDs(ctx context.Context, senderIDs []int64) (map[int64]int64, error)
	CountsByRoomIDs(ctx context.Context, roomIDs []int64, start, end time.Time) (map[int64]int64, error)
	CountsByRoomTotalIDs(ctx context.Context, roomIDs []int64) (map[int64]int64, error)
}

// TimeCount 表示按时间分桶后的统计数量。
type TimeCount struct {
	Time  string `json:"time"`
	Count int64  `json:"count"`
}

// KeyCount 表示某个维度及其对应统计数量。
type KeyCount struct {
	Key   int64 `json:"key"`
	Count int64 `json:"count"`
}

type keyCountRow struct {
	Key   int64
	Count int64
}

// NewMessageStatsStore 根据存储类型创建消息统计实现。
func NewMessageStatsStore(db *gorm.DB, messageCollection *mongo.Collection, messageStore string) MessageStatsStore {
	if messageStore == "mongo" && messageCollection != nil {
		return newMongoMessageStats(messageCollection, db)
	}
	return newMySQLMessageStats(db)
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
		Where("content LIKE ? OR content LIKE ? OR content LIKE ?", "%@agent%", "%@AI助手%", "%@AI助手_%").
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

func (s *mysqlMessageStats) HourlyCounts(ctx context.Context, start, end time.Time) ([]TimeCount, error) {
	type row struct {
		Time  string
		Count int64
	}
	var rows []row
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("DATE_FORMAT(created_at, '%Y-%m-%d %H:00') AS time, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("time").
		Order("time ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]TimeCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, TimeCount{Time: r.Time, Count: r.Count})
	}
	return out, nil
}

func (s *mysqlMessageStats) DailyCounts(ctx context.Context, start, end time.Time) ([]TimeCount, error) {
	type row struct {
		Time  string
		Count int64
	}
	var rows []row
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("DATE_FORMAT(created_at, '%Y-%m-%d') AS time, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("time").
		Order("time ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]TimeCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, TimeCount{Time: r.Time, Count: r.Count})
	}
	return out, nil
}

func (s *mysqlMessageStats) TopRooms(ctx context.Context, start, end time.Time, limit int) ([]KeyCount, error) {
	var rows []keyCountRow
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("room_id AS key, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("room_id").
		Order("count DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return keyCountFromRows(rows), nil
}

func (s *mysqlMessageStats) TopSenders(ctx context.Context, start, end time.Time, limit int) ([]KeyCount, error) {
	var rows []keyCountRow
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("sender_id AS key, COUNT(*) AS count").
		Where("created_at >= ? AND created_at < ?", start, end).
		Group("sender_id").
		Order("count DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return keyCountFromRows(rows), nil
}

func keyCountFromRows(rows []keyCountRow) []KeyCount {
	out := make([]KeyCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, KeyCount{Key: r.Key, Count: r.Count})
	}
	return out
}

func (s *mysqlMessageStats) CountBySender(ctx context.Context, senderID int64) (int64, error) {
	var count int64
	err := s.notDeleted(s.db.WithContext(ctx)).
		Where("sender_id = ?", senderID).
		Count(&count).Error
	return count, err
}

func (s *mysqlMessageStats) CountByRoom(ctx context.Context, roomID int64, start, end time.Time) (int64, error) {
	var count int64
	err := s.notDeleted(s.db.WithContext(ctx)).
		Where("room_id = ? AND created_at >= ? AND created_at < ?", roomID, start, end).
		Count(&count).Error
	return count, err
}

func (s *mysqlMessageStats) CountByRoomTotal(ctx context.Context, roomID int64) (int64, error) {
	var count int64
	err := s.notDeleted(s.db.WithContext(ctx)).
		Where("room_id = ?", roomID).
		Count(&count).Error
	return count, err
}

func (s *mysqlMessageStats) CountsBySenderIDs(ctx context.Context, senderIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(senderIDs))
	if len(senderIDs) == 0 {
		return out, nil
	}
	var rows []keyCountRow
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("sender_id AS key, COUNT(*) AS count").
		Where("sender_id IN ?", senderIDs).
		Group("sender_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Key] = row.Count
	}
	return out, nil
}

func (s *mysqlMessageStats) CountsByRoomIDs(ctx context.Context, roomIDs []int64, start, end time.Time) (map[int64]int64, error) {
	out := make(map[int64]int64, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}
	var rows []keyCountRow
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("room_id AS key, COUNT(*) AS count").
		Where("room_id IN ? AND created_at >= ? AND created_at < ?", roomIDs, start, end).
		Group("room_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Key] = row.Count
	}
	return out, nil
}

func (s *mysqlMessageStats) CountsByRoomTotalIDs(ctx context.Context, roomIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(roomIDs))
	if len(roomIDs) == 0 {
		return out, nil
	}
	var rows []keyCountRow
	err := s.notDeleted(s.db.WithContext(ctx)).
		Select("room_id AS key, COUNT(*) AS count").
		Where("room_id IN ?", roomIDs).
		Group("room_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Key] = row.Count
	}
	return out, nil
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
		bson.M{"content": bson.M{"$regex": "@AI助手", "$options": "i"}},
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

func (s *mongoMessageStats) CountBySender(ctx context.Context, senderID int64) (int64, error) {
	f := notDeletedFilter()
	f["sender_id"] = senderID
	return s.coll.CountDocuments(ctx, f)
}

func (s *mongoMessageStats) CountByRoom(ctx context.Context, roomID int64, start, end time.Time) (int64, error) {
	f := s.baseFilter(start, end)
	f["room_id"] = roomID
	return s.coll.CountDocuments(ctx, f)
}

func (s *mongoMessageStats) CountByRoomTotal(ctx context.Context, roomID int64) (int64, error) {
	f := notDeletedFilter()
	f["room_id"] = roomID
	return s.coll.CountDocuments(ctx, f)
}

func (s *mongoMessageStats) CountsBySenderIDs(ctx context.Context, senderIDs []int64) (map[int64]int64, error) {
	return s.groupCountByKeys(ctx, notDeletedFilter(), "sender_id", senderIDs)
}

func (s *mongoMessageStats) CountsByRoomIDs(ctx context.Context, roomIDs []int64, start, end time.Time) (map[int64]int64, error) {
	return s.groupCountByKeys(ctx, s.baseFilter(start, end), "room_id", roomIDs)
}

func (s *mongoMessageStats) CountsByRoomTotalIDs(ctx context.Context, roomIDs []int64) (map[int64]int64, error) {
	return s.groupCountByKeys(ctx, notDeletedFilter(), "room_id", roomIDs)
}

func (s *mongoMessageStats) groupCountByKeys(ctx context.Context, filter bson.M, field string, keys []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	filter[field] = bson.M{"$in": keys}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: filter}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$" + field, "count": bson.M{"$sum": 1}}}},
	}
	cursor, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Key   int64 `bson:"_id"`
		Count int64 `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Key] = row.Count
	}
	return out, nil
}

func (s *mongoMessageStats) HourlyCounts(ctx context.Context, start, end time.Time) ([]TimeCount, error) {
	return s.timeCounts(ctx, start, end, "%Y-%m-%d %H")
}

func (s *mongoMessageStats) DailyCounts(ctx context.Context, start, end time.Time) ([]TimeCount, error) {
	return s.timeCounts(ctx, start, end, "%Y-%m-%d")
}

func (s *mongoMessageStats) timeCounts(ctx context.Context, start, end time.Time, format string) ([]TimeCount, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: s.baseFilter(start, end)}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":   bson.M{"$dateToString": bson.M{"format": format, "date": "$created_at", "timezone": "Asia/Shanghai"}},
			"count": bson.M{"$sum": 1},
		}}},
		bson.D{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	cursor, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Time  string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]TimeCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, TimeCount{Time: r.Time, Count: r.Count})
	}
	return out, nil
}

func (s *mongoMessageStats) TopRooms(ctx context.Context, start, end time.Time, limit int) ([]KeyCount, error) {
	return s.topKey(ctx, start, end, "room_id", limit)
}

func (s *mongoMessageStats) TopSenders(ctx context.Context, start, end time.Time, limit int) ([]KeyCount, error) {
	return s.topKey(ctx, start, end, "sender_id", limit)
}

func (s *mongoMessageStats) topKey(ctx context.Context, start, end time.Time, field string, limit int) ([]KeyCount, error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: s.baseFilter(start, end)}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$" + field, "count": bson.M{"$sum": 1}}}},
		bson.D{{Key: "$sort", Value: bson.M{"count": -1}}},
		bson.D{{Key: "$limit", Value: int64(limit)}},
	}
	cursor, err := s.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Key   int64 `bson:"_id"`
		Count int64 `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]KeyCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, KeyCount{Key: r.Key, Count: r.Count})
	}
	return out, nil
}
