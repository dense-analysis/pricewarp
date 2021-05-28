package main

import (
    "flag"
    "fmt"
    "log"
    "net/http"
    "github.com/w0rp/w0rpboard/pkg/lax"
)

func readCliOptions() {
    debugPtr := flag.Bool("debug", false, "Run the server in debug mode")
    flag.Parse()

    if (*debugPtr) {
        lax.EnableDebugMode()
    }
}

type Post struct {
    Text string `json:"text"`
}

var postList []Post = make([]Post, 0, 20)

var postListHandler http.HandlerFunc = lax.Wrap(lax.View{
    Get: func(request *lax.Request) interface{} {
        return postList
    },
    Post: func(request *lax.Request) interface{} {
        post := Post{}

        if err := request.JSON(&post); err != nil {
            return lax.MakeBadRequestResponse(err)
        }

        if (len(post.Text) == 0 || len(post.Text) > 500) {
            return lax.MakeErrorListResponse(
                lax.Issue("text", "missing or invalid text"),
            )
        }

        postList = append(postList, post)

        return &post
    },
})

func main() {
    readCliOptions()

    http.HandleFunc("/post", postListHandler)

    fmt.Printf("Server started at localhost:8080\n")

    log.Fatal(http.ListenAndServe(":8080", nil))
}
