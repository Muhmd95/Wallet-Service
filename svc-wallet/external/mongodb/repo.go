package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	//project imports
	"svc-wallet/internal/wallet"
	"svc-wallet/util/logger"
)

type mongoRepository struct {
	collection *mongo.Collection // this is the collection in the mongo database where the wallets are stored
}

// this is the constructor for the mongoRepository struct it takes a mongo database and
//
//	returns a repository interface (to make the service layer interact only with the interface functions)
func NewWalletRepository(ctx context.Context, db *mongo.Database) (wallet.Repository, error) {
	log := logger.Ctx(ctx)
	coll := db.Collection("wallets") // this is the collection in the mongo database where the wallets are stored
	// configure the phone number to be unique
	_, err := coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "phone_number", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("unique_phone"),
		},
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetName("wallets_by_user"),
		},
	})

	if err != nil {
		// Return the error to main.go so it can decide how to handle the failure
		log.Error().Err(err).Msg("Failed to create wallet indexes (from repo layer)")
		return nil, err
	}
	return &mongoRepository{collection: coll}, nil // return the mongoRepository struct with the collection (this is a repository)

}

func (r *mongoRepository) CreateWallet(ctx context.Context, w *wallet.Wallet) error {
	log := logger.Ctx(ctx)
	result, err := r.collection.InsertOne(ctx, w) // this is the method that
	// will insert the wallet into the collection in the mongo database
	// context is passed to know the timeout of therequest
	// if the request takes too long it will be cancelled
	//the time out of the request is embedded in the ctx
	// beside ctx contains meta data
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// service will handle this
			return wallet.ErrDuplicatePhone // return the domain error for duplicate phone number
		}
		log.Error().Err(err).Msg("Failed to insert wallet (from repo layer)")
		return err // return any other error
	}
	w.ID = result.InsertedID.(primitive.ObjectID) // update the wallet object with the generated ID
	return nil
}

func (r *mongoRepository) GetWalletByPhoneNumber(ctx context.Context, phoneNumber string) (*wallet.Wallet, error) {
	log := logger.Ctx(ctx)
	var resWallet wallet.Wallet
	err := r.collection.FindOne(ctx, bson.M{"phone_number": phoneNumber}).Decode(&resWallet)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, wallet.ErrWalletNotFound // return the domain error for wallet not found
		}
		log.Error().Err(err).Msg("Failed to find wallet by phone number (from repo layer)")
		return nil, err // return any other error
	}
	return &resWallet, nil
}

// may be refactored in phase 2 to use transactions and atomic operations
func (r *mongoRepository) UpdateWalletBalance(ctx context.Context, phoneNumber string, amount int64, refID string) (*wallet.Wallet, error) {
	log := logger.Ctx(ctx)
	// this is the method that will update the balance of the wallet in the collection in the mongo database
	var updatedWallet wallet.Wallet
	// filter := bson.M{}
	// retError := wallet.ErrWalletNotFound
	// err := r.collection.FindOne(ctx, filter).Decode(&updatedWallet)
	// if err == nil {
	// 	log.Info().Msg("The transactions was already processed (repo layer)")
	// 	return &updatedWallet, nil
	// }

	// if amount > 0 {
	// 	filter = bson.M{
	// 		"phone_number":   phoneNumber,
	// 		"balance":        bson.M{"$lte": wallet.WalletMax - amount},
	// 		"processed_refs": bson.M{"$ne": refID},
	// 	}
	// 	retError = wallet.ErrExceedsMaxBalance
	// } else {
	// 	filter = bson.M{
	// 		"phone_number":   phoneNumber,
	// 		"balance":        bson.M{"$gte": -amount},
	// 		"processed_refs": bson.M{"$ne": refID},
	// 	}
	// 	retError = wallet.ErrInsufficientBalance
	// }
	// the filter
	// to decrease the balance pass amount as negative value

	// i will remove all logic from the wallet service as the transactions handles all logic
	filter := bson.M{
		"phone_number":   phoneNumber,
		"processed_refs": bson.M{"$ne": refID},
	}
	update := bson.M{
		"$set": bson.M{"updated_at": time.Now(), "balance": amount}, // i am now fetching the balance from the transactions
		"$push": bson.M{
			"processed_refs": bson.M{
				"$each":  bson.A{refID}, // need to push an array to use slice, push and position methods
				"$slice": -80,
			},
		},
	} // the update operation
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After) // this is to return the updated document after the update
	// findoneandupdate will return the updated wallet and prevents race conditions
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedWallet)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// first check if was processed
			if err := r.collection.FindOne(ctx, bson.M{
				"phone_number":   phoneNumber,
				"processed_refs": refID,
			}).Decode(&updatedWallet); err == nil {
				log.Info().Msg("The transactions was already processed (repo layer)")
				return &updatedWallet, nil
			}
		}
		// 	// then it is not processed
		// 	return nil, retError // return the domain error for wallet not found
		// }
		log.Error().Err(err).Msg("Failed to update the wallet balance (from repo layer)")
		return nil, err // return any other error
	}
	return &updatedWallet, nil

}

func (r *mongoRepository) GetWalletByID(ctx context.Context, walletID string) (*wallet.Wallet, error) {
	log := logger.Ctx(ctx)
	var resWallet wallet.Wallet
	walletObjID, err := primitive.ObjectIDFromHex(walletID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to convert from string to objectID (from repo layer)")
		return nil, wallet.ErrInvalidWalletID
	}
	err = r.collection.FindOne(ctx, bson.M{"_id": walletObjID}).Decode(&resWallet)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, wallet.ErrWalletNotFound // return the domain error for wallet not found
		}
		log.Error().Err(err).Msg("Failed to find wallet by wallet id(from repo layer)")
		return nil, err // return any other error
	}
	return &resWallet, nil
}

func (r *mongoRepository) GetWalletsByUserID(ctx context.Context, userID string) ([]wallet.Wallet, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, wallet.ErrInvalidUserID
	}
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userObjID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	wallets := make([]wallet.Wallet, 0)
	if err := cursor.All(ctx, &wallets); err != nil {
		return nil, err
	}
	return wallets, nil
}

func (r *mongoRepository) DeleteWallet(ctx context.Context, walletID string) error {
	walletObjID, err := primitive.ObjectIDFromHex(walletID)
	if err != nil {
		return wallet.ErrInvalidWalletID
	}
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": walletObjID})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return wallet.ErrWalletNotFound
	}
	return nil
}

func (r *mongoRepository) DeleteUserWallets(ctx context.Context, userID string) ([]string, error) {
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, wallet.ErrInvalidUserID
	}
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userObjID}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}
	if _, err := r.collection.DeleteMany(ctx, bson.M{"user_id": userObjID}); err != nil {
		return nil, err
	}
	walletIDs := make([]string, len(rows))
	for i, row := range rows {
		walletIDs[i] = row.ID.Hex()
	}
	return walletIDs, nil
}
