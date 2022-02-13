package main

import (
	"os"
	"os/signal"
	"context"
	"syscall"
	"log"
	"fmt"
	"time"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/w0rp/pricewarp/internal/env"
	"github.com/w0rp/pricewarp/internal/session"
	"github.com/w0rp/pricewarp/internal/template"
	"github.com/w0rp/pricewarp/internal/route/auth"
	"github.com/w0rp/pricewarp/internal/route/alert"
)

func handleIndex(writer http.ResponseWriter, request *http.Request) {
	user, err := session.LoadUserFromSession(request)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(writer, "database connection error\n")

		return
	}

	if (user != nil) {
		http.Redirect(writer, request, "/alert", http.StatusFound)
	} else {
		http.Redirect(writer, request, "/login", http.StatusFound)
	}
}


func main() {
	env.LoadEnvironmentVariables()
	session.InitSessionStorage()
	template.Init()

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", handleIndex).Methods("GET")
	router.HandleFunc("/login", auth.HandleViewLoginForm).Methods("GET")
	router.HandleFunc("/login", auth.HandleLogin).Methods("POST")
	router.HandleFunc("/logout", auth.HandleLogout).Methods("POST")
	router.HandleFunc("/alert", alert.HandleAlertList).Methods("GET")
	router.HandleFunc("/alert", alert.HandleSubmitAlert).Methods("POST")
	router.HandleFunc("/alert/{id}", alert.HandleAlert).Methods("GET")
	router.HandleFunc("/alert/{id}", alert.HandleUpdateAlert).Methods("POST")
	router.HandleFunc("/alert/{id}", alert.HandleDeleteAlert).Methods("DELETE")

	// TODO: Only enable static files if a DEBUG flag is true
	fileServer := http.FileServer(http.Dir("./static/"))
	router.PathPrefix("/static/").
		Handler(http.StripPrefix("/static/", fileServer))

	// TODO: Make port configurable
	server := http.Server{
		Addr: ":8000",
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
