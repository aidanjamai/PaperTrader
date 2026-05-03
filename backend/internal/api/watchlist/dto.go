package watchlist

import "papertrader/internal/service"

type AddRequest struct {
	Symbol string `json:"symbol"`
}

type ListResponse struct {
	Items []service.WatchlistEntryView `json:"items"`
}
