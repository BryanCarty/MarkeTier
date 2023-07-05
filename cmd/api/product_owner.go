package main

import (
	"errors"
	"marketier/internal/data"
	"marketier/internal/validator"
	"net/http"
	"time"
)

func (app *application) registerProductOwnerHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FirstName   string    `json:"first_name"`
		LastName    string    `json:"last_name"`
		Email       string    `json:"email"`
		DateOfBirth time.Time `json:"date_of_birth"`
		Gender      string    `json:"gender"`
		Address     string    `json:"address"`
		Password    string    `json:"password"`
		DisplayName string    `json:"display_name"`
		About       string    `json:"about"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.ProductOwnerUserAccount{
		BaseUserAccount: data.BaseUserAccount{
			FirstName:   input.FirstName,
			LastName:    input.LastName,
			Email:       input.Email,
			DateOfBirth: input.DateOfBirth,
			Gender:      input.Gender,
			Address:     input.Address,
			AccountType: 3,
		},
		DisplayName:    input.DisplayName,
		About:          input.About,
		SalesGenerated: 0,
	}

	err = user.BaseUserAccount.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateProductOwnerUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.ProductOwnerModel.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	token, err := app.models.Tokens.New(user.BaseUserAccount.UserId, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
			"userID":          user.BaseUserAccount.UserId,
		}

		err = app.mailer.Send(user.BaseUserAccount.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showProductOwner(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	productOwner, err := app.models.ProductOwnerModel.GetById(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": productOwner}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateProductOwnersHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	user, err := app.models.ProductOwnerModel.GetById(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	var input struct {
		Email       *string `json:"email"`
		Address     *string `json:"address"`
		Password    *string `json:"password"`
		DisplayName *string `json:"display_name"`
		About       *string `json:"about"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Email != nil {
		user.BaseUserAccount.Email = *input.Email
	}

	if input.Address != nil {
		user.BaseUserAccount.Address = *input.Address
	}

	if input.Password != nil {
		user.BaseUserAccount.Password.Set(*input.Password)
	}

	if input.DisplayName != nil {
		user.DisplayName = *input.DisplayName
	}

	if input.About != nil {
		user.About = *input.About
	}

	v := validator.New()

	if data.ValidateProductOwnerUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.ProductOwnerModel.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
