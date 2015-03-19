package main

import (
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

	log.Println("Orchestra listening on port " + port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal(err)
	}
}

type params struct {
	requests  string
	timeout   time.Duration
	respType  int
	delimiter string
}

func digestRequest(r *http.Request) params {
	rs := strings.TrimSpace(r.FormValue("requests"))
	rt := strings.ToLower(strings.TrimSpace(r.FormValue("type")))
	respType := -1
	switch rt {
	case "json":
		respType = typeJson
		break
	case "delimiter":
		respType = typeDelimiter
		break
	}
	if rt == "delimiter" {
		respType = typeDelimiter
	}
	var timeout time.Duration
	if t := strings.TrimSpace(r.FormValue("timeout")); t != "" {
		tms, _ := strconv.ParseInt(t, 10, 64)
		timeout = time.Duration(tms) * time.Millisecond
	}
	return params{
		rs,
		timeout,
		respType,
		r.FormValue("delimiter"),
	}
}

func handler(w http.ResponseWriter, r *http.Request) {

	log.Println(r.Method, r.URL.Path, r.URL.RawQuery)

	params := digestRequest(r)

	if params.requests == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(badRequestRequiredMsg))
		return
	}

	kv := strings.Split(params.requests, ",")
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
	if params.timeout > 0 {
		orchestra.SetTimeout(params.timeout)
	}

	if params.respType > -1 {
		switch params.respType {
		case typeDelimiter:
			orchestra.UseDelimeter()
			if params.delimiter != "" {
				orchestra.SetDelimiter(params.delimiter)
			}
			break
		default:
			orchestra.UseJson()
		}
	}
	orchestra.Process(w)
}
