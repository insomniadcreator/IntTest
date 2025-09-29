package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUserServiceClient_GetUserByID_Success(t *testing.T) {
	// Создаем мок-сервер для пользовательского сервиса
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "name": "Alice Johnson", "email": "alice@example.com"}`))
	}))
	defer mockServer.Close()

	client := &UserServiceClient{
		BaseURL: mockServer.URL,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	ctx := context.Background()
	user, err := client.GetUserByID(ctx, 1)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if user.ID != 1 || user.Name != "Alice Johnson" {
		t.Errorf("Expected user Alice Johnson, got: %+v", user)
	}
}

func TestUserServiceClient_GetUserByID_UserNotFound(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	client := &UserServiceClient{
		BaseURL: mockServer.URL,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	ctx := context.Background()
	_, err := client.GetUserByID(ctx, 999)

	if err == nil {
		t.Fatal("Expected error for non-existent user, got nil")
	}

	if err.Error() != "user not found" {
		t.Errorf("Expected 'user not found' error, got: %v", err)
	}
}

func TestUserServiceClient_GetUserByID_ServiceUnavailable(t *testing.T) {
	// Создаем сервер, который не отвечает
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Не отправляем ответ, имитируя зависание
		time.Sleep(10 * time.Second)
	}))
	defer mockServer.Close()

	client := &UserServiceClient{
		BaseURL: mockServer.URL,
		Client: &http.Client{
			Timeout: 1 * time.Second, // Короткий таймаут для теста
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := client.GetUserByID(ctx, 1)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected error when service is unavailable, got nil")
	}

	if duration > 3*time.Second {
		t.Errorf("Request took too long: %v", duration)
	}

	if err.Error() == "user not found" {
		t.Error("Should not return 'user not found' for timeout")
	}
}

func TestUserServiceClient_GetUserByID_NetworkError(t *testing.T) {
	// Используем несуществующий URL для имитации сетевой ошибки
	client := &UserServiceClient{
		BaseURL: "http://nonexistent-service:9999",
		Client: &http.Client{
			Timeout: 1 * time.Second,
		},
	}

	ctx := context.Background()
	_, err := client.GetUserByID(ctx, 1)

	if err == nil {
		t.Fatal("Expected network error, got nil")
	}

	if err.Error() == "user not found" {
		t.Error("Should not return 'user not found' for network error")
	}
}
