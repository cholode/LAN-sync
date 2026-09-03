package infrastructure

import (
	"context"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"lan-im-go/shared/observability/metrics"
	"lan-im-go/pkg"
)

var (
	MongoClient       *mongo.Client
	MessageCollection *mongo.Collection
)

// InitMongo 连接 MongoDB 并准备 messages 集合。
// 仅当 MESSAGE_STORE=mongo 时调用此函数。
func InitMongo() {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	database := os.Getenv("MONGO_DB")
	if database == "" {
		database = "lan_im"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetMonitor(metrics.NewMongoCommandMonitor("mongo")))
	if err != nil {
		pkg.Fatalf("[MongoDB] 连接配置失败: %v", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		pkg.Fatalf("[MongoDB] Ping 失败: %v", err)
	}

	MongoClient = client
	MessageCollection = client.Database(database).Collection("messages")

	if err := ensureMessageIndexes(ctx); err != nil {
		pkg.Fatalf("[MongoDB] 消息索引创建失败: %v", err)
	}

	pkg.Infof("[MongoDB] 连接成功, database=%s collection=messages", database)
}

func ensureMessageIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "client_msg_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "room_id", Value: 1},
				{Key: "created_at", Value: -1},
				{Key: "_id", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "room_id", Value: 1},
				{Key: "sender_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "room_id", Value: 1},
				{Key: "created_at", Value: 1},
			},
		},
	}

	_, err := MessageCollection.Indexes().CreateMany(ctx, indexes)
	return err
}

// CloseMongo 释放 MongoDB 客户端。
func CloseMongo() {
	if MongoClient != nil {
		_ = MongoClient.Disconnect(context.Background())
	}
}
