package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK/" + r.URL.Path[1:]))
})

var tHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
	w.(http.Flusher).Flush()
	time.Sleep(3 * time.Second)
})

var orcRespJson = `[{"id":"request1","status_code":200,"status":"200 OK","body":"OK/1"},{"id":"request2","status_code":200,"status":"200 OK","body":"OK/2"},{"id":"request3","status_code":200,"status":"200 OK","body":"OK/3"},{"id":"request4","status_code":200,"status":"200 OK","body":"OK/4"},{"id":"request5","status_code":200,"status":"200 OK","body":"OK/5"}]`

var orcRespDelim = `Id: request1, Status: 200 OK
OK/1
====================
Id: request2, Status: 200 OK
OK/2
====================
Id: request3, Status: 200 OK
OK/3
====================
Id: request4, Status: 200 OK
OK/4
====================
Id: request5, Status: 200 OK
OK/5`

var handRespJson = `[{"id":"id1","status_code":200,"status":"200 OK","body":"OK/"},{"id":"id2","status_code":200,"status":"200 OK","body":"OK/"}]`

var handRespDelim = []string{`Id: id1, Status: 200 OK
OK/
---XXX---
Id: id2, Status: 200 OK
OK/`,
	`Id: id1, Status: 200 OK
OK/
000000
Id: id2, Status: 200 OK
OK/`,
}

func TestConn(t *testing.T) {
	testServer := httptest.NewServer(okHandler)
	conn := NewConn(ConnRequest{"sample", testServer.URL})
	err := conn.Fetch()
	if err != nil {
		t.Fatal(err)
	}
	body, err := conn.Response.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, []byte("OK/")) {
		t.Fatalf("expected %v found %v", "OK/", string(body))
	}
	testServer.Close()
}

func TestOrchestra(t *testing.T) {
	testServer := httptest.NewServer(okHandler)
	rs := make([]ConnRequest, 5)
	for i := 0; i < 5; i++ {
		rs[i] = ConnRequest{fmt.Sprint("request", i+1), fmt.Sprintf("%s/%d", testServer.URL, i+1)}
	}
	orchestra := NewOrchestra(rs...)
	w := httptest.NewRecorder()
	orchestra.Process(w)
	if strings.TrimSpace(w.Body.String()) != orcRespJson {
		t.Fatalf("expected %v found %v", orcRespJson, w.Body.String())
	}

	w = httptest.NewRecorder()
	orchestra.SetDelimiter("====================")
	orchestra.Process(w)
	if strings.TrimSpace(w.Body.String()) != orcRespDelim {
		t.Fatalf("expected %v found %v", orcRespDelim, w.Body.String())
	}
	testServer.Close()
}

func TestOrchestraAdd(t *testing.T) {
	testServer := httptest.NewServer(okHandler)
	rs := make([]ConnRequest, 4)
	for i := 0; i < 4; i++ {
		rs[i] = ConnRequest{fmt.Sprint("request", i+1), fmt.Sprintf("%s/%d", testServer.URL, i+1)}
	}
	orchestra := NewOrchestra(rs...)
	orchestra.Add(ConnRequest{fmt.Sprint("request", 5), fmt.Sprintf("%s/%d", testServer.URL, 5)})
	w := httptest.NewRecorder()
	orchestra.Process(w)
	if strings.TrimSpace(w.Body.String()) != orcRespJson {
		t.Fatalf("expected %v found %v", orcRespJson, w.Body.String())
	}
}

func TestTimeout(t *testing.T) {
	tServer := httptest.NewServer(tHandler)
	rs := make([]ConnRequest, 5)
	for i := 0; i < 5; i++ {
		rs[i] = ConnRequest{fmt.Sprint("request", i+1), fmt.Sprintf("%s/%d", tServer.URL, i+1)}
	}
	orchestra := NewOrchestra(rs...)
	orchestra.SetTimeout(2 * time.Second)
	w := httptest.NewRecorder()
	orchestra.Process(w)
	checkErrResp(t, w)
}

func TestHandler(t *testing.T) {
	oServer := httptest.NewServer(okHandler)
	req, err := http.NewRequest("GET", "/?requests=id1:"+oServer.URL+",id2:"+oServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	testHandler := http.HandlerFunc(handler)
	testHandler.ServeHTTP(w, req)
	if strings.TrimSpace(w.Body.String()) != handRespJson {
		t.Fatalf("expected %v found %v", handRespJson, string(w.Body.Bytes()))
	}
}

func TestHandlerTimeout(t *testing.T) {
	tServer := httptest.NewServer(tHandler)
	req, err := http.NewRequest("GET", "/?timeout=2000&requests=id1:"+tServer.URL+",id2:"+tServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	testHandler := http.HandlerFunc(handler)
	testHandler.ServeHTTP(w, req)
	checkErrResp(t, w)
}

func TestHandlerRespJson(t *testing.T) {
	tServer := httptest.NewServer(okHandler)
	req, err := http.NewRequest("GET", "/?type=json&requests=id1:"+tServer.URL+",id2:"+tServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	testHandler := http.HandlerFunc(handler)
	testHandler.ServeHTTP(w, req)
	if strings.TrimSpace(w.Body.String()) != handRespJson {
		t.Fatalf("expected %v found %v", handRespJson, w.Body.String())
	}
}

func TestHandlerRespDelim(t *testing.T) {
	tServer := httptest.NewServer(okHandler)
	req, err := http.NewRequest("GET", "/?type=delimeter&requests=id1:"+tServer.URL+",id2:"+tServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	testHandler := http.HandlerFunc(handler)
	testHandler.ServeHTTP(w, req)
	if strings.TrimSpace(w.Body.String()) != handRespDelim[0] {
		t.Fatalf("expected %v found %v", handRespDelim[0], w.Body.String())
	}
	req, err = http.NewRequest("GET", "/?type=delimiter&delimiter=000000&requests=id1:"+tServer.URL+",id2:"+tServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.Body.Reset()
	testHandler.ServeHTTP(w, req)
	if strings.TrimSpace(w.Body.String()) != handRespDelim[1] {
		t.Fatalf("expected %v found %v", handRespDelim[1], w.Body.String())
	}
}

func checkErrResp(t *testing.T, w *httptest.ResponseRecorder) {
	var m []interface{}
	err := json.Unmarshal(w.Body.Bytes(), &m)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range m {
		if v.(map[string]interface{})["error"] == nil {
			t.Fatal("error expected", v)
		}
	}
}
