package auth

import (
	"net/http"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/model"
	"github.com/dense-analysis/pricewarp/internal/session"
	"github.com/dense-analysis/pricewarp/internal/template"
	"golang.org/x/crypto/bcrypt"
)

type LoginFormData struct {
	User         *model.User
	ErrorMessage string
}

func HandleViewLoginForm(writer http.ResponseWriter, request *http.Request) {
	data := LoginFormData{}

	if request.Method == "POST" {
		data.ErrorMessage = "Invalid login!"
	}

	template.Render(template.Login, writer, data)
}

func HandleLogin(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	username := request.Form.Get("username")
	password := request.Form.Get("password")

	var userID int
	loginValid := false

	if len(username) > 0 && len(password) > 0 {
		row := conn.QueryRow(
			"select id, password from crypto_user where username = $1",
			username,
		)

		var passwordHash string

		if err := row.Scan(&userID, &passwordHash); err == nil {
			if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil {
				loginValid = true
			}
		}
	}

	if loginValid {
		session.SaveUserInSession(writer, request, &model.User{ID: userID, Username: username})
		http.Redirect(writer, request, "/alert", http.StatusFound)
	} else {
		HandleViewLoginForm(writer, request)
	}
}

func HandleLogout(writer http.ResponseWriter, request *http.Request) {
	session.ClearSession(writer, request)
	http.Redirect(writer, request, "/login", http.StatusFound)
}
