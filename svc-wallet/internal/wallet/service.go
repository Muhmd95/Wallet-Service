package wallet

import (
	"context"
	"svc-wallet/util/logger"
	"time"
	"errors"
)

// any validation of thre request should be done in the handler and not in the service layer

type Service struct {
	repo Repository // this is the repository layer that will be used to interact with the database
	// txManager TxManager  
	// this will be a structure for managing the multi step transactions that will contain a client of mongo
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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
	year+=req.NationalID[1:3]
	month := req.NationalID[3:5]
	day := req.NationalID[5:7]

	birthDateStr := year+"-"+month+"-"+day

	birthDate, err := time.Parse("2006-01-02", birthDateStr)
	if err != nil {
		log.Error().Err(err).Msg("Couldn't parse the birth date (from service layer)")
		return nil, err
	}


	wallet := &Wallet{
		PhoneNumber:  req.PhoneNumber,
		OwnerName:    req.OwnerName,
		CurrencyCode: req.CurrencyCode,
		Balance:      0, // initial balance is 0
		NationalID: req.NationalID,
		BirthDate: birthDate,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
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
		NationalID: wallet.NationalID,
		BirthDate: wallet.BirthDate,
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
		NationalID: wallet.NationalID,
		BirthDate: wallet.BirthDate,
		FamilyID:     nil, // this
		//  will be implemented in the future when the family feature is added
	}, nil

}

// need to be refacoted in phase 2
func (s *Service) ModifyWalletBalance(ctx context.Context, phoneNumber string, amount int64, refID string) (*UpdateWalletBalanceResponse, error) {

	log := logger.Ctx(ctx)

	_, err := s.repo.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Wallet not found (from service layer)")
		}
		return nil, err
	}

	// balance musnt exceed the wallet max or doesnt be below 0
	result, err := s.repo.UpdateWalletBalance(ctx, phoneNumber, amount, refID)
	if err != nil {
		if errors.Is(err, ErrExceedsMaxBalance) {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Wallet exceeds maximum balance (from service layer)")
		} else if errors.Is(err, ErrInsufficientBalance) {
			log.Warn().Err(err).Str("phone_number", phoneNumber).Msg("Insufficient balance (from service layer)")
		}
		return nil, err
	}

	return &UpdateWalletBalanceResponse{
		WalletID:  result.ID.Hex(),
		Balance:   result.Balance,
		UpdatedAt: result.UpdatedAt,
	}, nil
}
