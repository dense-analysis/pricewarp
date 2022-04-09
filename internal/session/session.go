// Package session handles saving/loading users to/from sessions
package session

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/model"
)

var sessionStore *sessions.CookieStore

// LoadEnvironmentVariables starts up session storage or crashes the program with an error
func InitSessionStorage() {
	secretKey := os.Getenv("SECRET_KEY")

	if len(secretKey) == 0 {
		fmt.Fprintf(os.Stderr, "No SECRET_KEY variable set!\n")
		os.Exit(1)
	}

	sessionStore = sessions.NewCookieStore([]byte(secretKey))
}

func LoadUserFromSession(conn *database.Conn, request *http.Request, user *model.User) (bool, error) {
	session, sessionError := sessionStore.Get(request, "sessionid")

	if sessionError != nil {
		// Ignore session errors, treat them as users not being found.
		return false, nil
	}

	if userID, ok := session.Values["userID"].(int); ok {
		row := conn.QueryRow(
			"select username from crypto_user where id = $1",
			userID,
		)

		var username string

		if err := row.Scan(&username); err != nil {
			if err == database.ErrNoRows {
				return false, nil
			}

			return false, err
		}

		*user = model.User{ID: userID, Username: username}

		return true, nil
	}

	return false, nil
}

func SaveUserInSession(writer http.ResponseWriter, request *http.Request, user *model.User) error {
	session, _ := sessionStore.Get(request, "sessionid")
	session.Values["userID"] = user.ID

	return session.Save(request, writer)
}

func ClearSession(writer http.ResponseWriter, request *http.Request) error {
	session, _ := sessionStore.Get(request, "sessionid")

	for key := range session.Values {
		delete(session.Values, key)
	}

	return session.Save(request, writer)
}
