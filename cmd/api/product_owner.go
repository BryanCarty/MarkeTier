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

func (app *application) uploadProductImagesHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input ParseFormData
	input.FileNames = []string{"product_image_1", "product_image_2", "product_image_3"}
	err = app.parseMultipartForm(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	fileTypes := [3]string{}
	imgWidth := [3]int{}
	imgHeight := [3]int{}
	sizes := [3]int64{}
	decodedImages := [3]image.Image{}

	for index, file := range input.Files {
		sizes[index] = file.(Sizer).Size()

		fileHeader := make([]byte, 512)
		if n, err := file.Read(fileHeader); n == 0 || err != nil {
			app.badRequestResponse(w, r, err)
			return
		}
		fileType := strings.Split(http.DetectContentType(fileHeader), "/")[1]
		fileTypes[index] = fileType
		if _, err := file.Seek(0, 0); err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}
		img, _, err := image.Decode(file)
		if err != nil {
			app.badRequestResponse(w, r, err)
			return
		}
		decodedImages[index] = img
		imgWidth[index] = img.Bounds().Dx()
		imgWidth[index] = img.Bounds().Dy()
	}

	v := validator.New()

	validateProductImages(v, fileTypes, sizes, imgWidth, imgHeight)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	product_img_1_800 := resize.Resize(800, 800, decodedImages[0], resize.Lanczos3)

	product_img_1_400 := resize.Resize(400, 400, decodedImages[0], resize.Lanczos3)

	product_img_1_100 := resize.Resize(100, 100, decodedImages[0], resize.Lanczos3)

	product_img_2_800 := resize.Resize(800, 800, decodedImages[1], resize.Lanczos3)

	product_img_2_400 := resize.Resize(400, 400, decodedImages[1], resize.Lanczos3)

	product_img_2_100 := resize.Resize(100, 100, decodedImages[1], resize.Lanczos3)

	product_img_3_800 := resize.Resize(800, 800, decodedImages[2], resize.Lanczos3)

	product_img_3_400 := resize.Resize(400, 400, decodedImages[2], resize.Lanczos3)

	product_img_3_100 := resize.Resize(100, 100, decodedImages[2], resize.Lanczos3)

	imageMap := map[string]image.Image{
		fmt.Sprintf("../internals/images/product_images/product_img_1_800:%v.png", id): product_img_1_800,
		fmt.Sprintf("../internals/images/product_images/product_img_1_400:%v.png", id): product_img_1_400,
		fmt.Sprintf("../internals/images/product_images/product_img_1_100:%v.png", id): product_img_1_100,
		fmt.Sprintf("../internals/images/product_images/product_img_2_800:%v.png", id): product_img_2_800,
		fmt.Sprintf("../internals/images/product_images/product_img_2_400:%v.png", id): product_img_2_400,
		fmt.Sprintf("../internals/images/product_images/product_img_2_100:%v.png", id): product_img_2_100,
		fmt.Sprintf("../internals/images/product_images/product_img_3_800:%v.png", id): product_img_3_800,
		fmt.Sprintf("../internals/images/product_images/product_img_3_400:%v.png", id): product_img_3_400,
		fmt.Sprintf("../internals/images/product_images/product_img_3_100:%v.png", id): product_img_3_100,
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

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "product images saved successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
