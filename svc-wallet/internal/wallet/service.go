package wallet

import (
	"context"
	"time"
)

type Service struct {
	repo Repository // this is the repository layer that will be used to interact with the database
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateWallet(ctx context.Context, req *CreateWalletRequest) (*CreateWalletResponse, error) {
	// validateing the request payload before proceeding to create the wallet
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	wallet := &Wallet{
		PhoneNumber:  req.PhoneNumber,
		OwnerName:    req.OwnerName,
		CurrencyCode: req.CurrencyCode,
		Balance:      0, // initial balance is 0
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.repo.CreateWallet(ctx, wallet)
	if err != nil {
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
	err := validatePhoneNumber(phoneNumber)
	if err != nil {
		return nil, err
	}

	wallet, err := s.repo.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
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
func (s *Service) ModifyWalletBalance(ctx context.Context, phoneNumber string, amount int64) (*GetWalletResponse, error) {
	err := validatePhoneNumber(phoneNumber)
	if err != nil {
		return nil, err
	}
	wallet, err := s.repo.GetWalletByPhoneNumber(ctx, phoneNumber)
	if err != nil {
		return nil, err
	}
	if wallet.Balance+amount < 0 {
		return nil, ErrInsufficientBalance // return the domain error for insufficient balance
	}
	result, err := s.repo.UpdateWalletBalance(ctx, phoneNumber, amount)
	if err != nil {
		return nil, err
	}

	return &GetWalletResponse{
		WalletID:     result.ID.Hex(),
		PhoneNumber:  result.PhoneNumber,
		OwnerName:    result.OwnerName,
		CurrencyCode: result.CurrencyCode,
		Balance:      result.Balance,
		FamilyID:     nil,
	}, nil
}
