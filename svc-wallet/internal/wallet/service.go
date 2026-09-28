package wallet

import (
	"context"
	"errors"
	"strconv"
	"svc-wallet/util/logger"
	"svc-wallet/util/metrics"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.opentelemetry.io/otel"
)

// Service methods validate transport-independent business inputs so every
// transport, including gRPC, follows the same rules.

type Service struct {
	repo Repository // this is the repository layer that will be used to interact with the database
	rdb  *redis.Client
}

func NewService(repo Repository, rc *redis.Client) *Service {
	return &Service{repo: repo, rdb: rc}
}

func (s *Service) CreateWallet(ctx context.Context, req *CreateWalletRequest) (*CreateWalletResponse, error) {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "CreateWallet")
	defer span.End()
	log := logger.Ctx(ctx)
	// the request is validated in the REST handler (currently disabled)

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
			log.Warn().Err(err).Msg("Duplicate wallet creation attempt (from service layer)")
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

func (s *Service) CreateUserWallet(ctx context.Context, userID, phoneNumber string) (*CreateWalletResponse, error) {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "CreateUserWallet")
	defer span.End()
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, ErrInvalidUserID
	}
	if err := ValidatePhoneNumber(&phoneNumber); err != nil {
		return nil, err
	}

	now := time.Now()
	w := &Wallet{
		UserID:        userObjID,
		PhoneNumber:   phoneNumber,
		Balance:       0,
		CurrencyCode:  "EGP",
		ProcessedRefs: make([]string, 0),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateWallet(ctx, w); err != nil {
		return nil, err
	}
	return &CreateWalletResponse{WalletID: w.ID.Hex(), Balance: w.Balance, CreatedAt: w.CreatedAt}, nil
}

func (s *Service) GetUserWallets(ctx context.Context, userID string) ([]Wallet, error) {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "GetUserWallets")
	defer span.End()
	if _, err := primitive.ObjectIDFromHex(userID); err != nil {
		return nil, ErrInvalidUserID
	}
	return s.repo.GetWalletsByUserID(ctx, userID)
}

func (s *Service) DeleteWallet(ctx context.Context, walletID string) error {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "DeleteWallet")
	defer span.End()
	if _, err := primitive.ObjectIDFromHex(walletID); err != nil {
		return ErrInvalidWalletID
	}
	if err := s.repo.DeleteWallet(ctx, walletID); err != nil {
		return err
	}
	if err := s.rdb.Del(ctx, "wallet:"+walletID).Err(); err != nil {
		metrics.RedisOperationsTotal.WithLabelValues("delete", "error").Inc()
		log := logger.Ctx(ctx)
		log.Warn().Err(err).Msg("Failed to invalidate deleted wallet cache")
	} else {
		metrics.RedisOperationsTotal.WithLabelValues("delete", "success").Inc()
	}
	return nil
}

func (s *Service) DeleteUserWallets(ctx context.Context, userID string) error {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "DeleteUserWallets")
	defer span.End()
	if _, err := primitive.ObjectIDFromHex(userID); err != nil {
		return ErrInvalidUserID
	}
	walletIDs, err := s.repo.DeleteUserWallets(ctx, userID)
	if err != nil {
		return err
	}
	if len(walletIDs) == 0 {
		return nil
	}
	keys := make([]string, len(walletIDs))
	for i, walletID := range walletIDs {
		keys[i] = "wallet:" + walletID
	}
	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		metrics.RedisOperationsTotal.WithLabelValues("delete", "error").Inc()
		log := logger.Ctx(ctx)
		log.Warn().Err(err).Msg("Failed to invalidate deleted wallet caches")
	} else {
		metrics.RedisOperationsTotal.WithLabelValues("delete", "success").Inc()
	}
	return nil
}

func (s *Service) GetWalletByPhoneNumber(ctx context.Context, phoneNumber string) (*GetWalletResponse, error) {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "GetWalletByPhoneNumber")
	defer span.End()
	log := logger.Ctx(ctx)

	wallet, err := s.repo.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			log.Warn().Err(err).Msg("Wallet not found (from service layer)")
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
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "GetWalletByID")
	defer span.End()
	log := logger.Ctx(ctx)

	wallet, err := s.repo.GetWalletByID(ctx, walletID)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			log.Warn().Err(err).Msg("Wallet not found (from service layer)")
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
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "ModifyWalletBalance")
	defer span.End()

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
			log.Warn().Err(err).Msg("Wallet exceeds maximum balance (from service layer)")
		} else if errors.Is(err, ErrInsufficientBalance) {
			log.Warn().Err(err).Msg("Insufficient balance (from service layer)")
		}
		log.Error().Msg("Couldn't update the wallet balance (from service)")
		return nil, err
	}

	log.Debug().Msg("Wallet balance updated successfully (from service layer)")

	return &UpdateWalletBalanceResponse{
		WalletID:  result.ID.Hex(),
		Balance:   result.Balance,
		UpdatedAt: result.UpdatedAt,
	}, nil
}

