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
func (r *mongoRepository) UpdateWalletBalance(ctx context.Context, phoneNumber string, amount int64) (*wallet.Wallet, error) {
	log := logger.Ctx(ctx)
	// this is the method that will update the balance of the wallet in the collection in the mongo database
	// i will check the business logic before in the service layer
	filter := bson.M{"phone_number": phoneNumber} // the filter
	// to decrease the balance pass amount as negative value
	update := bson.M{"$inc": bson.M{"balance": amount}, "$set": bson.M{"updated_at": time.Now()}} // the update operation
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)                           // this is to return the updated document after the update
	// findoneandupdate will return the updated wallet and prevents race conditions
	var updatedWallet wallet.Wallet
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedWallet)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, wallet.ErrWalletNotFound // return the domain error for wallet not found
		}
		log.Error().Err(err).Msg("Failed to update the wallet balance (from repo layer)")
		return nil, err // return any other error
	}
	return &updatedWallet, nil

}
