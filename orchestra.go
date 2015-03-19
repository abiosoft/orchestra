// Orchestration Layer
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	typeJson = iota
	typeDelimiter

	defaultTimeout   = 10 * time.Second
	defaultDelimeter = "---XXX---\n"
)

var (
	errInvalidResponseType = errors.New("Invalid Response Type specified. Must be one of TypeJson, TypeDelimiter")
	errTimeout             = errors.New("Timeout exceeded! Connection terminated.")
)

// Orchestra is the high level representation of the Orchestration Layer
type Orchestra struct {
	conns        []*Conn
	responseType uint8
	cLock        *sync.Mutex
	delimiter    string
	timeout      time.Duration
}

// ConnRequest is the representation of the Connection Request used to initialize the Orchestra
type ConnRequest struct {
	id  string // identification
	url string // target url
}

// NewOrchestra creates a new orchestra. It initializes with ConnRequest(s)
func NewOrchestra(requests ...ConnRequest) *Orchestra {
	conns := make([]*Conn, len(requests))
	for i := range requests {
		conns[i] = NewConn(requests[i])
		conns[i].Timeout = defaultTimeout
	}
	return &Orchestra{
		conns,
		typeJson,
		&sync.Mutex{},
		defaultDelimeter,
		defaultTimeout,
	}
}

// Add adds a new Connection Request to the Orchestrac
func (o *Orchestra) Add(r ConnRequest) {
	o.cLock.Lock()
	defer o.cLock.Unlock()
	conn := NewConn(r)
	conn.Timeout = o.timeout
	o.conns = append(o.conns, conn)
}

// SetTimeout sets the timeout for http.Client used for each request
func (o *Orchestra) SetTimeout(t time.Duration) {
	o.timeout = t
	for i := range o.conns {
		o.conns[i].Timeout = o.timeout
	}
}

// SetDelimiter instructs the Orchestra to use separate plain text outputs with delimeter instead of Json
func (o *Orchestra) SetDelimiter(d string) {
	o.delimiter = d
	if !strings.HasSuffix(d, "\n") {
		o.delimiter += "\n"
	}
	o.responseType = typeDelimiter
}

// UseDelimeter instructs the Orchestra to use Json for output
func (o *Orchestra) UseDelimeter() {
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
	wg.Add(len(o.conns))
	for i := range o.conns {
		go fetchConns(o.conns[i], &wg)
	}
	wg.Wait()
	processConns(o, w)
}

func fetchConns(conn *Conn, wg *sync.WaitGroup) {
	conn.Fetch()
	wg.Done()
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
	resps := make([]respOutput, len(o.conns))
	for i := range resps {
		if o.conns[i].err != nil {
			resps[i] = respOutput{Id: o.conns[i].id, Error: o.conns[i].err.Error()}
			continue
		}
		resps[i] = o.conns[i].Response.output()
	}
	encoder := json.NewEncoder(w)
	return encoder.Encode(resps)
}

// outputDelimiter extracts all responses from o and writes to w. It separates each response with
// the specified delimeter
func outputDelimiter(o *Orchestra, w io.Writer) error {
	for i := range o.conns {
		var r respOutput
		var err error
		if o.conns[i].err != nil {
			r = respOutput{Id: o.conns[i].id, Error: o.conns[i].err.Error()}
		} else {
			r = o.conns[i].Response.output()
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
	id       string            // identification
	url      string            // target url
	Header   http.Header       // http headers
	Params   map[string]string // form parameters
	Response *Response         // request response
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
	// pass headers
	req.Header = c.Header

	// workaround for query params
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
func (r *Response) output() respOutput {
	if r.extract == nil {
		_, err := r.ReadAll()
		if err != nil {
			// check for timeout error
			if te, ok := err.(net.Error); ok && te.Timeout() {
				err = errTimeout
			}
			return respOutput{Error: err.Error()}
		}
	}
	return respOutput{
		r.id,
		r.StatusCode,
		r.Status,
		string(r.extract),
		"",
	}
}

// RespOutput is an output struct suited for Json marshal
type respOutput struct {
	Id         string `json:"id"`
	StatusCode int    `json:"status_code,omitempty"`
	Status     string `json:"status,omitempty"`
	Body       string `json:"body,omitempty"`
	Error      string `json:"error,omitempty"`
}

// String returns the string representation to be used in TypeDelimeter response type.
func (r *respOutput) String() string {
	if r.Error != "" {
		return fmt.Sprintf("Id: %v, Status: %v\n%v\n", r.Id, "error", r.Error)
	}
	return fmt.Sprintf("Id: %v, Status: %v\n%v\n", r.Id, r.Status, r.Body)
}

// Bytes return the bytes representation of String()
func (r *respOutput) Bytes() []byte {
	return []byte(r.String())
}
