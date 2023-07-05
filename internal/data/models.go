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
		/*Movies:      MovieModel{DB: db},
		Permissions: PermissionModel{DB: db},
		Tokens:      TokenModel{DB: db},
		Users:       UserModel{DB: db},*/
	}
}
