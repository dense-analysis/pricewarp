package util

import (
	"fmt"
	"log"
	"net/http"
)

func RespondInternalServerError(writer http.ResponseWriter, err error) {
	writer.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(writer, "Internal Server Error\n")
	log.Printf("internal error: %+v\n", err)
}

func RespondValidationError(writer http.ResponseWriter, message string) {
	writer.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(writer, "Validation Error: %s\n", message)
}

func RespondNotFound(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(writer, "404: Not Found\n")
}

func RespondForbidden(writer http.ResponseWriter) {
	writer.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(writer, "403: Forbidden\n")
}
