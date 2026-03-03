package komodo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecutePullStack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/execute" {
			t.Fatalf("expected /execute, got %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "testkey" {
			t.Fatalf("expected X-Api-Key header")
		}
		if r.Header.Get("X-Api-Secret") != "testsecret" {
			t.Fatalf("expected X-Api-Secret header")
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["type"] != "PullStack" {
			t.Fatalf("expected type=PullStack, got %v", body["type"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Update{
			ID:     "upd-123",
			Status: "completed",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "testkey", "testsecret")
	upd, err := client.ExecutePullStack(context.Background(), "mystack", []string{})
	if err != nil {
		t.Fatalf("ExecutePullStack: %v", err)
	}
	if upd.ID != "upd-123" {
		t.Fatalf("expected ID=upd-123, got %s", upd.ID)
	}
}

func TestReadListStackServices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/read" {
			t.Fatalf("expected /read, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["type"] != "ListStackServices" {
			t.Fatalf("expected type=ListStackServices, got %v", body["type"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]StackService{
			{Name: "web", State: "running"},
			{Name: "db", State: "running"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "testkey", "testsecret")
	services, err := client.ReadListStackServices(context.Background(), "mystack")
	if err != nil {
		t.Fatalf("ReadListStackServices: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].Name != "web" {
		t.Fatalf("expected first service=web, got %s", services[0].Name)
	}
}

func TestErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Error{
			Error: "invalid stack name",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "testkey", "testsecret")
	_, err := client.ExecutePullStack(context.Background(), "badstack", []string{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "komodo error: invalid stack name" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetStackLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Log{
			Output: "line1\nline2\nline3",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", "secret")
	log, err := client.ReadGetStackLog(context.Background(), "mystack", []string{"web"}, 100, false)
	if err != nil {
		t.Fatalf("ReadGetStackLog: %v", err)
	}
	if log.Output != "line1\nline2\nline3" {
		t.Fatalf("expected output, got %s", log.Output)
	}
}
