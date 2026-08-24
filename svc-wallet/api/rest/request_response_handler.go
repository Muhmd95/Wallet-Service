package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	// paths from project root:
	"svc-wallet/internal/wallet"
	"svc-wallet/util/logger"
)

// createWallet handler

// CreateWallet handles the creation of a new wallet.
// @Summary      Create a new wallet
// @Description  Validates the incoming payload and creates a new wallet associated with a phone number.
// @Tags         Wallet
// @Accept       json
// @Produce      json
// @Param        request  body      wallet.CreateWalletRequest  true  "Create Wallet Payload"
// @Success      201      {object}  wallet.CreateWalletResponse "Wallet created successfully"
// @Failure      400      {object}  map[string]string           "Bad Request (Invalid payload or data)"
// @Failure      409      {object}  map[string]string           "Conflict (Duplicate phone number)"
// @Failure      500      {object}  map[string]string           "Internal Server Error"
// @Router       /wallet [post]
func (c *WalletController) CreateWallet(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// extract the context and the logger
	ctx := r.Context()
	log := logger.Ctx(ctx)

	if r.Method != http.MethodPost {
		log.Warn().Msg("Method not allowed")
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var reqData wallet.CreateWalletRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
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
			//log.Warn().Str("phone_number", reqData.PhoneNumber).Msg("Duplicate wallet creation attempt")
			respondWithError(w, http.StatusConflict, err.Error())
			return
		}
		//log.Error().Err(err).Msg("Failed to create wallet")
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseData, err := json.Marshal(createResponse)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response")
		respondWithError(w, http.StatusInternalServerError, "Failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/v1/wallet/phone/"+reqData.PhoneNumber) // set the location for the new wallet
	w.WriteHeader(http.StatusCreated)
	w.Write(responseData)

	log.Info().Str("wallet_id", createResponse.WalletID).Msg("Wallet created successfully")

}

// get wallet handler

