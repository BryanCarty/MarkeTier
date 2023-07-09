package data

import (
	"context"
	"database/sql"
	"errors"
	"marketier/internal/validator"
	"time"
)

type Review struct {
	ReviewId int64  `json:"review_id"`
	UserId   int64  `json:"user_id"`
	Title    string `json:"title"`
	About    string `json:"about"`
	Version  int    `json:"version"`
}

type ReviewModel struct {
	DB *sql.DB
}

func ValidateReview(v *validator.Validator, review *Review) {
	v.Check(review.Title != "", "title", "must be provided")
	v.Check(len(review.Title) <= 250, "title", "must not be more than 250 bytes long")
	v.Check(review.About != "", "about", "must be provided")
	v.Check(len(review.About) <= 1000, "name", "must not be more than 1000 bytes long")
}

func (r ReviewModel) Insert(review *Review) error {
	query := `
        INSERT INTO reviews (user_id, title, about) 
        VALUES ($1, $2, $3)
        RETURNING review_id, version`

	args := []interface{}{review.UserId, review.Title, review.About}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.DB.QueryRowContext(ctx, query, args...).Scan(&review.ReviewId, &review.Version)
}

func (r ReviewModel) Get(id int64) (*Review, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
        SELECT review_id, user_id, title, about, version
        FROM reviews
        WHERE id = $1`

	var review Review

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&review.ReviewId,
		&review.UserId,
		&review.Title,
		&review.About,
		&review.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &review, nil
}

func (r ReviewModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
        DELETE FROM reviews
        WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := r.DB.ExecContext(ctx, query, id)
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

func (r ReviewModel) Update(review *Review) error {
	query := `
        UPDATE reviews 
        SET title = $1, about = $2, version = version + 1
        WHERE id = $4 AND version = $5
        RETURNING version`

	args := []interface{}{
		review.Title,
		review.About,
		review.ReviewId,
		review.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.DB.QueryRowContext(ctx, query, args...).Scan(&review.Version)
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
