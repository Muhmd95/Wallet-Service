package wallet

// the schema for the wallet collection in the database

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Wallet struct {
	// main attributes
	// used bson and not bson and json to not expose it
	// to the outside world as the DTO will be used for that purpose
	ID           primitive.ObjectID  `bson:"_id,omitempty"`
	PhoneNumber  string              `bson:"phone_number"`
	OwnerName    string              `bson:"owner_name"`
	Balance      int64               `bson:"balance"`
	CurrencyCode string              `bson:"currency_code"`
	FamilyID     *primitive.ObjectID `bson:"family_id,omitempty"` // a pointer because it can be null if the wallet is not part of a family
	NationalID   string				 `bson:"national_id"`
	BirthDate    time.Time			 `bson:"birth_date"`

	// time stamps
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}

// --- Domain Errors ---
// The service layer will check for these exact errors without knowing about
// MongoDB to completely separate service from db
var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrDuplicatePhone      = errors.New("phone number is already registered")
	ErrInvalidPhoneNumber  = errors.New("invalid phone number format")
	ErrInsufficientBalance = errors.New("insufficient balance for the requested operation")
	ErrExceedsMaxBalance   = errors.New("deposit exceeds maximum wallet capacity")
	ErrInvalidNationalID   = errors.New("invalid national id format")
	ErrInvalidWalletID	   = errors.New("invalid wallet id format")
)
