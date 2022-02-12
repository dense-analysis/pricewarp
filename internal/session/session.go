// Package session handles saving/loading users to/from sessions
package session

import (
	"os"
	"fmt"
	"net/http"
	"github.com/gorilla/sessions"
	"github.com/w0rp/pricewarp/internal/model"
	"github.com/w0rp/pricewarp/internal/database"
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

func LoadUserFromSession(request *http.Request) (*model.User, error) {
	session, sessionError := sessionStore.Get(request, "sessionid")

	if sessionError != nil {
		return nil, nil
	}

	if userID, ok := session.Values["userID"].(int); ok {
		conn, connectionErr := database.Connect()

		if connectionErr != nil {
			return nil, connectionErr
		}

		defer conn.Close()

		row := conn.QueryRow(
			"select username from crypto_user where id = $1",
			userID,
		)

		var username string

		if err := row.Scan(&username); err == nil {
			return &model.User{ID: userID, Username: username}, nil
		}
	}

	return nil, nil
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
