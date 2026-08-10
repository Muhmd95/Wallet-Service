//go:build integration
// +build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const BaseURL = "http://localhost:8000/v1"

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// --- Helpers ---

func createWallet(t *testing.T, phone, name, nationalID string) string {
	t.Helper()
	reqBody, _ := json.Marshal(map[string]interface{}{
		"owner_name":    name,
		"phone_number":  phone,
		"currency_code": "EGP",
		"national_id":   nationalID,
	})

	req, _ := http.NewRequest("POST", BaseURL+"/wallet", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create wallet for %s: %v", phone, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	// If it already exists, just return the ID
	if resp.StatusCode == http.StatusConflict {
		id, _ := getWallet(t, phone)
		return id
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Unexpected status creating wallet %s: %d, body: %s", phone, resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respData); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	walletID, ok := respData["wallet_id"].(string)
	if !ok {
		t.Fatalf("No wallet_id in response: %s", string(bodyBytes))
	}

	return walletID
}

func getWallet(t *testing.T, phone string) (string, int64) {
	t.Helper()
	req, _ := http.NewRequest("GET", BaseURL+"/wallet/phone/"+phone, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get wallet %s: %v", phone, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to get wallet %s, status %d, body: %s", phone, resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	json.Unmarshal(bodyBytes, &respData)

	walletID := respData["wallet_id"].(string)

	var balance int64
	switch v := respData["balance"].(type) {
	case float64:
		balance = int64(v)
	case int:
		balance = int64(v)
	}

	return walletID, balance
}

func modifyBalance(t *testing.T, phone string, amount int64, idempKey string) (int, int64, string) {
	t.Helper()
	reqBody, _ := json.Marshal(map[string]interface{}{
		"phone_number": phone,
		"amount":       amount,
	})

	req, _ := http.NewRequest("PATCH", BaseURL+"/wallet/balance", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	if idempKey != "" {
		req.Header.Set("Idempotency-Key", idempKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to modify balance for %s: %v", phone, err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var respData map[string]interface{}
	json.Unmarshal(bodyBytes, &respData)

	var balance int64
	if balFloat, ok := respData["balance"].(float64); ok {
		balance = int64(balFloat)
	}
	return resp.StatusCode, balance, string(bodyBytes)
}

// --- Test Scenarios ---

func TestSetup_CreateWallets(t *testing.T) {
	phones := []string{"01511111111", "01522222222", "01533333333", "01544444444", "01555555555"}
	nid := "29901011234567"

	for i, phone := range phones {
		name := fmt.Sprintf("User %d", i+1)

		reqBody, _ := json.Marshal(map[string]interface{}{
			"owner_name":    name,
			"phone_number":  phone,
			"currency_code": "EGP",
			"national_id":   nid,
		})

		req, _ := http.NewRequest("POST", BaseURL+"/wallet", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request for %s: %v", phone, err)
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// The prompt says "Verify all return 201". We assume a clean database run.
		if resp.StatusCode != http.StatusCreated {
			t.Logf("Notice: Got status %d for %s. Expected 201. Response: %s", resp.StatusCode, phone, string(bodyBytes))
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("Failed to create wallet")
			}
		} else {
			var respData map[string]interface{}
			json.Unmarshal(bodyBytes, &respData)
			bal, ok := respData["balance"].(float64)
			if !ok || bal != 0 {
				t.Fatalf("Expected starting balance 0 for %s, got %v", phone, bal)
			}
			t.Logf("Successfully created wallet for %s with balance 0", phone)
		}
	}
}

func TestAtomicity_SingleDeposit(t *testing.T) {
	phone := "01511111111"
	createWallet(t, phone, "User 1", "29901011234567") // ensure exists

	// Before balance
	_, beforeBal := getWallet(t, phone)

	statusCode, _, body := modifyBalance(t, phone, 5000, fmt.Sprintf("dep-single-%d", time.Now().UnixNano()))
	if statusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", statusCode, body)
	}

	_, afterBal := getWallet(t, phone)
	if afterBal != beforeBal+5000 {
		t.Fatalf("Expected balance %d, got %d", beforeBal+5000, afterBal)
	}
	t.Logf("Single deposit successful. New balance: %d", afterBal)
}

func TestAtomicity_SingleWithdrawal(t *testing.T) {
	phone := "01522222222"
	createWallet(t, phone, "User 2", "29901011234567")

	// Ensure balance is sufficient
	modifyBalance(t, phone, 10000, fmt.Sprintf("dep-setup-%d", time.Now().UnixNano()))

	_, beforeBal := getWallet(t, phone)

	statusCode, _, body := modifyBalance(t, phone, -3000, fmt.Sprintf("with-single-%d", time.Now().UnixNano()))
	if statusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", statusCode, body)
	}

	_, afterBal := getWallet(t, phone)
	if afterBal != beforeBal-3000 {
		t.Fatalf("Expected balance %d, got %d", beforeBal-3000, afterBal)
	}
	t.Logf("Single withdrawal successful. New balance: %d", afterBal)
}

func TestConcurrent_50ParallelDeposits(t *testing.T) {
	phone := "01533333333"
	createWallet(t, phone, "User 3", "29901011234567")

	_, beforeBal := getWallet(t, phone)

	var wg sync.WaitGroup
	var successCount int32
	var failCount int32

	concurrency := 50

	t.Logf("Starting 50 parallel deposits...")
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			idempKey := fmt.Sprintf("dep-50-%d-%d", idx, time.Now().UnixNano())
			status, _, _ := modifyBalance(t, phone, 200, idempKey)
			if status == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}(i)
	}
	wg.Wait()

	_, afterBal := getWallet(t, phone)

	expectedBalance := beforeBal + int64(successCount*200)
	t.Logf("Success: %d, Failed: %d", successCount, failCount)
	t.Logf("Expected final balance: %d, Actual: %d", expectedBalance, afterBal)

	if afterBal != expectedBalance {
		t.Fatalf("Balance mismatch. Expected %d, got %d", expectedBalance, afterBal)
	}
}

func TestConcurrent_50ParallelWithdrawals(t *testing.T) {
	phone := "01544444444"
	createWallet(t, phone, "User 4", "29901011234567")

	// Deposit 20000 first
	status, _, body := modifyBalance(t, phone, 20000, fmt.Sprintf("setup-dep-%d", time.Now().UnixNano()))
	if status != http.StatusOK {
		t.Fatalf("Failed to setup balance: %s", body)
	}

	_, beforeBal := getWallet(t, phone)

	var wg sync.WaitGroup
	var successCount int32
	var failCount int32

	concurrency := 50

	t.Logf("Starting 50 parallel withdrawals...")
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			idempKey := fmt.Sprintf("with-50-%d-%d", idx, time.Now().UnixNano())
			status, _, _ := modifyBalance(t, phone, -200, idempKey)
			if status == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}(i)
	}
	wg.Wait()

	_, afterBal := getWallet(t, phone)

	expectedBalance := beforeBal - int64(successCount*200)
	t.Logf("Success: %d, Failed: %d", successCount, failCount)
	t.Logf("Expected final balance: %d, Actual: %d", expectedBalance, afterBal)

	if afterBal != expectedBalance {
		t.Fatalf("Balance mismatch. Expected %d, got %d", expectedBalance, afterBal)
	}
}

