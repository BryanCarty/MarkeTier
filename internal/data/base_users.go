package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"marketier/internal/validator"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

var AnonymousUserAccount = &BaseUserAccount{}

type BaseUserAccount struct {
	UserId              int64      `json:"user_id"`
	FirstName           string     `json:"first_name"`
	LastName            string     `json:"last_name"`
	Email               string     `json:"email"`
	DateOfBirth         time.Time  `json:"date_of_birth"`
	Gender              string     `json:"gender"`
	Address             string     `json:"address"`
	Password            password   `json:"-"`
	AccountCreationTime time.Time  `json:"account_creation_time"`
	LastLoginTime       *time.Time `json:"last_login_time"`
	AccountStatus       string     `json:"account_status"`
	Version             int64      `json:"version"`
	AccountType         int8       `json:"account_type"`
}

func (u *BaseUserAccount) IsAnonymous() bool {
	return u == AnonymousUserAccount
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(len(email) <= 100, "email", "must be less than 100 bytes")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateBaseUser(v *validator.Validator, baseUserAccount *BaseUserAccount) {
	v.Check(baseUserAccount.FirstName != "", "name", "must be provided")
	v.Check(len(baseUserAccount.FirstName) <= 500, "name", "must not be more than 500 bytes long")
	v.Check(baseUserAccount.LastName != "", "name", "must be provided")
	v.Check(len(baseUserAccount.LastName) <= 500, "name", "must not be more than 500 bytes long")
	ValidateEmail(v, baseUserAccount.Email)
	v.Check(baseUserAccount.DateOfBirth.After(time.Now().AddDate(-150, 0, 0)), "date_of_birth", "you must not be more than 150 years old")
	v.Check(baseUserAccount.DateOfBirth.Before(time.Now().AddDate(-18, 0, 0)), "date_of_birth", "you must be more than 18 years old")
	v.Check(validator.In(baseUserAccount.Gender, []string{"male", "female", "other"}...), "gender", "must be 'male', 'female', or 'other'")
	v.Check(baseUserAccount.Address != "", "address", "must be provided")
	v.Check(len(baseUserAccount.Address) <= 2500, "address", "must be less than 2500 bytes long")
	if baseUserAccount.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *baseUserAccount.Password.plaintext)
	}
	if baseUserAccount.Password.hash == nil {
		panic("missing password hash for user")
	}
	v.Check(validator.In(strconv.Itoa(int(baseUserAccount.AccountType)), []string{"1", "2", "3", "4"}...), "account_type", "account_type id must be 1, 2, 3, or 4")
}

type BaseUserAccountModel struct {
	DB *sql.DB
}

func (baseUserModel BaseUserAccountModel) Insert(baseUser *BaseUserAccount) error {
	query := `
        INSERT INTO base_users (first_name, last_name, email, date_of_birth, gender, address, password, account_type) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING user_id, account_creation_time, account_status, version`

	args := []interface{}{baseUser.FirstName, baseUser.LastName, baseUser.Email, baseUser.DateOfBirth, baseUser.Gender, baseUser.Address, baseUser.Password.hash, baseUser.AccountType}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := baseUserModel.DB.QueryRowContext(ctx, query, args...).Scan(&baseUser.UserId, &baseUser.AccountCreationTime, &baseUser.AccountStatus, &baseUser.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "base_users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m BaseUserAccountModel) GetByEmail(email string) (*BaseUserAccount, error) {
	query := `
        SELECT user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type
        FROM base_users
        WHERE email = $1`

	var baseUser BaseUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&baseUser.UserId,
		&baseUser.FirstName,
		&baseUser.LastName,
		&baseUser.Email,
		&baseUser.DateOfBirth,
		&baseUser.Gender,
		&baseUser.Address,
		&baseUser.Password.hash,
		&baseUser.AccountCreationTime,
		&baseUser.LastLoginTime,
		&baseUser.AccountStatus,
		&baseUser.Version,
		&baseUser.AccountType,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &baseUser, nil
}

func (m BaseUserAccountModel) Update(baseUser *BaseUserAccount) error {
	query := `
        UPDATE base_users
        SET first_name = $1, last_name = $2, email = $3, address = $4, password = $5, last_login_time = $6, account_status = $7, version = version + 1
        WHERE user_id = $8 AND version = $9
        RETURNING version`

	args := []interface{}{
		baseUser.FirstName,
		baseUser.LastName,
		baseUser.Email,
		baseUser.Address,
		baseUser.Password.hash,
		baseUser.LastLoginTime,
		baseUser.AccountStatus,
		baseUser.UserId,
		baseUser.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&baseUser.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m BaseUserAccountModel) GetForToken(tokenScope, tokenPlaintext string) (*BaseUserAccount, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
		SELECT base_users.user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type
        FROM base_users
        INNER JOIN tokens
        ON base_users.user_id = tokens.user_id
        WHERE tokens.hash = $1
        AND tokens.scope = $2
        AND tokens.expiry > $3`

	args := []interface{}{tokenHash[:], tokenScope, time.Now()}

	var baseUser BaseUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&baseUser.UserId,
		&baseUser.FirstName,
		&baseUser.LastName,
		&baseUser.Email,
		&baseUser.DateOfBirth,
		&baseUser.Gender,
		&baseUser.Address,
		&baseUser.Password.hash,
		&baseUser.AccountCreationTime,
		&baseUser.LastLoginTime,
		&baseUser.AccountStatus,
		&baseUser.Version,
		&baseUser.AccountType,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &baseUser, nil
}

func (baseUserAccountModel BaseUserAccountModel) GetById(id int64) (*BaseUserAccount, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}
	query := `
	SELECT user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type
	FROM base_users
	WHERE user_id = $1`

	var baseUser BaseUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := baseUserAccountModel.DB.QueryRowContext(ctx, query, id).Scan(
		&baseUser.UserId,
		&baseUser.FirstName,
		&baseUser.LastName,
		&baseUser.Email,
		&baseUser.DateOfBirth,
		&baseUser.Gender,
		&baseUser.Address,
		&baseUser.Password.hash,
		&baseUser.AccountCreationTime,
		&baseUser.LastLoginTime,
		&baseUser.AccountStatus,
		&baseUser.Version,
		&baseUser.AccountType,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &baseUser, nil
}

func (baseUserAccountModel BaseUserAccountModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
        DELETE FROM base_users
        WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := baseUserAccountModel.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
