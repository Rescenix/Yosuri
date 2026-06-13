package market

import (
	"encoding/json"
	"time"
)

type Listing struct {
	ID        string          `json:"id"`
	SellerID  string          `json:"seller_id"`
	ItemID    string          `json:"item_id"`
	ItemData  json.RawMessage `json:"item_data"`
	Price     int             `json:"price"`
	Quantity  int             `json:"quantity"`
	Status    string          `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
}
