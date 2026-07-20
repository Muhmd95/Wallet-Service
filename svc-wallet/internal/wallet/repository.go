package wallet

// i will create the interface and implementation of the repository layer
// in the same file so it will be easier to read and more compact

import (
	"context" // this is used to pass the context from the controller
	//  to the service layer and then to the repository layer so if the
	// http request is cancelled or times out the context will be cancelled
	//  and the repository layer will stop the database operation

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo" // this is used to interact with the mongo database
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	// this is the interface for the repository layer so that the service layer can use it
	// without knowing the implementation details of the repository layer
	CreateWallet(ctx context.Context, wallet *Wallet) error

	GetWalletByPhoneNumber(ctx context.Context, phoneNumber string) (*Wallet, error)

	UpdateWalletBalance(ctx context.Context, phoneNumber string, amount int64) (*Wallet, error)
}

type mongoRepository struct {
	collection *mongo.Collection // this is the collection in the mongo database where the wallets are stored
}

// this is the constructor for the mongoRepository struct it takes a mongo database and
//
//	returns a repository interface (to make the service layer interact only with the interface functions)
func NewMongoRepository(db *mongo.Database) (Repository, error) {
	coll := db.Collection("wallets") // this is the collection in the mongo database where the wallets are stored
	// configure the phone number to be unique
	_, err := coll.Indexes().CreateOne(context.Background(), mongo.IndexModel{ // creating an index on phnumber, context is blank cuz to context
		Keys:    bson.M{"phone_number": 1},                               // this is the index on the phone number field, 1 means ascending order
		Options: options.Index().SetUnique(true).SetName("unique_phone"), // this is the name of the index and it is unique so
		//that no two wallets can have the same phone number
	})

	if err != nil {
		// Return the error to main.go so it can decide how to handle the failure
		return nil, err
	}
	return &mongoRepository{collection: coll}, nil // return the mongoRepository struct with the collection (this is a repository)

}

func (r *mongoRepository) CreateWallet(ctx context.Context, wallet *Wallet) error {
	result, err := r.collection.InsertOne(ctx, wallet) // this is the method that
	// will insert the wallet into the collection in the mongo database and update the wallet
	// object with the generated ID
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicatePhone // return the domain error for duplicate phone number
		}
		return err // return any other error
	}
	wallet.ID = result.InsertedID.(primitive.ObjectID) // update the wallet object with the generated ID
	return nil
}

func (r *mongoRepository) GetWalletByPhoneNumber(ctx context.Context, phoneNumber string) (*Wallet, error) {
	var wallet Wallet
	err := r.collection.FindOne(ctx, bson.M{"phone_number": phoneNumber}).Decode(&wallet)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrWalletNotFound // return the domain error for wallet not found
		}
		return nil, err // return any other error
	}
	return &wallet, nil
}

// may be refactored in phase 2 to use transactions and atomic operations
func (r *mongoRepository) UpdateWalletBalance(ctx context.Context, phoneNumber string, amount int64) (*Wallet, error) {
	// this is the method that will update the balance of the wallet in the collection in the mongo database
	// i will check the business logic before in the service layer
	filter := bson.M{"phone_number": phoneNumber} // the filter
	// to decrease the balance pass amount as negative value
	update := bson.M{"$inc": bson.M{"balance": amount}}                 // the update operation
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After) // this is to return the updated document after the update
	// findoneandupdate will return the updated wallet and prevents race conditions
	var updatedWallet Wallet
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedWallet)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrWalletNotFound // return the domain error for wallet not found
		}
		return nil, err // return any other error
	}
	return &updatedWallet, nil

}
