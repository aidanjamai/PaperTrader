package collections

import (
	"context"
	"errors"
	"log"
	"time"

	"papertrader/internal/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type IntraDaily struct {
	Request  IntraDailyRequest  `bson:",inline"`
	Response IntraDailyResponse `bson:",inline"`
}

type IntraDailyRequest struct {
	Symbol    string `json:"symbol" bson:"symbol"`
	StartDate string `json:"start_date" bson:"start_date"`
	EndDate   string `json:"end_date" bson:"end_date"`
}

type IntraDailyResponse struct {
	Symbol           string  `json:"symbol" bson:"response_symbol"`
	Date             string  `json:"date"`
	PreviousPrice    float64 `json:"previous_price"`
	Price            float64 `json:"price"`
	Volume           int     `json:"volume"`
	Change           float64 `json:"change"`
	ChangePercentage float64 `json:"change_percentage"`
}

var ErrIntraDailyNotFound = errors.New("intra daily not found")
var ErrIntraDailyAlreadyExists = errors.New("intra daily already exists")

// IntraDailyMongoStore handles MongoDB operations for intra daily
type IntraDailyMongoStore struct {
	collection *mongo.Collection
}

func NewIntraDailyMongoStore(client *mongo.Client, mongoDBConfig *config.MongoDBConfig) *IntraDailyMongoStore {
	db := client.Database(mongoDBConfig.Database)
	col := db.Collection(mongoDBConfig.IntraDailyCollection) // Use dedicated collection for intra-daily data
	return &IntraDailyMongoStore{collection: col}
}

func (id *IntraDailyMongoStore) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create compound index on symbol, start_date, and end_date for efficient queries
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "symbol", Value: 1},
			{Key: "start_date", Value: 1},
			{Key: "end_date", Value: 1},
		},
	}
	_, err := id.collection.Indexes().CreateOne(ctx, indexModel)
	return err
}

func (id *IntraDailyMongoStore) CreateIntraDaily(intraDaily *IntraDailyRequest, response *IntraDailyResponse) (*IntraDailyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Println("Creating intra daily", intraDaily)

	//check if intraDaily already exists
	existingIntraDaily, err := id.GetIntraDailyByRequest(intraDaily)
	if err != nil && !errors.Is(err, ErrIntraDailyNotFound) {
		return nil, err
	}
	if existingIntraDaily != nil {
		return nil, ErrIntraDailyAlreadyExists
	}

	//create intra daily
	_, err = id.collection.InsertOne(ctx, IntraDaily{Request: *intraDaily, Response: *response})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (id *IntraDailyMongoStore) GetIntraDailyByRequest(intraDaily *IntraDailyRequest) (*IntraDailyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Println("Getting intra daily by request", intraDaily)

	//check if intraDaily already exists
	filter := bson.M{
		"symbol":     intraDaily.Symbol,
		"start_date": intraDaily.StartDate,
		"end_date":   intraDaily.EndDate,
	}

	var foundIntraDaily IntraDaily
	err := id.collection.FindOne(ctx, filter).Decode(&foundIntraDaily)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrIntraDailyNotFound
		}
		return nil, err
	}
	return &foundIntraDaily.Response, nil
}
