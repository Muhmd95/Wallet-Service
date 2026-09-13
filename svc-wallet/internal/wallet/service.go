package wallet

import (
	"context"
	"errors"
	"strconv"
	"svc-wallet/util/logger"
	"time"

	"github.com/redis/go-redis/v9"
)

// any validation of thre request should be done in the handler and not in the service layer

type Service struct {
	repo Repository // this is the repository layer that will be used to interact with the database
	rdb *redis.Client
}
func NewService(repo Repository, rc *redis.Client) *Service {
	return &Service{repo: repo, rdb: rc}
}

func (s *Service) CreateWallet(ctx context.Context, req *CreateWalletRequest) (*CreateWalletResponse, error) {
	log := logger.Ctx(ctx)
	// the request is validated in the handler

	//  29011012345678
	var year string
	if req.NationalID[0] == '2' {
		year = "19"
	} else {
		year = "20"
	}
	year += req.NationalID[1:3]
	month := req.NationalID[3:5]
	day := req.NationalID[5:7]

	birthDateStr := year + "-" + month + "-" + day

	birthDate, err := time.Parse("2006-01-02", birthDateStr)
	if err != nil {
		log.Error().Err(err).Msg("Couldn't parse the birth date (from service layer)")
		return nil, err
	}

	wallet := &Wallet{
		PhoneNumber:   req.PhoneNumber,
		OwnerName:     req.OwnerName,
		CurrencyCode:  req.CurrencyCode,
		Balance:       0, // initial balance is 0
		NationalID:    req.NationalID,
		BirthDate:     birthDate,
		ProcessedRefs: make([]string, 0),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	err = s.repo.CreateWallet(ctx, wallet)
	if err != nil {
		if errors.Is(err, ErrDuplicatePhone) {
			log.Warn().Err(err).Str("phone_number", req.PhoneNumber).Msg("Duplicate wallet creation attempt (from service layer)")
		}
		return nil, err
	}

	response := &CreateWalletResponse{
		WalletID:  wallet.ID.Hex(), // convert ObjectID to string
		Balance:   wallet.Balance,
		CreatedAt: wallet.CreatedAt,
	}

	return response, nil

}

func (s *Service) GetWalletByPhoneNumber(ctx context.Context, phoneNumber string) (*GetWalletResponse, error) {
	log := logger.Ctx(ctx)

	wallet, err := s.repo.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Wallet not found (from service layer)")
		}
		return nil, err
	}

	return &GetWalletResponse{
		WalletID:     wallet.ID.Hex(),
		PhoneNumber:  wallet.PhoneNumber,
		OwnerName:    wallet.OwnerName,
		CurrencyCode: wallet.CurrencyCode,
		Balance:      wallet.Balance,
		NationalID:   wallet.NationalID,
		BirthDate:    wallet.BirthDate,
		FamilyID:     nil, // this
		//  will be implemented in the future when the family feature is added
	}, nil

}

func (s *Service) GetWalletByID(ctx context.Context, walletID string) (*GetWalletResponse, error) {
	log := logger.Ctx(ctx)

	wallet, err := s.repo.GetWalletByID(ctx, walletID)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			log.Warn().Err(err).Str("wallet_id", walletID).Msg("Wallet not found (from service layer)")
		}
		return nil, err
	}

	return &GetWalletResponse{
		WalletID:     wallet.ID.Hex(),
		PhoneNumber:  wallet.PhoneNumber,
		OwnerName:    wallet.OwnerName,
		CurrencyCode: wallet.CurrencyCode,
		Balance:      wallet.Balance,
		NationalID:   wallet.NationalID,
		BirthDate:    wallet.BirthDate,
		FamilyID:     nil, // this
		//  will be implemented in the future when the family feature is added
	}, nil

}

