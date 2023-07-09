package data

import (
	"context"
	"database/sql"
	"errors"
	"marketier/internal/validator"
	"time"
)

type Contact struct {
	ContactId int64  `json:"contact_id"`
	Subject   string `json:"subject"`
	About     string `json:"about"`
	Version   int    `json:"version"`
}

type ContactModel struct {
	DB *sql.DB
}

func ValidateContact(v *validator.Validator, contact *Contact) {
	v.Check(contact.Subject != "", "contact", "must be provided")
	v.Check(len(contact.Subject) <= 250, "contact", "must not be more than 250 bytes long")
	v.Check(contact.About != "", "about", "must be provided")
	v.Check(len(contact.About) <= 4000, "about", "must not be more than 4000 bytes long")
}

func (c ContactModel) Insert(contact *Contact) error {
	query := `
        INSERT INTO contacts (subject, about) 
        VALUES ($1, $2)
        RETURNING contact_id, version`

	args := []interface{}{contact.Subject, contact.About}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return c.DB.QueryRowContext(ctx, query, args...).Scan(&contact.ContactId, &contact.Version)
}

func (c ContactModel) Get(id int64) (*Contact, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
        SELECT contact_id, subject, about, version
        FROM contacts
        WHERE id = $1`

	var contact Contact

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := c.DB.QueryRowContext(ctx, query, id).Scan(
		&contact.ContactId,
		&contact.Subject,
		&contact.About,
		&contact.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &contact, nil
}

func (c ContactModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
        DELETE FROM contacts
        WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := c.DB.ExecContext(ctx, query, id)
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

func (c ContactModel) Update(contact *Contact) error {
	query := `
        UPDATE contacts 
        SET subject = $1, about = $2, version = version + 1
        WHERE id = $4 AND version = $5
        RETURNING version`

	args := []interface{}{
		contact.Subject,
		contact.About,
		contact.ContactId,
		contact.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := c.DB.QueryRowContext(ctx, query, args...).Scan(&contact.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}
