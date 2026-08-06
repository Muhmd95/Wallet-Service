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
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{ // creating an index on phnumber, passing ctx to track the time
		Keys:    bson.M{"phone_number": 1},                               // this is the index on the phone number field, 1 means ascending order
		Options: options.Index().SetUnique(true).SetName("unique_phone"), // this is the name of the index and it is unique so
		//that no two wallets can have the same phone number
	})

	if err != nil {
		// Return the error to main.go so it can decide how to handle the failure
		log.Error().Err(err).Msg("Failed to create unique index on phone_number (from repo layer)")
		return nil, err
	}
	return &mongoRepository{collection: coll}, nil // return the mongoRepository struct with the collection (this is a repository)

}

func (r *mongoRepository) CreateWallet(ctx context.Context, w *wallet.Wallet) error {
	log := logger.Ctx(ctx)
	result, err := r.collection.InsertOne(ctx, w) // this is the method that
	// will insert the wallet into the collection in the mongo database and update the wallet
	// object with the generated ID
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
	filter := bson.M{}
	retError := wallet.ErrWalletNotFound
	// err := r.collection.FindOne(ctx, filter).Decode(&updatedWallet)
	// if err == nil {
	// 	log.Info().Msg("The transactions was already processed (repo layer)")
	// 	return &updatedWallet, nil
	// }

	if amount > 0 {
		filter = bson.M{
		"phone_number": phoneNumber,
		"balance": bson.M{"$lte": wallet.WalletMax - amount},
		"processed_refs": bson.M{"$ne": refID},
		}
		retError = wallet.ErrExceedsMaxBalance
	} else {
		filter = bson.M{
		"phone_number": phoneNumber,
		"balance": bson.M{"$gte": -amount},
		"processed_refs": bson.M{"$ne": refID},
		}
		retError = wallet.ErrInsufficientBalance
	}
	 // the filter
	// to decrease the balance pass amount as negative value
	update := bson.M{
		"$inc": bson.M{"balance": amount},
		"$set": bson.M{"updated_at": time.Now()},
		"$push": bson.M{
			"processed_refs": bson.M{
				"$each": bson.A{refID}, // need to push an array to use slice, push and position methods
				"$slice": -80,
			},
		},
		 }                 // the update operation
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After) // this is to return the updated document after the update
	// findoneandupdate will return the updated wallet and prevents race conditions
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedWallet)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// first check if was processed
			if err := r.collection.FindOne(ctx, bson.M{
				"phone_number": phoneNumber,
				"processed_refs": refID,
			}).Decode(&updatedWallet); err == nil {
				log.Info().Msg("The transactions was already processed (repo layer)")
				return &updatedWallet, nil
			}
			// then it is not processed
			return nil, retError // return the domain error for wallet not found
		}
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

