package handler

import (
	"net/http"
	"io"
	"os"
)

//handle file upload
func UploadHandler(w http.ResponseWriter, r *http.Request){
	if r.Method == "GET"{
		data,err := os.ReadFile("../static/view/index.html")
		if err != nil {
			io.WriteString(w, "internal server error")
			return
		}
		io.Writer.Write(w, data)
	}else if(r.Method == "POST"){

	}
}