// interface for the consumer to use
func (s *Service) ProcessTransactionEvent(ctx context.Context, evt TransactionEvent) error {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "ProcessTransactionEvent")
	defer span.End()
	log := logger.Ctx(ctx)
	_, err := s.ModifyWalletBalance(ctx, evt.PhoneNumber, evt.BalanceAfter, evt.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to process transaction event")
		return err
	}
	cached, err := s.rdb.HGetAll(ctx, "wallet:"+evt.WalletID).Result()
	if err != nil {
		metrics.RedisOperationsTotal.WithLabelValues("read", "error").Inc()
		log.Error().Err(err).Msg("Failed to get wallet from Redis cache")
		return nil // if redis is down return
	}
	if len(cached) == 0 {
		metrics.RedisOperationsTotal.WithLabelValues("read", "miss").Inc()
		if err := s.rdb.HSet(ctx, "wallet:"+evt.WalletID, "balance", evt.BalanceAfter, "last_time", evt.OccurredAt).Err(); err != nil {
			metrics.RedisOperationsTotal.WithLabelValues("write", "error").Inc()
		} else {
			metrics.RedisOperationsTotal.WithLabelValues("write", "success").Inc()
		}
		return nil // if the wallet is not in the cache, set it and return
	}
	metrics.RedisOperationsTotal.WithLabelValues("read", "hit").Inc()
	last, err := strconv.ParseInt(cached["last_time"], 10, 64)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse last_time from Redis cache")
	}

	// conpare the event time with the last cashed balance (idempotency against the processed events
	if evt.OccurredAt >= last { // OccurredAt = created_at millis from dto.go
		if err := s.rdb.HSet(ctx, "wallet:"+evt.WalletID, "balance", evt.BalanceAfter, "last_time", evt.OccurredAt).Err(); err != nil {
			metrics.RedisOperationsTotal.WithLabelValues("write", "error").Inc()
		} else {
			metrics.RedisOperationsTotal.WithLabelValues("write", "success").Inc()
		}
	}
	// else: skip = duplicate or old replay
	return nil
}

func (s *Service) GetWalletBalance(ctx context.Context, walletID string) (*GetWalletBalanceResponse, error) {
	ctx, span := otel.Tracer("wallet-service").Start(ctx, "GetWalletBalance")
	defer span.End()
	log := logger.Ctx(ctx)
	m, err := s.rdb.HGetAll(ctx, "wallet:"+walletID).Result()
	if err != nil {
		metrics.RedisOperationsTotal.WithLabelValues("read", "error").Inc()
		log.Error().Err(err).Msg("Failed to get wallet from Redis cache")
	}
	if len(m) != 0 {
		metrics.RedisOperationsTotal.WithLabelValues("read", "hit").Inc()
		balance, err := strconv.ParseInt(m["balance"], 10, 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse balance from Redis cache")
			return nil, err
		}
		updatedAtMillis, err := strconv.ParseInt(m["last_time"], 10, 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse last_time from Redis cache")
			return nil, err
		}
		return &GetWalletBalanceResponse{
			Balance:   balance,
			UpdatedAt: time.UnixMilli(updatedAtMillis),
		}, nil
	} // HIT
	if err == nil {
		metrics.RedisOperationsTotal.WithLabelValues("read", "miss").Inc()
	}
	w, err := s.repo.GetWalletByID(ctx, walletID) // MISS
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			log.Warn().Err(err).Msg("Wallet not found (from service layer)")
		} else if errors.Is(err, ErrInvalidWalletID) {
			log.Warn().Err(err).Msg("Invalid wallet ID (from service layer)")
		}
		return nil, err
	}
	if err := s.rdb.HSet(ctx, "wallet:"+walletID, "balance", w.Balance, "last_time", w.UpdatedAt.UnixMilli()).Err(); err != nil {
		metrics.RedisOperationsTotal.WithLabelValues("write", "error").Inc()
	} else {
		metrics.RedisOperationsTotal.WithLabelValues("write", "success").Inc()
	}
	return &GetWalletBalanceResponse{
		Balance:   w.Balance,
		UpdatedAt: w.UpdatedAt,
	}, nil
}
