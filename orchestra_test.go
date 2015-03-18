package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
})

var orcRespJson = `[{"id":"request1","status_code":200,"status":"200 OK","body":"OK"},{"id":"request2","status_code":200,"status":"200 OK","body":"OK"},{"id":"request3","status_code":200,"status":"200 OK","body":"OK"},{"id":"request4","status_code":200,"status":"200 OK","body":"OK"},{"id":"request5","status_code":200,"status":"200 OK","body":"OK"}]`

var orcRespDelim = `Id: request1,Status: 200 OK
OK
====================
Id: request2,Status: 200 OK
OK
====================
Id: request3,Status: 200 OK
OK
====================
Id: request4,Status: 200 OK
OK
====================
Id: request5,Status: 200 OK
OK`

var handRespJson = `[{"id":"id1","status_code":200,"status":"200 OK","body":"OK"},{"id":"id2","status_code":200,"status":"200 OK","body":"OK"}]`

func TestConn(t *testing.T) {
	testServer := httptest.NewServer(okHandler)
	conn := NewConn(ConnRequest{"sample", testServer.URL})
	err := conn.Fetch()
	if err != nil {
		t.Error(err)
	}
	body, err := conn.Response.ReadAll()
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(body, []byte("OK")) {
		t.Errorf("expected %v found %v", "OK", string(body))
	}
	testServer.Close()
}

func TestOrchestra(t *testing.T) {
	testServer := httptest.NewServer(okHandler)
	rs := make([]ConnRequest, 5)
	for i := 0; i < 5; i++ {
		rs[i] = ConnRequest{fmt.Sprint("request", i+1), testServer.URL}
	}
	orchestra := NewOrchestra(rs...)
	w := httptest.NewRecorder()
	orchestra.Process(w)
	if strings.TrimSpace(w.Body.String()) != orcRespJson {
		t.Errorf("expected %v found %v", orcRespJson, w.Body.String())
	}
	w = httptest.NewRecorder()
	orchestra.UseDelimiter("====================")
	orchestra.Process(w)
	if strings.TrimSpace(w.Body.String()) != orcRespDelim {
		t.Errorf("expected %v found %v", orcRespDelim, w.Body.String())
	}
	testServer.Close()
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
		t.Errorf("expected %v found %v", handRespJson, string(w.Body.Bytes()))
	}
}
