package auth

import (
	"fmt"
	"strings"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"github.com/w0rp/pricewarp/internal/model"
	"github.com/w0rp/pricewarp/internal/session"
	"github.com/w0rp/pricewarp/internal/database"
	"github.com/w0rp/pricewarp/internal/route/util"
)

var htmlTemplate = `<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="UTF-8">
		<title>Pricewarp</title>
	</head>
	<body>
		{htmlBody}
	</body>
</html>
`

func HandleViewLoginForm(writer http.ResponseWriter, request *http.Request) {
	loginFormBody := `
		<p>Log in!</p>
		{errorMessage}
		<form method="post">
			<label>
				Username:
				<input type="text" name="username">
			</label>
			<label>
				Password:
				<input type="password" name="password">
			</label>
			<input type="submit" value="Submit">
		</form>`

	errorMessage := ""

	if request.Method == "POST" {
		errorMessage = "<p>Invalid login!</p>"
	}

	pageHtmlTemplate := strings.Replace(htmlTemplate, "{htmlBody}", loginFormBody, 1)
	html := strings.Replace(pageHtmlTemplate, "{errorMessage}", errorMessage, 1)
	fmt.Fprint(writer, html)
}

func HandleLogin(writer http.ResponseWriter, request *http.Request) {
	request.ParseForm()
	username := request.Form.Get("username")
	password := request.Form.Get("password")

	conn, connectionErr := database.Connect()

	if connectionErr != nil {
		util.RespondInternalServerError(writer, connectionErr)

		return
	}

	defer conn.Close()

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
