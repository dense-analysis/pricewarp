package main

import (
	"log"
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/w0rp/pricewarp/internal/env"
	"github.com/w0rp/pricewarp/internal/session"
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

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/", handleIndex).Methods("GET")
	router.HandleFunc("/login", auth.HandleViewLoginForm).Methods("GET")
	router.HandleFunc("/login", auth.HandleLogin).Methods("POST")
	router.HandleFunc("/logout", auth.HandleLogout).Methods("POST")
	router.HandleFunc("/alert", alert.HandleAlertList).Methods("GET")
	router.HandleFunc("/alert", alert.HandleSubmitAlert).Methods("POST")
	router.HandleFunc("/alert/{id}", alert.HandleAlert).Methods("GET")
	router.HandleFunc("/alert/{id}", alert.HandleUpdateAlert).Methods("PUT")
	router.HandleFunc("/alert/{id}", alert.HandleDeleteAlert).Methods("DELETE")

	log.Println("Server started")

	// TODO: Make port configurable
	log.Fatal(http.ListenAndServe(":8000", router))
}
