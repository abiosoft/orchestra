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
	"time"
)

const (
	typeJson = iota
	typeDelimiter

	defaultTimeout   = 10 * time.Second
	defaultDelimiter = "\n---XXX---\n"
)

var (
	errInvalidResponseType = errors.New("Invalid Response Type specified. Must be one of typeJson, typeDelimiter")
	errTimeout             = errors.New("Timeout exceeded! Connection terminated.")
)

// Orchestra is the high level representation of the Orchestration Layer.
type Orchestra struct {
	conns        []*Conn
	responseType uint8
	cLock        *sync.Mutex
	delimiter    string
	timeout      time.Duration
}

// ConnRequest is the representation of the Connection Request used to initialize the Orchestra.
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
		defaultDelimiter,
		defaultTimeout,
	}
}

// Add adds a new Connection Request to the Orchestra.
func (o *Orchestra) Add(r ConnRequest) {
	o.cLock.Lock()
	defer o.cLock.Unlock()
	conn := NewConn(r)
	conn.Timeout = o.timeout
	o.conns = append(o.conns, conn)
}

// SetTimeout sets the timeout for http.Client used for each request.
func (o *Orchestra) SetTimeout(t time.Duration) {
	o.timeout = t
	for i := range o.conns {
		o.conns[i].Timeout = o.timeout
	}
}

// SetDelimiter instructs the Orchestra to use separate plain text outputs with delimeter instead of json.
func (o *Orchestra) SetDelimiter(d string) {
	o.delimiter = "\n" + d
	if !strings.HasSuffix(d, "\n") {
		o.delimiter += "\n"
	}
	o.responseType = typeDelimiter
}

// UseDelimeter instructs the Orchestra to use Json for output.
func (o *Orchestra) UseDelimeter() {
	o.responseType = typeDelimiter
}

// UseJson instructs the Orchestra to use Json for output.
func (o *Orchestra) UseJson() {
	o.responseType = typeJson
}

// Process processes all connection requests and send them concurrently
// When done, it outputs to w.
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

// processConns distributes the output handler to respective function based on type.
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

// outputJson extracts all responses from o and json encode into w.
func outputJson(o *Orchestra, w io.Writer) error {
	resps := make([]*Response, len(o.conns))
	for i := range resps {
		resps[i] = o.conns[i].Response
	}
	encoder := json.NewEncoder(w)
	return encoder.Encode(resps)
}

// outputDelimiter extracts all responses from o and writes to w. It separates each response with
// the specified delimiter.
func outputDelimiter(o *Orchestra, w io.Writer) error {
	for i := range o.conns {
		_, err := o.conns[i].Response.writeTo(w)
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

// Conn is the individual connection that is handled by Orchestra.
// TODO allow other request methods apart from GET
type Conn struct {
	*http.Client
	id       string            // identification
	url      string            // target url
	Header   http.Header       // http headers
	Params   map[string]string // form parameters
	Response *Response         // request response
}

// NewConn creates a new Connection. It initiates with a ConnRequest for Id and Url.
func NewConn(r ConnRequest) *Conn {
	return &Conn{
		&http.Client{},
		r.id,
		r.url,
		make(http.Header),
		make(map[string]string),
		nil,
	}
}

// Fetch sends GET request to Conn's url and stores Response.
func (c *Conn) Fetch() error {
	now := time.Now()
	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		log.Println(err)
		c.Response = &Response{nil, c.id, err, 0}
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
		c.Response = &Response{nil, c.id, err, 0}
		return err
	}
	c.Response = &Response{
		response,
		c.id,
		nil,
		time.Since(now),
	}
	return nil
}

// Response is a wrapper around http.Response.
type Response struct {
	*http.Response
	id       string
	err      error
	duration time.Duration
}

// Output returns a Json marshal friendly struct of Response for output.
func (r *Response) output() respOutput {
	if r.err != nil {
		return respOutput{
			Id:    r.id,
			Error: r.err.Error(),
		}
	}
	return respOutput{
		Id:         r.id,
		StatusCode: r.StatusCode,
		Status:     r.Status,
		Duration:   r.durationStr(),
	}
}

// Read reads []byte of maximum of len(p) into p. It returns the number
// of bytes read and an error if any.
func (r *Response) Read(p []byte) (int, error) {
	return r.Body.Read(p)
}

// ReadAll reads all bytes from Response. It returns the bytes and an error if any.
func (r *Response) ReadAll() ([]byte, error) {
	return ioutil.ReadAll(r)
}

// writeTo writes Response of delimiter type into w.
func (resp *Response) writeTo(w io.Writer) (int, error) {
	r := resp.output()
	if r.Error != "" {
		return resp.writeErrTo(w, r.Error)
	}
	_, err := w.Write([]byte(fmt.Sprintf("Id: %v, Status: %v, Duration: %v\n", r.Id, r.Status, resp.durationStr())))
	if err != nil {
		return resp.writeErrTo(w, err.Error())
	}
	nn, err := io.Copy(w, resp.Body)
	return int(nn), err
}

// Similar to writeTo but writes error response
func (r *Response) writeErrTo(w io.Writer, err string) (int, error) {
	return w.Write([]byte(fmt.Sprintf("Id: %v, Status: %v\n%v\n", r.id, "error", err)))
}

// MarshalJSON defines how Response is marshaled for JSON encoding.
func (resp *Response) MarshalJSON() ([]byte, error) {
	r := resp.output()
	if r.Error != "" {
		return resp.marshalErr(resp.id, r.Error)
	}
	body, err := ioutil.ReadAll(resp)
	if err != nil {
		return resp.marshalErr(resp.id, err.Error())
	}
	r.Body = string(body)
	b, err := json.Marshal(r)
	if err != nil {
		return resp.marshalErr(resp.id, err.Error())
	}
	return b, nil
}

func (resp *Response) marshalErr(id, err string) ([]byte, error) {
	return json.Marshal(respOutput{Id: id, Error: err})
}

func (r *Response) durationStr() string {
	return fmt.Sprintf("%vms", int64(r.duration)/1e6)
}

// RespOutput is an output struct suited for Json marshal
type respOutput struct {
	Id         string `json:"id"`
	StatusCode int    `json:"status_code,omitempty"`
	Status     string `json:"status,omitempty"`
	Duration   string `json:"duration,omitempty"`
	Body       string `json:"body,omitempty"`
	Error      string `json:"error,omitempty"`
}