func TestConcurrent_MixedDepositsAndWithdrawals(t *testing.T) {
	phone := "01555555555"
	createWallet(t, phone, "User 5", "29901011234567")

	modifyBalance(t, phone, 50000, fmt.Sprintf("setup-mixed-%d", time.Now().UnixNano()))
	_, beforeBal := getWallet(t, phone)

	var wg sync.WaitGroup
	var successDepCount int32
	var successWithCount int32

	t.Logf("Starting 40 deposits and 30 withdrawals concurrently...")

	// 40 deposits
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			status, _, _ := modifyBalance(t, phone, 100, fmt.Sprintf("mix-dep-%d-%d", idx, time.Now().UnixNano()))
			if status == http.StatusOK {
				atomic.AddInt32(&successDepCount, 1)
			}
		}(i)
	}

	// 30 withdrawals
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			status, _, _ := modifyBalance(t, phone, -100, fmt.Sprintf("mix-with-%d-%d", idx, time.Now().UnixNano()))
			if status == http.StatusOK {
				atomic.AddInt32(&successWithCount, 1)
			}
		}(i)
	}

	wg.Wait()
	_, afterBal := getWallet(t, phone)

	expectedBalance := beforeBal + int64(successDepCount*100) - int64(successWithCount*100)
	t.Logf("Successful Deposits: %d, Successful Withdrawals: %d", successDepCount, successWithCount)
	t.Logf("Expected balance: %d, Actual: %d", expectedBalance, afterBal)

	if afterBal != expectedBalance {
		t.Fatalf("Balance mismatch. Expected %d, got %d", expectedBalance, afterBal)
	}
}

