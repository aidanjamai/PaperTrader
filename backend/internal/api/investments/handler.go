package investments

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"papertrader/internal/data"
	"papertrader/internal/data/collections"
)

type InvestmentsHandler struct {
	TradeStore     data.Trades
	StockStore     data.Stocks
	UserStore      data.Users
	UserStockStore collections.UserStocks
}

func NewInvestmentsHandler(tradeStore data.Trades, stockStore data.Stocks, userStore data.Users, userStockStore collections.UserStocks) *InvestmentsHandler {
	return &InvestmentsHandler{TradeStore: tradeStore, StockStore: stockStore, UserStore: userStore, UserStockStore: userStockStore}
}

func (Ih *InvestmentsHandler) BuyStock(w http.ResponseWriter, r *http.Request) {

	var buyStockRequest BuyStockRequest
	err := json.NewDecoder(r.Body).Decode(&buyStockRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//get stock price
	stock, err := Ih.StockStore.GetStockBySymbol(buyStockRequest.Symbol)
	if err != nil {
		log.Printf("Error getting stock price: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	user, err := Ih.UserStore.GetUserByID(buyStockRequest.UserID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userBalance := user.Balance
	price := stock.Price
	totalPrice := price * float64(buyStockRequest.Quantity)
	updatedBalance := userBalance - totalPrice

	//validate user balance
	if updatedBalance < 0 {
		log.Printf("User does not have enough funds to buy stock balance: %f, total price: %f", userBalance, totalPrice)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	date := time.Now()
	dateString := date.Format("01/02/2006")

	//update user balance
	err = Ih.UserStore.UpdateBalance(buyStockRequest.UserID, updatedBalance)
	if err != nil {
		log.Printf("Error updating user balance: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("User balance updated to: %f", updatedBalance)

	log.Printf("Creating user stock buy for %s with quantity %d at price %f on date %s", buyStockRequest.Symbol, buyStockRequest.Quantity, price, dateString)
	err = Ih.TradeStore.CreateTradeBuy(buyStockRequest.Symbol, buyStockRequest.Quantity, price, buyStockRequest.UserID, dateString)
	if err != nil {
		log.Printf("Error creating user stock buy trade in db: %v", err)
		Ih.UserStore.UpdateBalance(buyStockRequest.UserID, userBalance)
		log.Printf("Refunded user balance to: %f", userBalance)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//add investment to mongodb collection
	userStock := &collections.UserStock{
		UserID:            buyStockRequest.UserID,
		Symbol:            buyStockRequest.Symbol,
		Quantity:          buyStockRequest.Quantity,
		AvgPrice:          price,
		Total:             totalPrice,
		CurrentStockPrice: price,
	}
	log.Printf("Creating user stock buy for %s with quantity %d at price %f on date %s", buyStockRequest.Symbol, buyStockRequest.Quantity, price, dateString)
	err = Ih.UserStockStore.UpdateUserStockWithBuy(userStock)
	if err != nil {
		log.Printf("Error creating user stock in mongodb collection: %v", err)
		Ih.UserStore.UpdateBalance(buyStockRequest.UserID, userBalance)
		log.Printf("Refunded user balance to: %f", userBalance)
		//TODO: delete trade by userID and symbol
		// Ih.TradeStore.DeleteTradeByID(buyStockRequest.UserID, buyStockRequest.Symbol)
		// log.Printf("Deleted user stock buy trade in db")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("User stock buy created for %s with quantity %d at price %f on date %s", buyStockRequest.Symbol, buyStockRequest.Quantity, price, dateString)

	w.WriteHeader(http.StatusOK)
}

func (Ih *InvestmentsHandler) SellStock(w http.ResponseWriter, r *http.Request) {

	var sellStockRequest SellStockRequest
	err := json.NewDecoder(r.Body).Decode(&sellStockRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	//get user
	user, err := Ih.UserStore.GetUserByID(sellStockRequest.UserID)
	if err != nil {
		log.Printf("Error getting user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// validate user has quatity of stock to sell

	userStock, err := Ih.UserStockStore.GetUserStockBySymbol(sellStockRequest.UserID, sellStockRequest.Symbol)
	if err != nil {
		log.Printf("Error getting user stock: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if userStock.Quantity < sellStockRequest.Quantity {
		log.Printf("User does not have enough stock to sell: %d", userStock.Quantity)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//get stock price
	stock, err := Ih.StockStore.GetStockBySymbol(sellStockRequest.Symbol)
	if err != nil {
		log.Printf("Error getting stock: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	date := time.Now()
	dateString := date.Format("01/02/2006")
	price := stock.Price
	totalPrice := price * float64(sellStockRequest.Quantity)
	updatedBalance := user.Balance + totalPrice

	err = Ih.UserStore.UpdateBalance(sellStockRequest.UserID, updatedBalance)
	if err != nil {
		log.Printf("Error updating user balance: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("User balance updated to: %f", updatedBalance)

	// add investment to mongodb collection
	userStock.Quantity -= sellStockRequest.Quantity
	userStock.CurrentStockPrice = price
	userStock.Total = price * float64(sellStockRequest.Quantity)
	err = Ih.UserStockStore.UpdateUserStockWithSell(userStock)
	if err != nil {
		log.Printf("Error updating user stock in mongodb collection: %v", err)
		Ih.UserStore.UpdateBalance(sellStockRequest.UserID, user.Balance)
		log.Printf("Refunded user balance to: %f", user.Balance)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Creating user stock sell for %s with quantity %d at price %f on date %s", sellStockRequest.Symbol, sellStockRequest.Quantity, price, dateString)
	err = Ih.TradeStore.CreateTradeSell(sellStockRequest.Symbol, sellStockRequest.Quantity, price, sellStockRequest.UserID, dateString)
	if err != nil {
		log.Printf("Error creating user stock sell: %v", err)
		Ih.UserStore.UpdateBalance(sellStockRequest.UserID, user.Balance)
		log.Printf("Refunded user balance to: %f", user.Balance)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("User stock sell created for %s with quantity %d at price %f on date %s", sellStockRequest.Symbol, sellStockRequest.Quantity, price, dateString)
}

func (Ih *InvestmentsHandler) GetUserStocks(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userID")
	userStocks, err := Ih.UserStockStore.GetUserStocksByUserID(userID)
	if err != nil {
		log.Printf("Error getting user stocks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userStocks)
}
