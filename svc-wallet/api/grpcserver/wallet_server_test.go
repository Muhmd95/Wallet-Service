package grpcserver

import (
	"context"
	"testing"
	"time"

	walletv1 "github.com/Muhmd95/Contracts/wallet/v1"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"svc-wallet/internal/wallet"
)

type memoryRepository struct {
	wallets map[string]*wallet.Wallet
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{wallets: make(map[string]*wallet.Wallet)}
}

func (r *memoryRepository) CreateWallet(_ context.Context, value *wallet.Wallet) error {
	for _, existing := range r.wallets {
		if existing.PhoneNumber == value.PhoneNumber {
			return wallet.ErrDuplicatePhone
		}
	}
	value.ID = primitive.NewObjectID()
	copy := *value
	r.wallets[value.ID.Hex()] = &copy
	return nil
}

func (r *memoryRepository) GetWalletByPhoneNumber(_ context.Context, phone string) (*wallet.Wallet, error) {
	for _, value := range r.wallets {
		if value.PhoneNumber == phone {
			return value, nil
		}
	}
	return nil, wallet.ErrWalletNotFound
}

func (r *memoryRepository) UpdateWalletBalance(_ context.Context, phone string, amount int64, _ string) (*wallet.Wallet, error) {
	value, err := r.GetWalletByPhoneNumber(context.Background(), phone)
	if err != nil {
		return nil, err
	}
	value.Balance = amount
	return value, nil
}

func (r *memoryRepository) GetWalletByID(_ context.Context, walletID string) (*wallet.Wallet, error) {
	value, ok := r.wallets[walletID]
	if !ok {
		return nil, wallet.ErrWalletNotFound
	}
	return value, nil
}

func (r *memoryRepository) GetWalletsByUserID(_ context.Context, userID string) ([]wallet.Wallet, error) {
	result := make([]wallet.Wallet, 0)
	for _, value := range r.wallets {
		if value.UserID.Hex() == userID {
			result = append(result, *value)
		}
	}
	return result, nil
}

func (r *memoryRepository) DeleteWallet(_ context.Context, walletID string) error {
	if _, ok := r.wallets[walletID]; !ok {
		return wallet.ErrWalletNotFound
	}
	delete(r.wallets, walletID)
	return nil
}

func (r *memoryRepository) DeleteUserWallets(_ context.Context, userID string) ([]string, error) {
	ids := make([]string, 0)
	for id, value := range r.wallets {
		if value.UserID.Hex() == userID {
			ids = append(ids, id)
			delete(r.wallets, id)
		}
	}
	return ids, nil
}

func TestUserWalletRPCs(t *testing.T) {
	repo := newMemoryRepository()
	cache := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:0",
		MaxRetries:   -1,
		DialTimeout:  5 * time.Millisecond,
		ReadTimeout:  5 * time.Millisecond,
		WriteTimeout: 5 * time.Millisecond,
	})
	t.Cleanup(func() { _ = cache.Close() })
	server := NewWalletServer(wallet.NewService(repo, cache))
	ctx := context.Background()
	userID := primitive.NewObjectID().Hex()

	created, err := server.CreateWallet(ctx, &walletv1.CreateWalletRequest{
		UserId:      userID,
		PhoneNumber: "01012345678",
	})
	if err != nil {
		t.Fatalf("CreateWallet() error = %v", err)
	}
	listed, err := server.GetUserWallets(ctx, &walletv1.GetUserWalletsRequest{UserId: userID})
	if err != nil || len(listed.Wallets) != 1 {
		t.Fatalf("GetUserWallets() = %v, %v; want one wallet", listed, err)
	}

	balance, err := server.GetWalletBalance(ctx, &walletv1.GetWalletBalanceRequest{WalletId: created.WalletId})
	if err != nil || balance.Balance != 0 {
		t.Fatalf("GetWalletBalance() = %v, %v; want zero balance", balance, err)
	}

	if _, err := server.DeleteWallet(ctx, &walletv1.DeleteWalletRequest{WalletId: created.WalletId}); err != nil {
		t.Fatalf("DeleteWallet() error = %v", err)
	}
	if _, err := server.GetWalletBalance(ctx, &walletv1.GetWalletBalanceRequest{WalletId: created.WalletId}); err == nil {
		t.Fatal("GetWalletBalance() after deletion succeeded; want not found")
	}

	if _, err := server.CreateWallet(ctx, &walletv1.CreateWalletRequest{
		UserId: userID, PhoneNumber: "01112345678",
	}); err != nil {
		t.Fatalf("second CreateWallet() error = %v", err)
	}
	if _, err := server.DeleteUserWallets(ctx, &walletv1.DeleteUserWalletsRequest{UserId: userID}); err != nil {
		t.Fatalf("DeleteUserWallets() error = %v", err)
	}
	listed, err = server.GetUserWallets(ctx, &walletv1.GetUserWalletsRequest{UserId: userID})
	if err != nil || len(listed.Wallets) != 0 {
		t.Fatalf("wallets after DeleteUserWallets() = %v, %v; want empty", listed, err)
	}
}
