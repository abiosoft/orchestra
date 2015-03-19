## Orchestra
#### Minimalistic Orchestration Layer
[![Build Status](https://drone.io/github.com/abiosoft/orchestra/status.png)](https://drone.io/github.com/abiosoft/orchestra/latest)

Runs API calls concurrently

### Usage
#### APIs
APIs are set via comma separated key value column pairs through the `requests` query parameter.
```
http://127.0.0.1:8080?requests=identifier1:http://url1.xyz,identifier2:http://url2.xyz
```
What if the `value` url has its own query parameters? Url encode the entire query string starting from `?`.
### Parameters

| Parameter | Description | Default | Usage |
| --------- | ----------- | ------- | ----- |
| requests | Key value column pairs | | `requests=id1:http://url1.xy,id2:http://url2.xy` |
| timeout | Timeout in milliseconds | 10000 | `timeout=500`
| type | Response Type `[json, delimiter]`| json | `type=json`
| delimiter| Delimiter to use. (requires `type=delimiter`) | ---XXX--- | `type=delimeter&delimeter=---XXX---`

Sample request with all parameters
```
http://127.0.0.1:8080?requests=id1:http://url1.xy,id2:http://url2.xy&timeout=500&type=delimiter&delimiter=---XXX---
```

### Response
Response comes in 2 format specified by `type` parameter.
#### 1. Json
```
[
  {
    "id": "identifier1",
    "status_code": 200,
    "status": "200 OK",
    "body": "<html><body><h1>It works!</h1></body></html>\n"
  },
  {
    "id": "identifier2",
    "status_code": 400,
    "status": "400 Bad Request",
    "body": "Bad Request: required parameter 'requests' missing."
  },
  {
    "id": "identifier3",
    "error": "Request timed out"
  }
]
```
#### 2. Delimiter Separated
```
Id: identifier1, Status: 200 OK
<html><body><h1>It works!</h1></body></html>
---XXX---
Id: identifier2, Status: 400 Bad Request
Bad Request: required parameter 'requests' missing.
---XXX---
Id: identifier3, Status: error
Request timed out
```

### Server
Defaults to Port `8080` but can be overidden using the first command line argument
```
$ orchestra 8080
Orchestra listening on port 8080
```

### State
Orchestra is still in very early stage and active development

### Installation
```
$ go get github.com/abiosoft/orchestra
```
Go is a prerequisite, [install it here](https://golang.org/doc/install) if you do not have it installed.

### Planned
* Support POST and other request types.
* Configuration files.
* Web Interface
* Better CLI support.
* Request hierarchy and dependency. Ability to call one request before the other and possibly extract values from response of one request to use in another request.
* Robust test cases and high code coverage.
* Project page.
