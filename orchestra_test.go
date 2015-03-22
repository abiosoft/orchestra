package main

import (
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

var testResp = `{sample 200 200 OK %s  }`

var orcRespJson = `[{"id":"request1","status_code":200,"status":"200 OK","duration":"%s","body":"OK/1"},{"id":"request2","status_code":200,"status":"200 OK","duration":"%s","body":"OK/2"},{"id":"request3","status_code":200,"status":"200 OK","duration":"%s","body":"OK/3"},{"id":"request4","status_code":200,"status":"200 OK","duration":"%s","body":"OK/4"},{"id":"request5","status_code":200,"status":"200 OK","duration":"%s","body":"OK/5"}]`

var orcRespDelim = `Id: request1, Status: 200 OK, Duration: %s
OK/1
====================
Id: request2, Status: 200 OK, Duration: %s
OK/2
====================
Id: request3, Status: 200 OK, Duration: %s
OK/3
====================
Id: request4, Status: 200 OK, Duration: %s
OK/4
====================
Id: request5, Status: 200 OK, Duration: %s
OK/5`

var handRespJson = `[{"id":"id1","status_code":200,"status":"200 OK","duration":"%s","body":"OK/"},{"id":"id2","status_code":200,"status":"200 OK","duration":"%s","body":"OK/"}]`

var handRespDelim = []string{`Id: id1, Status: 200 OK, Duration: %s
OK/
---XXX---
Id: id2, Status: 200 OK, Duration: %s
OK/`,
	`Id: id1, Status: 200 OK, Duration: %s
OK/
000000
Id: id2, Status: 200 OK, Duration: %s
OK/`,
}

func TestConn(t *testing.T) {
	testServer := httptest.NewServer(okHandler)
	conn := NewConn(ConnRequest{"sample", testServer.URL})
	err := conn.Fetch()
	if err != nil {
		t.Fatal(err)
	}
	if conn.Response == nil {
		t.Fatal("conn.Response should not be nil")
	}
	testResp := insertDurations(testResp, conn)
	if out := fmt.Sprint(conn.Response.output()); out != testResp {
		t.Fatalf("Expected %v found %v", testResp, out)
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
	orcRespJson := insertDurations(orcRespJson, orchestra.conns...)
	if strings.TrimSpace(w.Body.String()) != orcRespJson {
		t.Fatalf("expected %v found %v", orcRespJson, w.Body.String())
	}

	w = httptest.NewRecorder()
	orchestra.SetDelimiter("====================")
	orchestra.Process(w)
	orcRespDelim := insertDurations(orcRespDelim, orchestra.conns...)
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
	orcRespJson := insertDurations(orcRespJson, orchestra.conns...)
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
	compareJsonsMinusDuration([]byte(handRespJson), w.Body.Bytes(), t)
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
	if !compareJsonsMinusDuration([]byte(handRespJson), w.Body.Bytes(), t) {
		t.Fatalf("expected %v found %v", handRespJson, w.Body.String())
	}
}

func TestHandlerRespDelim(t *testing.T) {
	tServer := httptest.NewServer(okHandler)
	req, err := http.NewRequest("GET", "/?type=delimiter&requests=id1:"+tServer.URL+",id2:"+tServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	testHandler := http.HandlerFunc(handler)
	testHandler.ServeHTTP(w, req)
	if !compareDelimsMinusDurations(strings.TrimSpace(w.Body.String()), handRespDelim[0], defaultDelimiter, t) {
		t.Fatalf("expected %v found %v", handRespDelim[0], w.Body.String())
	}
	req, err = http.NewRequest("GET", "/?type=delimiter&delimiter=000000&requests=id1:"+tServer.URL+",id2:"+tServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.Body.Reset()
	testHandler.ServeHTTP(w, req)
	if !compareDelimsMinusDurations(strings.TrimSpace(w.Body.String()), handRespDelim[1], "\n000000\n", t) {
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

func insertDurations(s string, conns ...*Conn) string {
	durs := make([]interface{}, len(conns))
	for i := range conns {
		durs[i] = conns[i].Response.durationStr()
	}
	return fmt.Sprintf(s, durs...)
}

func compareJsonsMinusDuration(s, s1 []byte, t *testing.T) bool {
	var m []interface{}
	var m1 []interface{}
	err := json.Unmarshal(s, &m)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(s1, &m1)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != len(m1) {
		t.Fatal("different json sizes")
	}
	keys := []string{"id", "status_code", "status", "body"}
	for i := 0; i < len(m); i++ {
		for _, k := range keys {
			if m[i].(map[string]interface{})[k] != m1[i].(map[string]interface{})[k] {
				return false
			}
		}
	}
	return true
}

func compareDelimsMinusDurations(s, s1, delim string, t *testing.T) bool {
	str := strings.Split(s, delim)
	str1 := strings.Split(s1, delim)
	equal := true
	if len(str) != len(str1) {
		t.Fatal("different output sizes")
	}
	for i := 0; i < len(str); i++ {
		st := strings.Split(str[i], "\n")
		st1 := strings.Split(str1[i], "\n")
		if len(st) != len(st1) {
			equal = false
			break
		}
		for k, v := range st {
			if k == 0 {
				j := strings.Index(v, "Duration")
				if st1[k][:j+1] != v[:j+1] {
					equal = false
					break
				}
				continue
			}
			if v != st1[k] {
				equal = false
				break
			}
		}
	}
	return equal
}
