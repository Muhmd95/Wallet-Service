package grpcserver

import (
	"context"

	"errors"
	walletv1 "github.com/Muhmd95/Contracts/wallet/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	"svc-wallet/internal/wallet"
	"svc-wallet/util/logger"
)

type WalletServer struct {
	walletv1.UnimplementedWalletServiceServer // for future compatibility

	service *wallet.Service
}

func NewWalletServer(service *wallet.Service) *WalletServer {
	return &WalletServer{service: service}
}

func (s *WalletServer) ModifyBalance(ctx context.Context, req *walletv1.ModifyBalanceRequest) (*walletv1.ModifyBalanceResponse, error) {
	log := logger.Ctx(ctx)

	amount := req.GetAmount()
	phoneNumber := req.GetPhoneNumber()
	refID := req.GetReferenceID()

	if amount == 0 {
		log.Warn().Msg("Update amount must be not equal to zero (from grpc server)")
		return nil, status.Error(codes.InvalidArgument, "Update amount must be not equal to zero")
	}
	// validation of amount is done in transactions service

	err := wallet.ValidatePhoneNumber(&phoneNumber)
	if err != nil {
		log.Warn().Err(err).Msg("Invalid phone number (from grpc server)")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	log.Info().Str("phone_number", phoneNumber).Int64("amount", amount).Msg("Modifying wallet balance")

	modifyResponse, err := s.service.ModifyWalletBalance(ctx, phoneNumber, amount, refID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to modify wallet balance (from grpc server)")
		if errors.Is(err, wallet.ErrWalletNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		} else if errors.Is(err, wallet.ErrInsufficientBalance) || errors.Is(err, wallet.ErrExceedsMaxBalance) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	log.Info().Str("phone_number", phoneNumber).Msg("Wallet balance modification response sent successfully (from grpc server)")

	return &walletv1.ModifyBalanceResponse{
		WalletId:  modifyResponse.WalletID,
		Balance:   modifyResponse.Balance,
		UpdatedAt: timestamppb.New(modifyResponse.UpdatedAt),
	}, nil

}

func (s *WalletServer) GetWallet(ctx context.Context, req *walletv1.GetWalletRequest) (*walletv1.GetWalletResponse, error) {
	log := logger.Ctx(ctx)
	PhoneNumber := req.PhoneNumber

	walletRes, err := s.service.GetWalletByPhoneNumber(ctx, PhoneNumber)
	if err != nil {
		if errors.Is(err, wallet.ErrWalletNotFound) {
			log.Warn().Err(err).Msg("Wallet not found (from grpc server)")
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "Internal server error")
	}

	return &walletv1.GetWalletResponse{
		WalletId:  walletRes.WalletID,
		OwnerName: walletRes.OwnerName,
		Balance:   walletRes.Balance,
	}, nil
}

func (s *WalletServer) CreateWallet(ctx context.Context, req *walletv1.CreateWalletRequest) (*walletv1.CreateWalletResponse, error) {
	result, err := s.service.CreateUserWallet(ctx, req.GetUserId(), req.GetPhoneNumber())
	if err != nil {
		return nil, grpcError(err)
	}
	return &walletv1.CreateWalletResponse{
		WalletId:  result.WalletID,
		Balance:   result.Balance,
		CreatedAt: timestamppb.New(result.CreatedAt),
	}, nil
}

func (s *WalletServer) GetUserWallets(ctx context.Context, req *walletv1.GetUserWalletsRequest) (*walletv1.GetUserWalletsResponse, error) {
	results, err := s.service.GetUserWallets(ctx, req.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	wallets := make([]*walletv1.WalletInfo, len(results))
	for i, result := range results {
		wallets[i] = &walletv1.WalletInfo{
			WalletId:     result.ID.Hex(),
			PhoneNumber:  result.PhoneNumber,
			Balance:      result.Balance,
			CurrencyCode: result.CurrencyCode,
			CreatedAt:    timestamppb.New(result.CreatedAt),
		}
	}
	return &walletv1.GetUserWalletsResponse{Wallets: wallets}, nil
}

func (s *WalletServer) GetWalletBalance(ctx context.Context, req *walletv1.GetWalletBalanceRequest) (*walletv1.GetWalletBalanceResponse, error) {
	result, err := s.service.GetWalletBalance(ctx, req.GetWalletId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &walletv1.GetWalletBalanceResponse{
		WalletId:  req.GetWalletId(),
		Balance:   result.Balance,
		UpdatedAt: timestamppb.New(result.UpdatedAt),
	}, nil
}

func (s *WalletServer) DeleteWallet(ctx context.Context, req *walletv1.DeleteWalletRequest) (*walletv1.DeleteWalletResponse, error) {
	if err := s.service.DeleteWallet(ctx, req.GetWalletId()); err != nil {
		return nil, grpcError(err)
	}
	return &walletv1.DeleteWalletResponse{Success: true}, nil
}

func (s *WalletServer) DeleteUserWallets(ctx context.Context, req *walletv1.DeleteUserWalletsRequest) (*walletv1.DeleteUserWalletsResponse, error) {
	if err := s.service.DeleteUserWallets(ctx, req.GetUserId()); err != nil {
		return nil, grpcError(err)
	}
	return &walletv1.DeleteUserWalletsResponse{Success: true}, nil
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	case errors.Is(err, wallet.ErrInvalidUserID),
		errors.Is(err, wallet.ErrInvalidWalletID),
		errors.Is(err, wallet.ErrInvalidPhoneNumber),
		errors.Is(err, wallet.ErrInvalidNationalID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, wallet.ErrWalletNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, wallet.ErrDuplicatePhone):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, wallet.ErrInsufficientBalance), errors.Is(err, wallet.ErrExceedsMaxBalance):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
