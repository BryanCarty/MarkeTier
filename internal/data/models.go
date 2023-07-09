package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	BaseUsersModel     BaseUserAccountModel
	MarketierUserModel MarketierAccountModel
	ProductOwnerModel  ProductOwnerAccountModel
	Tokens             TokenModel
	ProductModel       ProductModel
	ProposalModel      ProposalModel
	ContactModel       ContactModel
	ReviewModel        ReviewModel
	/*Movies      MovieModel
	Permissions PermissionModel

	Users       UserModel*/
}

func NewModels(db *sql.DB) Models {
	return Models{
		BaseUsersModel:     BaseUserAccountModel{DB: db},
		MarketierUserModel: MarketierAccountModel{DB: db},
		ProductOwnerModel:  ProductOwnerAccountModel{DB: db},
		Tokens:             TokenModel{DB: db},
		ProductModel:       ProductModel{DB: db},
		ProposalModel:      ProposalModel{DB: db},
		ContactModel:       ContactModel{DB: db},
		ReviewModel:        ReviewModel{DB: db},
		/*Movies:      MovieModel{DB: db},
		Permissions: PermissionModel{DB: db},
		Tokens:      TokenModel{DB: db},
		Users:       UserModel{DB: db},*/
	}
}
