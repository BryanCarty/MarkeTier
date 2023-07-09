package main

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"marketier/internal/data"
	"marketier/internal/validator"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nfnt/resize"
)

func (app *application) registerBaseUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FirstName   string    `json:"first_name"`
		LastName    string    `json:"last_name"`
		Email       string    `json:"email"`
		DateOfBirth time.Time `json:"date_of_birth"`
		Gender      string    `json:"gender"`
		Address     string    `json:"address"`
		Password    string    `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := &data.BaseUserAccount{
		FirstName:   input.FirstName,
		LastName:    input.LastName,
		Email:       input.Email,
		DateOfBirth: input.DateOfBirth,
		Gender:      input.Gender,
		Address:     input.Address,
		AccountType: 1,
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateBaseUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.BaseUsersModel.Insert(user)
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

	token, err := app.models.Tokens.New(user.UserId, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
			"userID":          user.UserId,
		}

		err = app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	err = app.writeJSON(w, http.StatusAccepted, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) activateBaseUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.BaseUsersModel.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	user.AccountStatus = "ACTIVATED"

	err = app.models.BaseUsersModel.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopeActivation, user.UserId)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateBaseUserPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password       string `json:"password"`
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	data.ValidatePasswordPlaintext(v, input.Password)
	data.ValidateTokenPlaintext(v, input.TokenPlaintext)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user, err := app.models.BaseUsersModel.GetForToken(data.ScopePasswordReset, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired password reset token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = user.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.models.BaseUsersModel.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.models.Tokens.DeleteAllForUser(data.ScopePasswordReset, user.UserId)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	env := envelope{"message": "your password was successfully reset"}

	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showShopper(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	shopper, err := app.models.BaseUsersModel.GetById(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": shopper}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

/**
type BaseUserAccount struct {
	UserId              int64     `json:"user_id"`
	FirstName           string    `json:"first_name"`
	LastName            string    `json:"last_name"`
	Email               string    `json:"email"`
	DateOfBirth         time.Time `json:"date_of_birth"`
	Gender              string    `json:"gender"`
	Address             string    `json:"address"`
	Password            password  `json:"-"`
	AccountCreationTime time.Time `json:"account_creation_time"`
	LastLoginTime       time.Time `json:"last_login_time"`
	AccountStatus       string    `json:"account_status"`
	Version             int64     `json:"version"`
	AccountType         int8      `json:"account_type"`
}**/

func (app *application) updateShoppersHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	user, err := app.models.BaseUsersModel.GetById(id)
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
		Email    *string `json:"email"`
		Address  *string `json:"address"`
		Password *string `json:"password"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Email != nil {
		user.Email = *input.Email
	}

	if input.Address != nil {
		user.Address = *input.Address
	}

	if input.Password != nil {
		user.Password.Set(*input.Password)
	}

	v := validator.New()

	if data.ValidateBaseUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.BaseUsersModel.Update(user)
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

func (app *application) deleteBaseUserHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.BaseUsersModel.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "user successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) uploadProfileImageHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input ParseFormData
	input.FileNames = []string{"profile_image"}
	err = app.parseMultipartForm(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	fileHeader := make([]byte, 512)

	// Copy the headers into the FileHeader buffer
	if n, err := input.Files[0].Read(fileHeader); n == 0 || err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// set position back to start.
	if _, err := input.Files[0].Seek(0, 0); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	img, _, err := image.Decode(input.Files[0])
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	fileType := strings.Split(http.DetectContentType(fileHeader), "/")
	validateProfileImage(v, fileType[1], input.Files[0].(Sizer).Size(), img.Bounds().Dx(), img.Bounds().Dy())

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	profile_img_360 := resize.Resize(360, 360, img, resize.Lanczos3)

	// Resize the image to 128x128
	profile_img_180 := resize.Resize(180, 180, img, resize.Lanczos3)

	// Resize the image to 40x40
	profile_img_40 := resize.Resize(40, 40, img, resize.Lanczos3)

	imageMap := map[string]image.Image{
		fmt.Sprintf("../internals/images/profile_images/profile_img_360:%v.png", id): profile_img_360,
		fmt.Sprintf("../internals/images/profile_images/profile_img_180:%v.png", id): profile_img_180,
		fmt.Sprintf("../internals/images/profile_images/profile_img_40:%v.png", id):  profile_img_40,
	}

	// Open a file to write the image data
	for name, img := range imageMap {
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		// Encode and save the image as PNG
		err = png.Encode(file, img)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		file.Close()
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "profile image saved successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
