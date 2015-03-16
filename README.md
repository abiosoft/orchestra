## Orchestra
#### Minimalistic Orchestration Layer
[![Build Status](https://drone.io/github.com/abiosoft/orchestra/status.png)](https://drone.io/github.com/abiosoft/orchestra/latest)

Runs API calls concurrently

### Usage
Parameters are specified via comma separated key value column pairs through the `requests` `GET` parameter.
```
http://127.0.0.1:8080?requests=identifier1:http://url1.xyz,identifier2:http://url2.xyz
```
What if the `value` url has its own `GET` parameters? Url encode the entire query string starting from `?`.

### Response
Response comes in 2 formats
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
  }
]
```
#### 2. Delimeter Separated
**under development**

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
* Better CLI support.
* Request hierarchy and dependency. Ability to call one request before the other and possibly extract values from response of one request to use in another request.
* Robust test cases and high code coverage.
* Project page.
