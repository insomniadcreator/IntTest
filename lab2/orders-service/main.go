package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Order struct {
	ID       int    `json:"id"`
	UserID   int    `json:"user_id"`
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`
	Status   string `json:"status"`
	User     *User  `json:"user,omitempty"`
}

type UserServiceClient struct {
	BaseURL string
	Client  *http.Client
}

func (c *UserServiceClient) GetUserByID(ctx context.Context, userID int) (*User, error) {
	url := fmt.Sprintf("%s/users/%d", c.BaseURL, userID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user service returned status: %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

var (
	orders = map[int]Order{
		1: {ID: 1, UserID: 1, Product: "Laptop", Quantity: 1, Status: "pending"},
		2: {ID: 2, UserID: 2, Product: "Mouse", Quantity: 2, Status: "shipped"},
	}
	mutex      = sync.RWMutex{}
	nextID     = 3
	userClient = &UserServiceClient{
		BaseURL: "http://localhost:8082",
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
)

func getOrders(w http.ResponseWriter, r *http.Request) {
	mutex.RLock()
	defer mutex.RUnlock()

	// Создаем копию заказов с информацией о пользователях
	ordersWithUsers := make([]Order, 0, len(orders))
	for _, order := range orders {
		ordersWithUsers = append(ordersWithUsers, order)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ordersWithUsers)
}

func getOrderByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/orders/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	mutex.RLock()
	order, exists := orders[id]
	mutex.RUnlock()

	if !exists {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	// Получаем данные пользователя
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	user, err := userClient.GetUserByID(ctx, order.UserID)
	if err != nil {
		log.Printf("Warning: failed to get user %d: %v", order.UserID, err)
		// Продолжаем работу даже если не удалось получить пользователя
	}

	// Создаем ответ с пользовательскими данными
	responseOrder := order
	if user != nil {
		responseOrder.User = user
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseOrder)
}

func createOrder(w http.ResponseWriter, r *http.Request) {
	var newOrder Order
	if err := json.NewDecoder(r.Body).Decode(&newOrder); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Проверяем существование пользователя
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	_, err := userClient.GetUserByID(ctx, newOrder.UserID)
	if err != nil {
		http.Error(w, fmt.Sprintf("User not found or service unavailable: %v", err), http.StatusBadRequest)
		return
	}

	mutex.Lock()
	newOrder.ID = nextID
	orders[nextID] = newOrder
	nextID++
	mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newOrder)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	http.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getOrders(w, r)
		case http.MethodPost:
			createOrder(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/orders/", getOrderByID)
	http.HandleFunc("/health", healthCheck)

	log.Println("Orders service started on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
