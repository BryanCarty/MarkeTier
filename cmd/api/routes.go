package main

import (
	"expvar"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (app *application) routes() http.Handler {
	router := httprouter.New()

	router.NotFound = http.HandlerFunc(app.notFoundResponse)
	router.MethodNotAllowed = http.HandlerFunc(app.methodNotAllowedResponse)

	router.HandlerFunc(http.MethodPost, "/v1/users/shoppers", app.registerBaseUserHandler)
	router.HandlerFunc(http.MethodPost, "/v1/users/marketiers", app.registerMarketierHandler)
	router.HandlerFunc(http.MethodPost, "/v1/users/product_owners", app.registerProductOwnerHandler)

	router.HandlerFunc(http.MethodGet, "/v1/users/shoppers/:id", app.requirePermission([]int8{0, 3, 4}, app.showShopper)) //other shoppers and general public can't view other shoppers
	router.HandlerFunc(http.MethodGet, "/v1/users/marketiers/:id", app.showMarketier)
	router.HandlerFunc(http.MethodGet, "/v1/users/product_owners/:id", app.showProductOwner)

	router.HandlerFunc(http.MethodPatch, "v1/users/shoppers/:id", app.requirePermission([]int8{0, 4}, app.updateShoppersHandler))
	router.HandlerFunc(http.MethodPatch, "v1/users/marketiers/:id", app.requirePermission([]int8{0, 4}, app.updateMarketiersHandler))
	router.HandlerFunc(http.MethodPatch, "v1/users/product_owners/:id", app.requirePermission([]int8{0, 4}, app.updateProductOwnersHandler))

	router.HandlerFunc(http.MethodDelete, "v1/users/shoppers/:id", app.requirePermission([]int8{0, 4}, app.deleteBaseUserHandler))
	router.HandlerFunc(http.MethodDelete, "v1/users/marketiers/:id", app.requirePermission([]int8{0, 4}, app.deleteBaseUserHandler))
	router.HandlerFunc(http.MethodDelete, "v1/users/product_owners/:id", app.requirePermission([]int8{0, 4}, app.deleteBaseUserHandler))

	router.HandlerFunc(http.MethodPut, "/v1/users/activated", app.activateBaseUserHandler)
	router.HandlerFunc(http.MethodPut, "/v1/users/password", app.updateBaseUserPasswordHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/authentication", app.createAuthenticationTokenHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/activation", app.createActivationTokenHandler)
	router.HandlerFunc(http.MethodPost, "/v1/tokens/password-reset", app.createPasswordResetTokenHandler)

	router.Handler(http.MethodGet, "/debug/vars", expvar.Handler())

	return app.metrics(app.recoverPanic(app.enableCORS(app.rateLimit(app.authenticate(router)))))
}

/**
router.HandlerFunc(http.MethodGet, "/v1/healthcheck", app.healthcheckHandler)

router.HandlerFunc(http.MethodGet, "/v1/movies", app.requirePermission("movies:read", app.listMoviesHandler))
router.HandlerFunc(http.MethodPost, "/v1/movies", app.requirePermission("movies:write", app.createMovieHandler))
router.HandlerFunc(http.MethodGet, "/v1/movies/:id", app.requirePermission("movies:read", app.showMovieHandler))
router.HandlerFunc(http.MethodPatch, "/v1/movies/:id", app.requirePermission("movies:write", app.updateMovieHandler))
router.HandlerFunc(http.MethodDelete, "/v1/movies/:id", app.requirePermission("movies:write", app.deleteMovieHandler))
**/
