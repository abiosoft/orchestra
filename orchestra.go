// Orchestration Layer
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	typeJson      = iota
	typeDelimiter = iota
)

var errInvalidResponseType = errors.New("Invalid Response Type specified. Must be one of TypeJson, TypeDelimiter")

// Orchestra is the high level representation of the Orchestration Layer
type Orchestra struct {
	conns        []*Conn
	responseType uint8
	cLock        *sync.Mutex
	delimiter    string
}

// ConnRequest is the representation of the Connection Request used to initialize the Orchestra
type ConnRequest struct {
	// id is the identification of the Connection
	id string
	// url is the URL that the request will be sent to
	url string
}

// NewOrchestra creates a new orchestra. It initializes with ConnRequest(s)
func NewOrchestra(requests ...ConnRequest) *Orchestra {
	conns := make([]*Conn, len(requests))
	for i := range requests {
		conns[i] = NewConn(requests[i])
	}
	return &Orchestra{
		conns,
		typeJson,
		&sync.Mutex{},
		"",
	}
}

// Add adds a new Connection Request to the Orchestrac
func (o *Orchestra) Add(r ConnRequest) {
	o.cLock.Lock()
	defer o.cLock.Unlock()
	conn := NewConn(r)
	o.conns = append(o.conns, conn)
}

// UseDelimiter instructs the Orchestra to use separate plain text outputs with delimeter instead of Json
func (o *Orchestra) UseDelimiter(d string) {
	o.delimiter = d
	if !strings.HasSuffix(d, "\n") {
		o.delimiter += "\n"
	}
	o.responseType = typeDelimiter
}

// UseJson instructs the Orchestra to use Json for output
func (o *Orchestra) UseJson() {
	o.responseType = typeJson
}

// Process processes all connection requests and send them concurrently
// When done, it outputs to w
func (o *Orchestra) Process(w http.ResponseWriter) {
	var wg sync.WaitGroup
	for i := range o.conns {
		wg.Add(1)
		go func(num int) {
			o.conns[num].Fetch()
			wg.Done()
		}(i)
	}
	wg.Wait()
	processConns(o, w)
}

// processConns distributes the output handler to respective function based on type
func processConns(o *Orchestra, w http.ResponseWriter) error {
	var err error
	switch o.responseType {
	case typeDelimiter:
		err = outputDelimiter(o, w)
		break
	case typeJson:
		w.Header().Set("Content-type", "application/json")
		err = outputJson(o, w)
		break
	default:
		return errInvalidResponseType
	}
	return err
}

// outputJson extracts all responses from o and json encode into w
func outputJson(o *Orchestra, w io.Writer) error {
	resps := make([]RespOutput, len(o.conns))
	for i := range resps {
		if o.conns[i].err != nil {
			resps[i] = RespOutput{Id: o.conns[i].id, Error: o.conns[i].err.Error()}
			continue
		}
		resps[i] = o.conns[i].Response.Output()
	}
	encoder := json.NewEncoder(w)
	return encoder.Encode(resps)
}

// outputDelimiter extracts all responses from o and writes to w. It separates each response with
// the specified delimeter
func outputDelimiter(o *Orchestra, w io.Writer) error {
	for i := range o.conns {
		var r RespOutput
		var err error
		if o.conns[i].err != nil {
			r = RespOutput{Id: o.conns[i].id, Error: o.conns[i].err.Error()}
		} else {
			r = o.conns[i].Response.Output()
		}
		_, err = w.Write(r.Bytes())
		if err != nil {
			log.Println(err)
			return err
		}
		if i < len(o.conns)-1 {
			_, err = w.Write([]byte(o.delimiter))
			if err != nil {
				log.Println(err)
				return err
			}
		}
	}
	return nil
}

// Conn is the individual connection that is handled by Orchestra
// TODO allow other request methods apart from GET
type Conn struct {
	*http.Client
	// Id is the unique identification of the Request
	id string
	// Url is the target Url request will be sent to
	url string
	// Header is required http headers
	Header http.Header
	// Form parameters
	Params map[string]string
	// A wrapper for http.Response with the ability to read body into []byte
	Response *Response
	err      error
}

// NewConn creates a new Connection. It initiates with a ConnRequest for Id and Url
func NewConn(r ConnRequest) *Conn {
	return &Conn{
		&http.Client{},
		r.id,
		r.url,
		make(http.Header),
		make(map[string]string),
		nil,
		nil,
	}
}

// Fetch sends GET request to Conn's url and stores Response
func (c *Conn) Fetch() error {
	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		log.Println(err)
		c.err = err
		return err
	}
	req.Header = c.Header
	values := req.URL.Query()
	for m, v := range c.Params {
		values.Add(m, v)
	}
	req.URL.RawQuery = values.Encode()
	response, err := c.Do(req)
	if err != nil {
		log.Println(err)
		c.err = err
		return err
	}
	c.Response = &Response{
		response,
		nil,
		c.id,
	}
	return nil
}

// Response is a wrapper around http.Response.
type Response struct {
	*http.Response
	extract []byte
	id      string
}

// ReadAll read all bytes in Response and stores it in Extract
func (r *Response) ReadAll() ([]byte, error) {
	if r.extract != nil {
		return r.extract, nil
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	r.Body.Close()
	r.extract = b
	return b, nil
}

// Output returns a Json marshal friendly strcut of Response for output
func (r *Response) Output() RespOutput {
	if r.extract == nil {
		_, err := r.ReadAll()
		if err != nil {
			log.Println(err)
			return RespOutput{Error: err.Error()}
		}
	}
	return RespOutput{
		r.id,
		r.StatusCode,
		r.Status,
		string(r.extract),
		"",
	}
}

// RespOutput is an output struct suited for Json marshal
type RespOutput struct {
	Id         string `json:"id"`
	StatusCode int    `json:"status_code,omitempty"`
	Status     string `json:"status,omitempty"`
	Body       string `json:"body,omitempty"`
	Error      string `json:"error,omitempty"`
}

// String returns the string representation to be used in TypeDelimeter response type.
func (r *RespOutput) String() string {
	return fmt.Sprintf("Id: %v,Status: %v\n%v\n", r.Id, r.Status, r.Body)
}

// Bytes return the bytes representation of String()
func (r *RespOutput) Bytes() []byte {
	return []byte(r.String())
}
