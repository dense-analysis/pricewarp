package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
	"github.com/dense-analysis/pricewarp/internal/model"
	"github.com/dense-analysis/pricewarp/internal/route/alert"
	"github.com/dense-analysis/pricewarp/internal/route/auth"
	"github.com/dense-analysis/pricewarp/internal/route/portfolio"
	"github.com/dense-analysis/pricewarp/internal/route/util"
	"github.com/dense-analysis/pricewarp/internal/session"
	"github.com/dense-analysis/pricewarp/internal/template"
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

	portfolioRoute := addDatabaseConnection(portfolio.HandlePortfolio)
	portfolioUpdateRoute := addDatabaseConnection(portfolio.HandlePortfolioUpdate)
	portfolioAssetRoute := addDatabaseConnection(portfolio.HandleAsset)
	portfolioBuyRoute := addDatabaseConnection(portfolio.HandleAssetBuy)
	portfolioSellRoute := addDatabaseConnection(portfolio.HandleAssetSell)

	router.HandleFunc("/", indexRoute).Methods("GET")
	router.HandleFunc("/login", auth.HandleViewLoginForm).Methods("GET")
	router.HandleFunc("/login", postLoginRoute).Methods("POST")
	router.HandleFunc("/logout", auth.HandleLogout).Methods("POST")
	router.HandleFunc("/alert", alertListRoute).Methods("GET")
	router.HandleFunc("/alert", alertCreateRoute).Methods("POST")
	router.HandleFunc("/alert/{id}", alertRoute).Methods("GET")
	router.HandleFunc("/alert/{id}", updateAlertRoute).Methods("POST")
	router.HandleFunc("/alert/{id}", deleteAlertRoute).Methods("DELETE")
	router.HandleFunc("/portfolio", portfolioRoute).Methods("GET")
	router.HandleFunc("/portfolio", portfolioUpdateRoute).Methods("POST")
	router.HandleFunc("/portfolio/{ticker}", portfolioAssetRoute).Methods("GET")
	router.HandleFunc("/portfolio/{ticker}/buy", portfolioBuyRoute).Methods("POST")
	router.HandleFunc("/portfolio/{ticker}/sell", portfolioSellRoute).Methods("POST")

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
		Addr:    address,
		Handler: router,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %s \n", err)
		}
	}()

	url := address

	if strings.HasPrefix(url, ":") {
		url = "localhost" + url
	}

	log.Printf("Server started at http://%s\n", url)
	<-done

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shut down failed: %+v", err)
	}

	log.Println("Server shut down successfully")
}
