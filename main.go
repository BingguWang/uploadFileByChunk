package main

import (
	"log"
	"net/http"
	"uploadFileByChunk/handler"
	"uploadFileByChunk/initRouter"
)

func init() {
	initRouter.InitRedis()
}
func main() {
	http.HandleFunc("/upload", handler.UploadFileHandler())
	http.HandleFunc("/progress", handler.GetProgress())
	http.HandleFunc("/merge", handler.MergeFile())
	err := http.ListenAndServe(":8088", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
