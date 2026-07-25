package wallet

import (
	"context"
	"math"
	"time"
	"svc-wallet/util/logger"
)

// any validation of thre request should be done in the handler and not in the service layer

type Service struct {
	repo Repository // this is the repository layer that will be used to interact with the database
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateWallet(ctx context.Context, req *CreateWalletRequest) (*CreateWalletResponse, error) {
	log := logger.Ctx(ctx)
	// the request is validated in the handler

	wallet := &Wallet{
		PhoneNumber:  req.PhoneNumber,
		OwnerName:    req.OwnerName,
		CurrencyCode: req.CurrencyCode,
		Balance:      0, // initial balance is 0
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := s.repo.CreateWallet(ctx, wallet)
	if err != nil {
		if err == ErrDuplicatePhone {
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
		if err == ErrWalletNotFound {
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
		FamilyID:     nil, // this
		//  will be implemented in the future when the family feature is added
	}, nil

}

// need to be refacoted in phase 2
func (s *Service) ModifyWalletBalance(ctx context.Context, phoneNumber string, amount int64) (*UpdateWalletBalanceResponse, error) {

	log := logger.Ctx(ctx)

	wallet, err := s.repo.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		if err == ErrWalletNotFound {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Wallet not found (from service layer)")
		}
		return nil, err
	}

	if amount > 0 {
		// If the difference between the max limit and current balance is smaller than the amount, it will overflow
		if math.MaxInt64-wallet.Balance < amount {
			err = ErrExceedsMaxBalance
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Exceeds max balance limit (from service layer)")
			return nil, ErrExceedsMaxBalance // return the domain error for exceeding max balance
		}
	}

	if wallet.Balance+amount < 0 {
		log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Insufficient balance (from service layer)")
		return nil, ErrInsufficientBalance // return the domain error for insufficient balance
	}
	result, err := s.repo.UpdateWalletBalance(ctx, phoneNumber, amount)
	if err != nil {
		if err == ErrWalletNotFound {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Wallet not found (from service layer)")
		}
		return nil, err
	}

	return &UpdateWalletBalanceResponse{
		WalletID:  result.ID.Hex(),
		Balance:   result.Balance,
		UpdatedAt: result.UpdatedAt,
	}, nil
}
