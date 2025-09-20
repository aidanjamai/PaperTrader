package config

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBConfig struct {
	URI        string
	Database   string
	Collection string
}

func NewMongoDBConfig() *MongoDBConfig {
	return &MongoDBConfig{
		URI:        os.Getenv("MONGODB_URI"),
		Database:   os.Getenv("MONGODB_DATABASE"),
		Collection: os.Getenv("MONGODB_COLLECTION"),
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