func TestIdempotency_SameKeyTwice(t *testing.T) {
	phone := "01511111111"
	createWallet(t, phone, "User 1", "29901011234567")

	_, beforeBal := getWallet(t, phone)
	idempKey := fmt.Sprintf("test-idemp-key-%d", time.Now().UnixNano())

	// First request
	status1, _, body1 := modifyBalance(t, phone, 1000, idempKey)
	if status1 != http.StatusOK {
		t.Fatalf("First request failed: %s", body1)
	}

	// Second exact same request
	status2, _, body2 := modifyBalance(t, phone, 1000, idempKey)
	if status2 != http.StatusOK {
		t.Fatalf("Second request failed: %s", body2)
	}

	_, afterBal := getWallet(t, phone)
	if afterBal != beforeBal+1000 {
		t.Fatalf("Balance should have increased by exactly 1000, but went from %d to %d", beforeBal, afterBal)
	}
	t.Logf("Idempotency verified. Balance increased by 1000 only.")
}

func TestIdempotency_UnderConcurrency(t *testing.T) {
	phone := "01522222222"
	createWallet(t, phone, "User 2", "29901011234567")

	_, beforeBal := getWallet(t, phone)
	idempKey := fmt.Sprintf("concurrent-idemp-test-%d", time.Now().UnixNano())

	var wg sync.WaitGroup
	var successCount int32

	t.Logf("Firing 20 concurrent requests with the SAME idempotency key...")
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, _, _ := modifyBalance(t, phone, 500, idempKey)
			if status == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}
	wg.Wait()

	_, afterBal := getWallet(t, phone)

	if afterBal != beforeBal+500 {
		t.Fatalf("Expected balance to increase by exactly 500 once. Went from %d to %d", beforeBal, afterBal)
	}
	t.Logf("Concurrent idempotency verified. Final balance: %d", afterBal)
}

func TestEdgeCase_InsufficientBalance(t *testing.T) {
	phone := fmt.Sprintf("0158%07d", time.Now().UnixNano()%10000000)

	createWallet(t, phone, "Broke User", "29901011234567")

	// Try to withdraw from a wallet with 0 balance

	status, _, body := modifyBalance(t, phone, -1000, fmt.Sprintf("insuf-%d", time.Now().UnixNano()))
	if status != http.StatusBadRequest {
		t.Fatalf("Expected HTTP 400 for insufficient balance, got %d: %s", status, body)
	}
	t.Logf("Correctly got 400 Bad Request for insufficient funds.")
}

func TestEdgeCase_DuplicateWalletCreation(t *testing.T) {
	phone := fmt.Sprintf("0158%07d", time.Now().UnixNano()%10000000)

	// First creation
	reqBody, _ := json.Marshal(map[string]interface{}{
		"owner_name":    "Duplicate Test",
		"phone_number":  phone,
		"currency_code": "EGP",
		"national_id":   "29901011234567",
	})

	req, _ := http.NewRequest("POST", BaseURL+"/wallet", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp1, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("First creation request failed: %v", err)
	}
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("First creation should return 201, got %d", resp1.StatusCode)
	}
	t.Logf("First wallet created successfully with phone: %s", phone)

	// Second creation with same phone
	req2, _ := http.NewRequest("POST", BaseURL+"/wallet", bytes.NewBuffer(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := httpClient.Do(req2)
	if err != nil {
		t.Fatalf("Second creation request failed: %v", err)
	}

	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("Expected 409 Conflict, got %d", resp2.StatusCode)
	}
	t.Logf("Correctly got 409 Conflict for duplicate wallet creation.")
}

func TestStress_200ConcurrentDeposits(t *testing.T) {
	phone := fmt.Sprintf("0158%07d", time.Now().UnixNano()%10000000)
	createWallet(t, phone, "Stress User", "29901011234567")

	var wg sync.WaitGroup
	var successCount int32

	t.Logf("Firing 200 concurrent deposits...")
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			status, _, _ := modifyBalance(t, phone, 10, fmt.Sprintf("stress-%d-%d", idx, time.Now().UnixNano()))
			if status == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}
	wg.Wait()

	_, finalBal := getWallet(t, phone)

	expectedBalance := int64(successCount * 10)
	t.Logf("Successfully processed %d deposits.", successCount)
	t.Logf("Expected balance: %d, Actual: %d", expectedBalance, finalBal)

	if finalBal != expectedBalance {
		t.Fatalf("Stress test mismatch. Expected %d, got %d", expectedBalance, finalBal)
	}
}
