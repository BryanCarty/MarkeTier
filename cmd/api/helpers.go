package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"marketier/internal/validator"

	"github.com/julienschmidt/httprouter"
)

func (app *application) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

type envelope map[string]interface{}

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

func (app *application) parseMultipartForm(w http.ResponseWriter, r *http.Request, dst *ParseFormData) error {
	maxMemory := int64(8 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory) // Max file size 3MB
	err := r.ParseMultipartForm(maxMemory)
	if err != nil {
		switch {
		case errors.Is(err, http.ErrNotMultipart):
			return errors.New("Request is not multipart")
		case err.Error() == "http: request body too large":
			return errors.New("Exceeded maximum file size")
		default:
			return err
		}
	}
	defer r.Body.Close()
	for _, name := range dst.FileNames {
		file, _, err := r.FormFile(name)
		if err != nil {
			switch {
			case errors.Is(err, http.ErrMissingFile):
				return errors.New("Required file is missing")
			default:
				return err
			}
		}
		file.Close()
		dst.Files = append(dst.Files, file)
	}
	return nil
}

func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	return s
}

func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)

	if csv == "" {
		return defaultValue
	}

	return strings.Split(csv, ",")
}

func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	return i
}

func (app *application) background(fn func()) {
	app.wg.Add(1)

	go func() {

		defer app.wg.Done()

		defer func() {
			if err := recover(); err != nil {
				app.logger.PrintError(fmt.Errorf("%s", err), nil)
			}
		}()

		fn()
	}()
}

type ParseFormData struct {
	FileNames []string
	Files     []multipart.File
}

type Sizer interface {
	Size() int64
}

func validateProfileImage(v *validator.Validator, fileType string, size int64, width int, height int) {

	if fileType != "jpeg" && fileType != "png" {
		v.AddError("profile_image", "only png and jpeg images are supported")
	}

	maxImageSize := int64(4 << 20)
	minImageSize := int64(100_000)
	if size < minImageSize || size > maxImageSize {
		v.AddError("profile_image", "must be between 100,000 bytes and 4MB")
	}

	if width < 360 || height < 360 {
		v.AddError("profile_image", "width and height must be greater than 360 pixels")
	}

	if width != height {
		v.AddError("profile_image", "length must equal height")
	}

}

func validateProposalImage(v *validator.Validator, fileType string, size int64, width int, height int) {

	if fileType != "jpeg" && fileType != "png" {
		v.AddError("proposal_image", "only png and jpeg images are supported")
	}

	maxImageSize := int64(5 << 20)
	minImageSize := int64(500_000)
	if size < minImageSize || size > maxImageSize {
		v.AddError("proposal_image", "must be between 500,000 bytes and 5MB")
	}

	if width < 800 || height < 800 {
		v.AddError("proposal_image", "width and height must be greater than 800 pixels")
	}

	if width != height {
		v.AddError("proposal_image", "length must equal height")
	}

}

func validateProductImages(v *validator.Validator, fileTypes [3]string, sizes [3]int64, widths [3]int, heights [3]int) {
	for index, val := range fileTypes {
		if val != "jpeg" && val != "png" {
			v.AddError(fmt.Sprintf("product_image_%v", index+1), "only png and jpeg images are supported")
		}

		maxImageSize := int64(5 << 20)
		minImageSize := int64(500_000)
		if sizes[index] < minImageSize || sizes[index] > maxImageSize {
			v.AddError(fmt.Sprintf("product_image_%v", index+1), "must be between 500,000 bytes and 5MB")
		}

		if widths[index] < 800 || heights[index] < 800 {
			v.AddError(fmt.Sprintf("product_image_%v", index+1), "width and height must be greater than 800 pixels")
		}

		if widths[index] != heights[index] {
			v.AddError(fmt.Sprintf("product_image_%v", index+1), "length must equal height")
		}
	}
}
