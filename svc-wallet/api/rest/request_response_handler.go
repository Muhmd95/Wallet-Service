package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	// paths from project root:
	"svc-wallet/internal/wallet"
	"svc-wallet/util/logger"
)

// WalletController handles HTTP requests related to wallet operations.
func (c *WalletController) WalletHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		c.CreateWallet(w, r)
	case http.MethodGet:
		c.GetWallet(w, r)
	default:
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// createWallet handler
func (c *WalletController) CreateWallet(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// extract the context and the logger
	ctx := r.Context()
	log := logger.Ctx(ctx)

	var reqData wallet.CreateWalletRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		log.Warn().Err(err).Msg("Failed to decode request body")
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	err := reqData.Validate()
	if err != nil {
		log.Warn().Err(err).Msg("Invalid request data")
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Info().Str("phone_number", reqData.PhoneNumber).Str("owner_name", reqData.OwnerName).Msg("Creating wallet")

	createResponse, err := c.service.CreateWallet(ctx, &reqData)
	if err != nil {
		if errors.Is(err, wallet.ErrDuplicatePhone) { // dont leak database errors
			log.Warn().Str("phone_number", reqData.PhoneNumber).Msg("Duplicate wallet creation attempt")
			respondWithError(w, http.StatusConflict, "Wallet with this phone number already exists")
			return
		}

		log.Error().Err(err).Msg("Failed to create wallet")
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(createResponse); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		respondWithError(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	log.Info().Str("wallet_id", createResponse.WalletID).Msg("Wallet created successfully")

}

// get wallet handler
func (c *WalletController) GetWallet(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	log := logger.Ctx(ctx)

	phoneNumber := r.URL.Query().Get("phone_number")
	// + in the url is converted to space so i will replace it with + again
	phoneNumber = strings.ReplaceAll(phoneNumber, " ", "+")

	err := wallet.ValidatePhoneNumber(phoneNumber)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid phone number")
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	getResponse, err := c.service.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) { // dont leak database errors
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Failed to get wallet by phone number")
			respondWithError(w, http.StatusNotFound, "Wallet not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get wallet")
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(getResponse); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		respondWithError(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	log.Info().Str("phone_number", phoneNumber).Msg("Get wallet response sent successfully")

}

// modify wallet balance handler
func (c *WalletController) ModifyWalletBalance(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	log := logger.Ctx(ctx)
	if r.Method != http.MethodPatch {
		log.Warn().Msg("Method not allowed")
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var reqData wallet.UpdateWalletBalanceRequest

	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
		log.Warn().Err(err).Msg("Invalid request payload")
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	err := wallet.ValidatePhoneNumber(reqData.PhoneNumber)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid phone number")
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	modifyResponse, err := c.service.ModifyWalletBalance(ctx, reqData.PhoneNumber, reqData.Amount)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) {
			log.Warn().Str("phone_number", reqData.PhoneNumber).Msg("Wallet not found")
			respondWithError(w, http.StatusNotFound, "Wallet not found")
			return
		} else if errors.Is(err, wallet.ErrInsufficientBalance) {
			log.Warn().Str("phone_number", reqData.PhoneNumber).Msg("Insufficient balance")
			respondWithError(w, http.StatusBadRequest, "Insufficient balance")
			return
		} else if errors.Is(err, wallet.ErrExceedsMaxBalance) {
			log.Warn().Str("phone_number", reqData.PhoneNumber).Msg("Deposit exceeds maximum wallet capacity")
			respondWithError(w, http.StatusBadRequest, "Deposit exceeds maximum wallet capacity")
			return
		}
		log.Error().Err(err).Msg("Failed to modify wallet balance")
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(modifyResponse); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		respondWithError(w, http.StatusInternalServerError, "Failed to encode response")
		return
	}

	log.Info().Str("phone_number", reqData.PhoneNumber).Msg("Wallet balance modification response sent successfully")

}

// helper function to return errors and json response to the client
func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