// GetWallet retrieves an existing wallet by phone number.
// @Summary      Retrieve a wallet
// @Description  Fetches the wallet details using the provided phone number query parameter.
// @Tags         Wallet
// @Accept       json
// @Produce      json
// @Param        phone_number  path     string  true  "Phone number"
// @Success      200           {object}  wallet.GetWalletResponse "Wallet retrieved successfully"
// @Failure      400           {object}  map[string]string        "Bad Request (Invalid phone number format)"
// @Failure      404           {object}  map[string]string        "Wallet Not Found"
// @Failure      500           {object}  map[string]string        "Internal Server Error"
// @Router       /wallet/phone/{phone_number} [get]
func (c *WalletController) GetWalletByPhoneNumber(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	log := logger.Ctx(ctx)

	if r.Method != http.MethodGet {
		log.Warn().Msg("Method not allowed")
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	phoneNumber := r.PathValue("phone_number")

	err := wallet.ValidatePhoneNumber(&phoneNumber)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid phone number")
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Info().Str("phone_number", phoneNumber).Msg("Retrieving wallet by phone number")

	getResponse, err := c.service.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) { // dont leak database errors
			//log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Failed to get wallet by phone number")
			respondWithError(w, http.StatusNotFound, err.Error())
			return
		}
		//log.Error().Err(err).Msg("Failed to get wallet")
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseData, err := json.Marshal(getResponse)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response")
		respondWithError(w, http.StatusInternalServerError, "Failed to marshal response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)

	log.Info().Str("phone_number", phoneNumber).Msg("Get wallet response sent successfully")

}

// GetWallet retrieves an existing wallet by wallet id.
// @Summary      Retrieve a wallet
// @Description  Fetches the wallet details using the provided phone number query parameter.
// @Tags         Wallet
// @Accept       json
// @Produce      json
// @Param        wallet_id  path     string  true  "Wallet ID"
// @Success      200           {object}  wallet.GetWalletResponse "Wallet retrieved successfully"
// @Failure      400           {object}  map[string]string        "Bad Request (Invalid wallet ID format)"
// @Failure      404           {object}  map[string]string        "Wallet Not Found"
// @Failure      500           {object}  map[string]string        "Internal Server Error"
// @Router       /wallet/{wallet_id} [get]
func (c *WalletController) GetWalletByID(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	log := logger.Ctx(ctx)

	if r.Method != http.MethodGet {
		log.Warn().Msg("Method not allowed")
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	walletID := r.PathValue("wallet_id")

	log.Info().Str("wallet_id", walletID).Msg("Retrieving wallet by wallet ID")

	getResponse, err := c.service.GetWalletByID(ctx, walletID)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) { // dont leak database errors
			//log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Failed to get wallet by phone number")
			respondWithError(w, http.StatusNotFound, err.Error())
			return
		} else if errors.Is(err, wallet.ErrInvalidWalletID) {
			log.Warn().Err(err).Msg("Invalid wallet id")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		//log.Error().Err(err).Msg("Failed to get wallet")
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseData, err := json.Marshal(getResponse)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response")
		respondWithError(w, http.StatusInternalServerError, "Failed to marshal response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)

	log.Info().Str("wallet_id", walletID).Msg("Get wallet response sent successfully")

}

// modify wallet balance handler

// ModifyWalletBalance updates the balance of an existing wallet.
// @Summary      Modify wallet balance
// @Description  Directly updates the balance of a wallet based on the requested amount.
// @Tags         Wallet
// @Accept       json
// @Produce      json
// @Param        request  body      wallet.UpdateWalletBalanceRequest  true  "Update Balance Payload"
// @Param        Idempotency-Key  header    string  true  "Unique key to prevent duplicate balance modifications"
// @Success      200      {object}  wallet.UpdateWalletBalanceResponse "Balance updated successfully"
// @Failure      400      {object}  map[string]string                  "Bad Request (Invalid payload, insufficient balance, or capacity limit)"
// @Failure      404      {object}  map[string]string                  "Wallet Not Found"
// @Failure      405      {object}  map[string]string                  "Method Not Allowed"
// @Failure      500      {object}  map[string]string                  "Internal Server Error"
// @Router       /wallet/balance [patch]
func (c *WalletController) ModifyWalletBalance(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ctx := r.Context()
	log := logger.Ctx(ctx)
	if r.Method != http.MethodPatch {
		log.Warn().Msg("Method not allowed")
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	refID := r.Header.Get("Idempotency-Key")

	if refID == "" {
		log.Warn().Msg("Missing Idempotency-Key header")
		respondWithError(w, http.StatusBadRequest, "Missing Idempotency-Key header")
		return
	}

	var reqData wallet.UpdateWalletBalanceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reqData); err != nil {
		log.Warn().Err(err).Msg("Invalid request payload")
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if reqData.Amount == 0 {
		log.Warn().Msg("Update amount must be not equal to zero")
		respondWithError(w, http.StatusBadRequest, "Update amount must be not equal to zero")
		return
	}

	err := wallet.ValidatePhoneNumber(&reqData.PhoneNumber)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid phone number")
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Info().Str("phone_number", reqData.PhoneNumber).Int64("amount", reqData.Amount).Msg("Modifying wallet balance")

	modifyResponse, err := c.service.ModifyWalletBalance(ctx, reqData.PhoneNumber, reqData.Amount, refID)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) {
			//log.Warn().Str("phone_number", reqData.PhoneNumber).Msg("Wallet not found")
			respondWithError(w, http.StatusNotFound, err.Error())
			return
		} else if errors.Is(err, wallet.ErrInsufficientBalance) || errors.Is(err, wallet.ErrExceedsMaxBalance) {
			//log.Warn().Str("phone_number", reqData.PhoneNumber).Msg("Insufficient balance")
			respondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		//log.Error().Err(err).Msg("Failed to modify wallet balance")
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	responseData, err := json.Marshal(modifyResponse)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal response")
		respondWithError(w, http.StatusInternalServerError, "Failed to marshal response")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)

	log.Info().Str("phone_number", reqData.PhoneNumber).Msg("Wallet balance modification response sent successfully")

}

// helper function to return errors and json response to the client
func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
