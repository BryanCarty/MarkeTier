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

func (app *application) registerMarketierHandler(w http.ResponseWriter, r *http.Request) {
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

	user := &data.MarketierUserAccount{
		BaseUserAccount: data.BaseUserAccount{
			FirstName:   input.FirstName,
			LastName:    input.LastName,
			Email:       input.Email,
			DateOfBirth: input.DateOfBirth,
			Gender:      input.Gender,
			Address:     input.Address,
			AccountType: 2,
		},
		DisplayName:    input.DisplayName,
		About:          input.About,
		SalesGenerated: 0,
		Tier:           0,
	}

	err = user.BaseUserAccount.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if data.ValidateMarketierUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.MarketierUserModel.Insert(user)
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

func (app *application) showMarketier(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	marketier, err := app.models.MarketierUserModel.GetById(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": marketier}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateMarketiersHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	user, err := app.models.MarketierUserModel.GetById(id)
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

	if data.ValidateMarketierUser(v, user); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.MarketierUserModel.Update(user)
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

func (app *application) uploadProposalImageHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input ParseFormData
	input.FileNames = []string{"proposal_image"}
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
	validateProposalImage(v, fileType[1], input.Files[0].(Sizer).Size(), img.Bounds().Dx(), img.Bounds().Dy())

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	proposal_img_800 := resize.Resize(800, 800, img, resize.Lanczos3)

	proposal_img_400 := resize.Resize(400, 400, img, resize.Lanczos3)

	proposal_img_100 := resize.Resize(100, 100, img, resize.Lanczos3)

	imageMap := map[string]image.Image{
		fmt.Sprintf("../internals/images/proposal_images/proposal_img_800:%v.png", id): proposal_img_800,
		fmt.Sprintf("../internals/images/proposal_images/proposal_img_400:%v.png", id): proposal_img_400,
		fmt.Sprintf("../internals/images/proposal_images/proposal_img_100:%v.png", id): proposal_img_100,
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

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "proposal image saved successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
