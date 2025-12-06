package collections

import (
	"context"
	"errors"
	"log"
	"papertrader/internal/config"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UserStock represents a user's stock holding
type UserStock struct {
	ID                string    `json:"id" bson:"_id,omitempty"`
	UserID            string    `json:"user_id" bson:"user_id"`
	Symbol            string    `json:"symbol" bson:"symbol"`
	Quantity          int       `json:"quantity" bson:"quantity"`
	AvgPrice          float64   `json:"avg_price" bson:"avg_price"`
	Total             float64   `json:"total" bson:"total"`
	CurrentStockPrice float64   `json:"current_stock_price" bson:"current_stock_price"`
	CreatedAt         time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" bson:"updated_at"`
}

var ErrStockHoldingNotFound = errors.New("stock holding not found")

// UserStockMongoStore handles MongoDB operations for user stocks
type UserStockMongoStore struct {
	collection *mongo.Collection
}

// NewUserStockMongoStore creates a new MongoDB store for user stocks
func NewUserStockMongoStore(client *mongo.Client, mongoDBConfig *config.MongoDBConfig) *UserStockMongoStore {
	db := client.Database(mongoDBConfig.Database)
	col := db.Collection(mongoDBConfig.UserStockCollection)
	return &UserStockMongoStore{collection: col}
}

// Init creates indexes for better performance
func (us *UserStockMongoStore) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create compound index on user_id and symbol for efficient queries
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "symbol", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}

	_, err := us.collection.Indexes().CreateOne(ctx, indexModel)
	return err
}

// create UserStock in mongodb collection
func (us *UserStockMongoStore) CreateUserStock(userStock *UserStock) (*UserStock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Println("Creating user stock", userStock)

	//check if userStock already exists
	existingUserStock, err := us.GetUserStockBySymbol(userStock.UserID, userStock.Symbol)
	if err != nil && !errors.Is(err, ErrStockHoldingNotFound) {
		return nil, err
	}
	if existingUserStock != nil {
		return nil, errors.New("userStock already exists")
	}

	userStock.ID = generateUserStockID()
	userStock.CreatedAt = time.Now()
	userStock.UpdatedAt = time.Now()

	_, err = us.collection.InsertOne(ctx, userStock)
	return userStock, err
}

// update UserStock with buy in mongodb collection
func (us *UserStockMongoStore) UpdateUserStockWithBuy(userStock *UserStock) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//check if userStock already exists
	existingUserStock, err := us.GetUserStockBySymbol(userStock.UserID, userStock.Symbol)

	if err != nil && !errors.Is(err, ErrStockHoldingNotFound) {
		return err
	}
	log.Println("Existing user stock", existingUserStock)
	err = nil
	if existingUserStock == nil {
		existingUserStock, err = us.CreateUserStock(userStock)
		if err != nil {
			return err
		}
	}

	//update existing userStock with buy
	originalQuantity := existingUserStock.Quantity
	existingUserStock.Quantity += userStock.Quantity
	existingUserStock.AvgPrice = (existingUserStock.AvgPrice*float64(originalQuantity) + userStock.AvgPrice*float64(userStock.Quantity)) / float64(existingUserStock.Quantity)
	existingUserStock.CurrentStockPrice = userStock.CurrentStockPrice
	existingUserStock.Total = existingUserStock.AvgPrice * float64(existingUserStock.Quantity)
	existingUserStock.UpdatedAt = time.Now()

	//update userStock with buy
	_, err = us.collection.UpdateOne(ctx, bson.M{"user_id": userStock.UserID, "symbol": userStock.Symbol}, bson.M{"$set": existingUserStock})
	if err != nil {
		return err
	}
	return nil
}

// update UserStock with sell in mongodb collection
func (us *UserStockMongoStore) UpdateUserStockWithSell(userStock *UserStock) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	//log.Println("Updating userStock with sell", userStock)

	//check if userStock already exists
	existingUserStock, err := us.GetUserStockBySymbol(userStock.UserID, userStock.Symbol)
	if err != nil {
		log.Println("Error getting userStock", err)
		return err
	}
	if existingUserStock == nil {
		return errors.New("userStock not found")
	}

	//check if userStock quantity is greater than existing userStock quantity
	if userStock.Quantity > existingUserStock.Quantity {
		return errors.New("userStock quantity is greater than existing userStock quantity")
	}
	//check if userStock quantity is less than 0
	if userStock.Quantity < 0 {
		return errors.New("user does not have enough stock to sell")
	}

	//update existing userStock with sell
	log.Println("Updating userStock with sell existing quantity:", existingUserStock.Quantity, "quantity requested:", userStock.Quantity)
	existingUserStock.Quantity -= userStock.Quantity
	log.Println("Updated userStock quantity:", existingUserStock.Quantity)
	existingUserStock.CurrentStockPrice = userStock.CurrentStockPrice
	existingUserStock.Total = existingUserStock.AvgPrice * float64(existingUserStock.Quantity)
	existingUserStock.UpdatedAt = time.Now()

	_, err = us.collection.UpdateOne(ctx, bson.M{"user_id": userStock.UserID, "symbol": userStock.Symbol}, bson.M{"$set": existingUserStock})
	if err != nil {
		return err
	}
	return nil
}

// GetUserStocksByUserID gets all stock holdings for a user
func (us *UserStockMongoStore) GetUserStocksByUserID(userID string) ([]UserStock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	cursor, err := us.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var stocks []UserStock
	if err = cursor.All(ctx, &stocks); err != nil {
		return nil, err
	}

	return stocks, nil
}

// GetUserStockBySymbol gets a specific stock holding for a user
func (us *UserStockMongoStore) GetUserStockBySymbol(userID, symbol string) (*UserStock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"user_id": userID,
		"symbol":  symbol,
	}

	var stock UserStock

	err := us.collection.FindOne(ctx, filter).Decode(&stock)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrStockHoldingNotFound
		}
		return nil, err
	}

	return &stock, nil
}

// DeleteUserStock deletes a stock holding
func (us *UserStockMongoStore) DeleteUserStock(userID, symbol string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{
		"user_id": userID,
		"symbol":  symbol,
	}

	_, err := us.collection.DeleteOne(ctx, filter)
	return err
}

// DeleteAllUserStocks deletes all stock holdings for a user
func (us *UserStockMongoStore) DeleteAllUserStocks(userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"user_id": userID}
	_, err := us.collection.DeleteMany(ctx, filter)
	return err
}

// Helper function to generate unique IDs
func generateUserStockID() string {
	return uuid.New().String()
}
