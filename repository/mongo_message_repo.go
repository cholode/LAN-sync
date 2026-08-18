package repository

import (
	"context"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"lan-im-go/models"
)

type mongoMessageRepo struct {
	collection *mongo.Collection
}

func NewMongoMessageRepo(collection *mongo.Collection) MessageRepository {
	return &mongoMessageRepo{collection: collection}
}

func notDeletedFilter() bson.M {
	return bson.M{
		"$or": bson.A{
			bson.M{"deleted_at": bson.M{"$exists": false}},
			bson.M{"deleted_at": 0},
		},
	}
}

func (r *mongoMessageRepo) SaveMessage(msg *models.Message) error {
	doc := msg.ToMessageDocument()
	if doc == nil {
		return nil
	}

	_, err := r.collection.UpdateOne(
		context.Background(),
		bson.M{"client_msg_id": doc.ClientMsgID},
		bson.M{"$setOnInsert": doc},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (r *mongoMessageRepo) SaveMessageBatch(msgs []*models.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	models := make([]mongo.WriteModel, 0, len(msgs))
	for _, msg := range msgs {
		doc := msg.ToMessageDocument()
		if doc == nil || doc.ClientMsgID == "" {
			continue
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"client_msg_id": doc.ClientMsgID}).
			SetUpdate(bson.M{"$setOnInsert": doc}).
			SetUpsert(true))
	}

	if len(models) == 0 {
		return nil
	}

	_, err := r.collection.BulkWrite(
		context.Background(),
		models,
		options.BulkWrite().SetOrdered(false),
	)
	return err
}

func (r *mongoMessageRepo) GetHistoryByCursor(roomID int64, cursorMsgID int64, limit int) ([]*models.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := notDeletedFilter()
	filter["room_id"] = roomID
	if cursorMsgID > 0 {
		var cursorDoc models.MessageDocument
		err := r.collection.FindOne(ctx, bson.M{"_id": cursorMsgID}).Decode(&cursorDoc)
		if err == nil {
			filter = bson.M{
				"room_id": roomID,
				"$and": bson.A{
					notDeletedFilter(),
					bson.M{"$or": bson.A{
						bson.M{"created_at": bson.M{"$lt": cursorDoc.CreatedAt}},
						bson.M{
							"created_at": cursorDoc.CreatedAt,
							"_id":        bson.M{"$lt": cursorMsgID},
						},
					}},
				},
			}
		} else {
			filter = bson.M{
				"room_id": roomID,
				"$and": bson.A{
					notDeletedFilter(),
					bson.M{"_id": bson.M{"$lt": cursorMsgID}},
				},
			}
		}
	}

	opts := options.Find().
		SetSort(bson.D{
			{Key: "created_at", Value: -1},
			{Key: "_id", Value: -1},
		}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []models.MessageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	out := make([]*models.Message, 0, len(docs))
	for i := len(docs) - 1; i >= 0; i-- {
		out = append(out, docs[i].ToMessage())
	}
	return out, nil
}

func (r *mongoMessageRepo) SoftDeleteUserMessagesInRoom(roomID int64, userID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := r.collection.UpdateMany(
		ctx,
		bson.M{
			"room_id":   roomID,
			"sender_id": userID,
			"$and": bson.A{
				notDeletedFilter(),
			},
		},
		bson.M{"$set": bson.M{"deleted_at": time.Now().UnixMilli()}},
	)
	return err
}

func (r *mongoMessageRepo) GetMessagesByTimeRange(roomID int64, start, end time.Time, limit int) ([]models.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := notDeletedFilter()
	filter["room_id"] = roomID
	filter["created_at"] = bson.M{
		"$gte": start,
		"$lt":  end,
	}
	opts := options.Find().
		SetSort(bson.D{
			{Key: "created_at", Value: 1},
			{Key: "_id", Value: 1},
		}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []models.MessageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	out := make([]models.Message, 0, len(docs))
	for i := range docs {
		if msg := docs[i].ToMessage(); msg != nil {
			out = append(out, *msg)
		}
	}
	return out, nil
}

func (r *mongoMessageRepo) GetMessagesAfterID(roomID int64, sinceID int64, limit int) ([]models.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := notDeletedFilter()
	filter["room_id"] = roomID
	if sinceID > 0 {
		filter["_id"] = bson.M{"$gt": sinceID}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []models.MessageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	out := make([]models.Message, 0, len(docs))
	for i := range docs {
		if msg := docs[i].ToMessage(); msg != nil {
			out = append(out, *msg)
		}
	}
	return out, nil
}

func (r *mongoMessageRepo) CountMessagesAfterID(roomID int64, sinceID int64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := notDeletedFilter()
	filter["room_id"] = roomID
	if sinceID > 0 {
		filter["_id"] = bson.M{"$gt": sinceID}
	}
	return r.collection.CountDocuments(ctx, filter)
}

func (r *mongoMessageRepo) SearchMessages(params MessageSearchParams) ([]*models.Message, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := notDeletedFilter()
	filter["room_id"] = params.RoomID
	if params.Keyword != "" {
		filter["content"] = bson.Regex{Pattern: regexp.QuoteMeta(params.Keyword), Options: "i"}
	}
	if params.SenderID > 0 {
		filter["sender_id"] = params.SenderID
	}
	if !params.Start.IsZero() || !params.End.IsZero() {
		createdAt := bson.M{}
		if !params.Start.IsZero() {
			createdAt["$gte"] = params.Start
		}
		if !params.End.IsZero() {
			createdAt["$lt"] = params.End
		}
		filter["created_at"] = createdAt
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var docs []models.MessageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}
	messages := make([]*models.Message, 0, len(docs))
	for i := range docs {
		if msg := docs[i].ToMessage(); msg != nil {
			messages = append(messages, msg)
		}
	}
	return messages, total, nil
}

var _ MessageRepository = (*mongoMessageRepo)(nil)
