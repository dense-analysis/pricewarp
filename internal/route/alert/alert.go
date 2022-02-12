// Package alert defines routes for alerts
package alert

import (
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/w0rp/pricewarp/internal/session"
)

func HandleAlertList(writer http.ResponseWriter, request *http.Request) {
	user, err := session.LoadUserFromSession(request)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(writer, "database connection error\n")

		return
	}

	if user == nil {
		http.Redirect(writer, request, "/login", http.StatusFound)

		return
	}

	fmt.Fprintf(writer, "alert list\n")
}

func HandleSubmitAlert(writer http.ResponseWriter, request *http.Request) {
	user, err := session.LoadUserFromSession(request)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(writer, "database connection error\n")

		return
	}

	if user == nil {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(writer, "403: Forbidden\n")
		return
	}

	fmt.Fprintf(writer, "submit alert\n")
}

func HandleAlert(writer http.ResponseWriter, request *http.Request) {
	user, err := session.LoadUserFromSession(request)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(writer, "database connection error\n")

		return
	}

	if user == nil {
		http.Redirect(writer, request, "/login", http.StatusFound)
		return
	}

	id := mux.Vars(request)["id"]

	fmt.Fprintf(writer, "alert: %s\n", id)
}

func HandleUpdateAlert(writer http.ResponseWriter, request *http.Request) {
	user, err := session.LoadUserFromSession(request)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(writer, "database connection error\n")

		return
	}

	if user == nil {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(writer, "403: Forbidden\n")
		return
	}

	id := mux.Vars(request)["id"]

	fmt.Fprintf(writer, "update alert: %s\n", id)
}

func HandleDeleteAlert(writer http.ResponseWriter, request *http.Request) {
	user, err := session.LoadUserFromSession(request)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(writer, "database connection error\n")

		return
	}

	if user == nil {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(writer, "403: Forbidden\n")
		return
	}

	id := mux.Vars(request)["id"]

	fmt.Fprintf(writer, "delete alert: %s\n", id)
}
