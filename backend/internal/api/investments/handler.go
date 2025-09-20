package investments

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"papertrader/internal/data"
)

type InvestmentsHandler struct {
	UserStockStore data.UserStocks
	StockStore     data.Stocks
	UserStore      data.Users
}

func NewInvestmentsHandler(userStockStore data.UserStocks) *InvestmentsHandler {
	return &InvestmentsHandler{UserStockStore: userStockStore}
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
	err = Ih.UserStockStore.CreateUserStockBuy(buyStockRequest.Symbol, buyStockRequest.Quantity, price, buyStockRequest.UserID, dateString)
	if err != nil {
		log.Printf("Error creating user stock buy: %v", err)
		Ih.UserStore.UpdateBalance(buyStockRequest.UserID, userBalance)
		log.Printf("Refunded user balance to: %f", userBalance)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("User stock buy created for %s with quantity %d at price %f on date %s", buyStockRequest.Symbol, buyStockRequest.Quantity, price, dateString)

	w.WriteHeader(http.StatusOK)
}
