package config

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBConfig struct {
	URI        string
	Database   string
	Collection string
}

// func NewMongoDBConfig() *MongoDBConfig {
// 	return &MongoDBConfig{
// 		URI:        os.Getenv("MONGODB_URI"),
// 		Database:   os.Getenv("MONGODB_DATABASE"),
// 		Collection: os.Getenv("MONGODB_COLLECTION"),
// 	}
// }

func NewMongoDBConfig() *MongoDBConfig {
	return &MongoDBConfig{
		URI:        "mongodb+srv://aidansj:Dr9GFmTewaVELpbh@cluster0.4r9q7sh.mongodb.net/",
		Database:   "PaperTrader",
		Collection: "UserStock",
	}
}

func ConnectMongoDB(config *MongoDBConfig) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.URI))
	if err != nil {
		return nil, err
	}

	// Test the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	log.Println("Connected to MongoDB successfully")
	return client, nil
}
