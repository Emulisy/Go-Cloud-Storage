package main


import (
	"net/http"
	"fmt"
	"github.com/Emulisy/goCloudStorage/handler"
)

func main(){
	http.HandleFunc("/file/upload", UploadHandler())
}

