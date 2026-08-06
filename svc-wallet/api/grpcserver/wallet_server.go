package grpcserver

import (
	"context"

	walletv1 "github.com/Muhmd95/Contracts/wallet/v1"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"errors"

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
		WalletId: modifyResponse.WalletID,
		Balance:  modifyResponse.Balance,
		UpdatedAt: timestamppb.New(modifyResponse.UpdatedAt),
	}, nil

}