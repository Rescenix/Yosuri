package market

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func respondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func getUserId(r *http.Request) string {
	uid := r.Header.Get("x-user-id")
	if uid == "" {
		uid = "test-user"
	}
	return uid
}

// 上架
func ListItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ItemID   string          `json:"itemId"`
		ItemData json.RawMessage `json:"itemData"`
		Price    int             `json:"price"`
		Quantity int             `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.ItemID == "" || body.Price <= 0 || body.Quantity <= 0 {
		respondError(w, http.StatusBadRequest, "缺少必要参数")
		return
	}
	sellerID := getUserId(r)

	var listingID string
	err := DB.QueryRow(
		`INSERT INTO market_listings (seller_id, item_id, item_data, price, quantity)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		sellerID, body.ItemID, body.ItemData, body.Price, body.Quantity,
	).Scan(&listingID)
	if err != nil {
		log.Printf("上架失败: %v", err)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("上架失败: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"listing": map[string]interface{}{"id": listingID},
	})

	// 清除所有搜索缓存（简单粗暴但有效）
	if RDB != nil {
		keys, _ := RDB.Keys(Ctx, "market:search:*").Result()
		if len(keys) > 0 {
			RDB.Del(Ctx, keys...)
		}
	}
}

// 购买
func BuyItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ListingID string `json:"listingId"`
		Quantity  int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.Quantity <= 0 {
		body.Quantity = 1
	}
	buyerID := getUserId(r)

	tx, err := DB.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "事务开启失败")
		return
	}
	defer tx.Rollback()

	var listing Listing
	err = tx.QueryRow(
		`SELECT id, seller_id, item_id, item_data, price, quantity, status
		 FROM market_listings WHERE id = $1 FOR UPDATE`, body.ListingID,
	).Scan(&listing.ID, &listing.SellerID, &listing.ItemID, &listing.ItemData, &listing.Price, &listing.Quantity, &listing.Status)
	if err != nil {
		respondError(w, http.StatusNotFound, "物品不存在")
		return
	}
	if listing.Status != "active" || listing.Quantity < body.Quantity {
		respondError(w, http.StatusBadRequest, "库存不足或已下架")
		return
	}
	// if listing.SellerID == buyerID {
	// 	respondError(w, http.StatusBadRequest, "不能购买自己的物品")
	// 	return
	// }
	totalCost := listing.Price * body.Quantity

	// var buyerGold int
	// err = tx.QueryRow(`SELECT gold FROM users WHERE id = $1 FOR UPDATE`, buyerID).Scan(&buyerGold)
	// if err != nil || buyerGold < totalCost {
	// 	respondError(w, http.StatusBadRequest, "金币不足")
	// 	return
	// }

	_, _ = tx.Exec(`UPDATE users SET gold = gold - $1 WHERE id = $2`, totalCost, buyerID)
	_, _ = tx.Exec(`UPDATE users SET gold = gold + $1 WHERE id = $2`, totalCost, listing.SellerID)

	newQty := listing.Quantity - body.Quantity
	newStatus := "active"
	if newQty == 0 {
		newStatus = "sold"
	}
	_, err = tx.Exec(`UPDATE market_listings SET quantity=$1, status=$2, updated_at=NOW() WHERE id=$3`,
		newQty, newStatus, body.ListingID)
	if err != nil {
		log.Printf("更新库存失败: %v, listingID=%s", err, body.ListingID) // 加这行
		respondError(w, http.StatusInternalServerError, "更新库存失败")
		return
	}

	_, err = tx.Exec(`INSERT INTO player_inventory (player_id, item_id, item_data, quantity)
		VALUES ($1, $2, $3, $4)`,
		buyerID, listing.ItemID, listing.ItemData, body.Quantity)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "添加物品失败")
		return
	}

	_, _ = tx.Exec(`INSERT INTO market_transactions (listing_id, buyer_id, seller_id, price, quantity)
		VALUES ($1, $2, $3, $4, $5)`,
		body.ListingID, buyerID, listing.SellerID, listing.Price, body.Quantity)

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "购买失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"paid":         totalCost,
		"receivedItem": listing.ItemID,
	})
}

