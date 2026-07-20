package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	// paths from project root:
	"svc-wallet/internal/wallet"
	"svc-wallet/util/tracer"
)

// createWallet handler
func (c *WalletController) CreateWallet(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	requestID := tracer.GetRequestID(ctx)

	var reqData wallet.CreateWalletRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		slog.Warn("Failed to decode request body", "request_id", requestID, "error", err.Error())
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	err := reqData.Validate()
	if err != nil {
		slog.Warn("Invalid request data", "request_id", requestID, "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Info("Creating wallet", "request_id", requestID, "phone_number", reqData.PhoneNumber, "owner_name", reqData.OwnerName)

	response, err := c.service.CreateWallet(ctx, &reqData)
	if err != nil {
		if errors.Is(err, wallet.ErrDuplicatePhone) { // dont leak database errors
			slog.Warn("Duplicate wallet creation attempt", "request_id", requestID, "phone_number", reqData.PhoneNumber)
			http.Error(w, "Wallet with this phone number already exists", http.StatusConflict)
			return
		}

		slog.Error("Failed to create wallet", "request_id", requestID, "error", err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(response); err != nil {
		slog.Error("Failed to encode response", "request_id", requestID, "error", err.Error())
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	slog.Info("Wallet created successfully", "request_id", requestID, "wallet_id", response.WalletID)

}

// get wallet handler
func (c *WalletController) GetWallet(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	requestID := tracer.GetRequestID(ctx)

	phoneNumber := r.URL.Query().Get("phone_number")
	// + in the url is converted to space so i will replace it with + again
	phoneNumber = strings.ReplaceAll(phoneNumber, " ", "+")

	err := wallet.ValidatePhoneNumber(phoneNumber)
	if err != nil {
		slog.Warn("Invalid phone number", "request_id", requestID, "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	walletResponse, err := c.service.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) { // dont leak database errors
			slog.Warn("Wallet not found", "request_id", requestID, "phone_number", phoneNumber)
			http.Error(w, "Wallet not found", http.StatusNotFound)
			return
		}
		slog.Error("Failed to get wallet", "request_id", requestID, "error", err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(walletResponse); err != nil {
		slog.Error("Failed to encode response", "request_id", requestID, "error", err.Error())
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	slog.Info("Wallet response sent successfully", "request_id", requestID, "phone_number", phoneNumber)

}

// modify wallet balance handler
func (c *WalletController) ModifyWalletBalance(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	requestID := tracer.GetRequestID(ctx)

	var reqData wallet.UpdateWalletBalanceRequest

	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		slog.Warn("Invalid request payload", "request_id", requestID, "error", err.Error())
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	err := wallet.ValidatePhoneNumber(reqData.PhoneNumber)
	if err != nil {
		slog.Warn("Invalid phone number", "request_id", requestID, "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resWallet, err := c.service.ModifyWalletBalance(ctx, reqData.PhoneNumber, reqData.Amount)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) {
			slog.Warn("Wallet not found", "request_id", requestID, "phone_number", reqData.PhoneNumber)
			http.Error(w, "Wallet not found", http.StatusNotFound)
			return
		} else if errors.Is(err, wallet.ErrInsufficientBalance) {
			slog.Warn("Insufficient balance", "request_id", requestID, "phone_number", reqData.PhoneNumber)
			http.Error(w, "Insufficient balance", http.StatusBadRequest)
			return
		} else if errors.Is(err, wallet.ErrExceedsMaxBalance) {
			slog.Warn("Deposit exceeds maximum wallet capacity", "request_id", requestID, "phone_number", reqData.PhoneNumber)
			http.Error(w, "Deposit exceeds maximum wallet capacity", http.StatusBadRequest)
			return
		}
		slog.Error("Failed to modify wallet balance", "request_id", requestID, "error", err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(resWallet); err != nil {
		slog.Error("Failed to encode response", "request_id", requestID, "error", err.Error())
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	slog.Info("Wallet balance modification response sent successfully", "request_id", requestID, "phone_number", reqData.PhoneNumber)

}