// fully idempotent consistent function
func (s *Service) ModifyWalletBalance(ctx context.Context, phoneNumber string, amount int64, refID string) (*UpdateWalletBalanceResponse, error) {

	log := logger.Ctx(ctx)

	// _, err := s.repo.GetWalletByPhoneNumber(ctx, phoneNumber)
	// if err != nil {
	// 	if errors.Is(err, ErrWalletNotFound) {
	// 		log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Wallet not found (from service layer)")
	// 	}
	// 	return nil, err
	// }

	// all logic moved to the transactions
	result, err := s.repo.UpdateWalletBalance(ctx, phoneNumber, amount, refID)
	if err != nil {
		if errors.Is(err, ErrExceedsMaxBalance) {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Wallet exceeds maximum balance (from service layer)")
		} else if errors.Is(err, ErrInsufficientBalance) {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Insufficient balance (from service layer)")
		}
		log.Error().Msg("Couldn't update the wallet balance (from service)")
		return nil, err
	}

	log.Info().Str("phone_number", phoneNumber).Int64("new_balance", result.Balance).Msg("Wallet balance updated successfully (from service layer)")

	return &UpdateWalletBalanceResponse{
		WalletID:  result.ID.Hex(),
		Balance:   result.Balance,
		UpdatedAt: result.UpdatedAt,
	}, nil
}

// interface for the consumer to use
func (s *Service) ProcessTransactionEvent(ctx context.Context, evt TransactionEvent) error {
	log := logger.Ctx(ctx)
	_, err := s.ModifyWalletBalance(ctx, evt.PhoneNumber, evt.BalanceAfter, evt.ID)
	if err != nil {
		log.Error().Err(err).Str("wallet_id", evt.WalletID).Str("event_id", evt.ID).Msg("Failed to process transaction event")
		return err
	}
	cached, err := s.rdb.HGetAll(ctx, "wallet:"+evt.WalletID).Result()
	if err != nil {
		log.Error().Err(err).Str("wallet_id", evt.WalletID).Msg("Failed to get wallet from Redis cache")
		return nil // if redis is down return
	}
	if len(cached) == 0 {
		s.rdb.HSet(ctx, "wallet:"+evt.WalletID, "balance", evt.BalanceAfter, "last_time", evt.OccurredAt)
		return nil // if the wallet is not in the cache, set it and return
	}
	last, err := strconv.ParseInt(cached["last_time"], 10, 64)
	if err != nil {
		log.Error().Err(err).Str("wallet_id", evt.WalletID).Msg("Failed to parse last_time from Redis cache")
	}

	// conpare the event time with the last cashed balance (idempotency against the processed events
	if evt.OccurredAt >= last {  // OccurredAt = created_at millis from dto.go
    	s.rdb.HSet(ctx, "wallet:"+evt.WalletID, "balance", evt.BalanceAfter, "last_time", evt.OccurredAt)
	}
	// else: skip = duplicate or old replay
	return nil
}

func (s *Service) GetWalletBalance(ctx context.Context, walletID string) (*GetWalletBalanceResponse, error) {
	log := logger.Ctx(ctx)
	m, err := s.rdb.HGetAll(ctx, "wallet:"+walletID).Result()
	if err != nil {
		log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to get wallet from Redis cache")
	}
	if len(m) != 0 { 
		balance, err := strconv.ParseInt(m["balance"], 10, 64)
		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to parse balance from Redis cache")
			return nil, err
		}
		updatedAtMillis, err := strconv.ParseInt(m["last_time"], 10, 64)
		if err != nil {
			log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to parse last_time from Redis cache")
			return nil, err
		}
		return &GetWalletBalanceResponse{
			Balance: balance,
			UpdatedAt: time.UnixMilli(updatedAtMillis),
		}, nil
	 }  // HIT
	w, err := s.repo.GetWalletByID(ctx, walletID)     // MISS
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			log.Warn().Err(err).Str("wallet_id", walletID).Msg("Wallet not found (from service layer)")
		} else if errors.Is(err, ErrInvalidWalletID) {
			log.Warn().Err(err).Str("wallet_id", walletID).Msg("Invalid wallet ID (from service layer)")
		}
		log.Error().Err(err).Str("wallet_id", walletID).Msg("Failed to get wallet from database")
		return nil, err
	}
	s.rdb.HSet(ctx, "wallet:"+walletID, "balance", w.Balance, "last_time", w.UpdatedAt.UnixMilli())
	return &GetWalletBalanceResponse{
		Balance: w.Balance,
		UpdatedAt: w.UpdatedAt,
	}, nil
}