// 搜索
func SearchItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	keyword := r.URL.Query().Get("keyword")
	minPrice, _ := strconv.Atoi(r.URL.Query().Get("minPrice"))
	maxPrice, _ := strconv.Atoi(r.URL.Query().Get("maxPrice"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	sortBy := r.URL.Query().Get("sort")
	sellerID := r.URL.Query().Get("sellerId")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	cacheKey := ""
	if sellerID == "" && page <= 3 {
		cacheKey = fmt.Sprintf("market:search:%s:%d:%d:%s:%d", keyword, minPrice, maxPrice, sortBy, page)
		if RDB != nil {
			val, err := RDB.Get(Ctx, cacheKey).Result()
			if err == nil {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(val))
				return
			}
		}
	}

	conditions := []string{"status = 'active'"}
	args := []interface{}{}
	argID := 1

	if keyword != "" {
		conditions = append(conditions, fmt.Sprintf("(item_id ILIKE $%d OR item_data->>'name' ILIKE $%d)", argID, argID))
		args = append(args, "%"+keyword+"%")
		argID++
	}
	if minPrice > 0 {
		conditions = append(conditions, fmt.Sprintf("price >= $%d", argID))
		args = append(args, minPrice)
		argID++
	}
	if maxPrice > 0 {
		conditions = append(conditions, fmt.Sprintf("price <= $%d", argID))
		args = append(args, maxPrice)
		argID++
	}
	if sellerID != "" {
		conditions = append(conditions, fmt.Sprintf("seller_id = $%d", argID))
		args = append(args, sellerID)
		argID++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	order := "ORDER BY price ASC"
	switch sortBy {
	case "price_desc":
		order = "ORDER BY price DESC"
	case "newest":
		order = "ORDER BY created_at DESC"
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM market_listings %s", where)
	DB.QueryRow(countQuery, args...).Scan(&total)

	dataQuery := fmt.Sprintf(
		"SELECT id, seller_id, item_id, COALESCE(item_data, '{}'::jsonb) AS item_data, price, quantity, status, created_at "+
			"FROM market_listings %s %s LIMIT $%d OFFSET $%d",
		where, order, argID, argID+1)
	args = append(args, limit, (page-1)*limit)

	rows, err := DB.Query(dataQuery, args...)
	if err != nil {
		log.Printf("搜索失败: %v", err)
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("查询失败: %v", err))
		return
	}
	defer rows.Close()

	listings := []Listing{}
	for rows.Next() {
		var l Listing
		if err := rows.Scan(&l.ID, &l.SellerID, &l.ItemID, &l.ItemData, &l.Price, &l.Quantity, &l.Status, &l.CreatedAt); err != nil {
			log.Printf("扫描行失败: %v", err)
			continue
		}
		listings = append(listings, l)
	}

	result := map[string]interface{}{
		"listings": listings,
		"page":     page,
		"limit":    limit,
		"total":    total,
	}
	resBytes, _ := json.Marshal(result)

	if cacheKey != "" && RDB != nil {
		RDB.Set(Ctx, cacheKey, resBytes, 60*time.Second)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resBytes)
}

// 下架
func CancelListing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ListingID string `json:"listingId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if body.ListingID == "" {
		respondError(w, http.StatusBadRequest, "missing listingId")
		return
	}
	userID := getUserId(r)

	tx, err := DB.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "事务失败")
		return
	}
	defer tx.Rollback()

	var listing Listing
	err = tx.QueryRow(
		`SELECT id, seller_id, item_id, item_data, quantity, status FROM market_listings WHERE id = $1 FOR UPDATE`,
		body.ListingID,
	).Scan(&listing.ID, &listing.SellerID, &listing.ItemID, &listing.ItemData, &listing.Quantity, &listing.Status)
	if err != nil || listing.Status != "active" || listing.SellerID != userID {
		respondError(w, http.StatusForbidden, "无权操作或物品已售出")
		return
	}

	_, err = tx.Exec(`UPDATE market_listings SET status='cancelled', updated_at=NOW() WHERE id=$1`, body.ListingID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "更新失败")
		return
	}

	_, err = tx.Exec(`INSERT INTO player_inventory (player_id, item_id, item_data, quantity)
		VALUES ($1, $2, $3, $4)`,
		userID, listing.ItemID, listing.ItemData, listing.Quantity)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "归还物品失败")
		return
	}

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "提交事务失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}
