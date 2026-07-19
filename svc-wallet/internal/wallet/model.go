package wallet

// the schema for the wallet collection in the database

import (
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

	// time stamps
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}
