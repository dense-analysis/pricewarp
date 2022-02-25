package main

import (
	"os"
	"os/signal"
	"context"
	"syscall"
	"log"
	"time"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/w0rp/pricewarp/internal/env"
	"github.com/w0rp/pricewarp/internal/database"
	"github.com/w0rp/pricewarp/internal/model"
	"github.com/w0rp/pricewarp/internal/session"
	"github.com/w0rp/pricewarp/internal/template"
	"github.com/w0rp/pricewarp/internal/route/util"
	"github.com/w0rp/pricewarp/internal/route/auth"
	"github.com/w0rp/pricewarp/internal/route/alert"
	"github.com/w0rp/pricewarp/internal/route/portfolio"
)

func handleIndex(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	var user model.User
	found, err := session.LoadUserFromSession(conn, request, &user)

	if err != nil {
		util.RespondInternalServerError(writer, err)
	} else if found {
		http.Redirect(writer, request, "/alert", http.StatusFound)
	} else {
		http.Redirect(writer, request, "/login", http.StatusFound)
	}
}

func addDatabaseConnection(
	f func(*database.Conn, http.ResponseWriter, *http.Request),
) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		conn, err := database.Connect()

		if err != nil {
			util.RespondInternalServerError(writer, err)
		} else {
			defer conn.Close()
			f(conn, writer, request)
		}
	}
}

func main() {
	env.LoadEnvironmentVariables()
	session.InitSessionStorage()
	template.Init()

	router := mux.NewRouter().StrictSlash(true)

	indexRoute := addDatabaseConnection(handleIndex)

	postLoginRoute := addDatabaseConnection(auth.HandleLogin)

	alertListRoute := addDatabaseConnection(alert.HandleAlertList)
	alertCreateRoute := addDatabaseConnection(alert.HandleSubmitAlert)
	alertRoute := addDatabaseConnection(alert.HandleAlert)
	updateAlertRoute := addDatabaseConnection(alert.HandleUpdateAlert)
	deleteAlertRoute := addDatabaseConnection(alert.HandleDeleteAlert)

	portfolioListRoute := addDatabaseConnection(portfolio.HandlePortfolioList)
	portfolioUpdateRoute := addDatabaseConnection(portfolio.HandlePortfolioUpdate)
	portfolioBuyRoute := addDatabaseConnection(portfolio.HandlePortfolioBuy)
	portfolioSellRoute := addDatabaseConnection(portfolio.HandlePortfolioSell)
	portfolioAssetRoute := addDatabaseConnection(portfolio.HandlePortfolioAsset)

	router.HandleFunc("/", indexRoute).Methods("GET")
	router.HandleFunc("/login", auth.HandleViewLoginForm).Methods("GET")
	router.HandleFunc("/login", postLoginRoute).Methods("POST")
	router.HandleFunc("/logout", auth.HandleLogout).Methods("POST")
	router.HandleFunc("/alert", alertListRoute).Methods("GET")
	router.HandleFunc("/alert", alertCreateRoute).Methods("POST")
	router.HandleFunc("/alert/{id}", alertRoute).Methods("GET")
	router.HandleFunc("/alert/{id}", updateAlertRoute).Methods("POST")
	router.HandleFunc("/alert/{id}", deleteAlertRoute).Methods("DELETE")
	router.HandleFunc("/portfolio", portfolioListRoute).Methods("GET")
	router.HandleFunc("/portfolio", portfolioUpdateRoute).Methods("POST")
	router.HandleFunc("/portfolio/{id}", portfolioAssetRoute).Methods("GET")
	router.HandleFunc("/portfolio/{id}/buy", portfolioBuyRoute).Methods("POST")
	router.HandleFunc("/portfolio/{id}/sell", portfolioSellRoute).Methods("POST")

	if os.Getenv("DEBUG") == "true" {
		fileServer := http.FileServer(http.Dir("./static/"))
		router.PathPrefix("/static/").
			Handler(http.StripPrefix("/static/", fileServer))
	}

	address := os.Getenv("ADDRESS")

	if len(address) == 0 {
		address = ":8000"
	}

	server := http.Server{
		Addr: address,
		Handler: router,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %s \n", err)
		}
	}()

	log.Println("Server started")
	<-done

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer func() {
		cancel()
	}()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shut down failed: %+v", err)
	}

	log.Println("Server shut down successfully")
}
