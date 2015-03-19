package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

const badRequestInvalidMsg = "Bad Request: entries should be in comma separated multiple 'id:url' format e.g. 'sampleid:http://url.com,sampleid2:http://url2.com'"
const badRequestRequiredMsg = "Bad Request: required parameter 'requests' missing."

func main() {
	http.HandleFunc("/", handler)

	port := "8080"

	if len(os.Args) > 1 {
		port = os.Args[1]
	}

	fmt.Println("Orchestra listening on port " + port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {

	log.Println(r.Method, r.URL.Path, r.URL.RawQuery)

	rs := strings.TrimSpace(r.FormValue("requests"))
	if rs == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(badRequestRequiredMsg))
		return
	}

	var timeout time.Duration
	if t := strings.TrimSpace(r.FormValue("timeout")); t != "" {
		tms, _ := strconv.ParseInt(t, 10, 64)
		timeout = time.Duration(tms) * time.Millisecond
	}

	kv := strings.Split(rs, ",")
	crs := make([]ConnRequest, len(kv))

	for i, v := range kv {
		str := strings.SplitN(v, ":", 2)
		if len(str) < 2 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(badRequestInvalidMsg))
			return
		}
		crs[i] = ConnRequest{strings.TrimSpace(str[0]), strings.TrimSpace(str[1])}
	}

	orchestra := NewOrchestra(crs...)
	if timeout > 0 {
		orchestra.SetTimeout(timeout)
	}
	orchestra.Process(w)
}
