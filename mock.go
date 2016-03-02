package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type HandlerFunc func(http.ResponseWriter, *http.Request)

type Response struct {
	Status int
	Body   string
}

type Params map[string]string

func (p Params) Matches(query url.Values) bool {
	for k, v := range p {
		if query.Get(k) != v {
			return false
		}
	}

	for k := range query {
		if query.Get(k) != p[k] {
			return false
		}
	}

	return true
}

type Request struct {
	Url      string
	Method   string
	Params   Params
	Data     string
	Response Response
	Called   bool
}

func (r *Request) Matches(req *http.Request) bool {
	return req.Method == r.Method && r.Params.Matches(req.URL.Query())
}

type MocksVerification struct {
	AllCalled bool `json:"allCalled"`
}

type Mocks map[string]*Request

func (m *Mocks) AddFromRequest(req *http.Request) error {
	dec := json.NewDecoder(req.Body)
	var r Request
	if err := dec.Decode(&r); err == nil || err == io.EOF {
		(*m)[r.Url] = &r
	} else {
		return err
	}
	return nil
}

func (m *Mocks) Reset() error {
	for k := range *m {
		delete(*m, k)
	}
	return nil
}

func (m *Mocks) ResponseFor(r *http.Request) (*Request, Response, bool) {
	req, found := (*m)[r.URL.Path]
	if found {
		if req.Matches(r) {
			return req, req.Response, true
		} else {
			log.Printf("Hit %s but didn't match. Expected params: %s. Given params: %s.\n", r.URL.Path, req.Params, r.URL.Query())
			return nil, Response{}, false
		}
	}
	return nil, Response{}, false
}

func (m *Mocks) Verification() (MocksVerification, error) {
	for _, r := range *m {
		if !r.Called {
			return MocksVerification{AllCalled: false}, nil
		}
	}
	return MocksVerification{AllCalled: true}, nil
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return true, err
}

func mocksHandler(mocks Mocks) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			if verification, err := mocks.Verification(); err != nil {
				log.Println(err)
			} else {
				enc := json.NewEncoder(w)
				enc.Encode(verification)
			}
		case "POST":
			if err := mocks.AddFromRequest(r); err != nil {
				log.Println(err)
			}
		case "DELETE":
			if err := mocks.Reset(); err != nil {
				log.Println(err)
			}
		}
	}
}

func mocksReporter(mocks Mocks) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, fmt.Sprintf("%s", mocks))
	}
}

func staticFile(w http.ResponseWriter, path string) {
	var contentType string
	if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	}
	w.Header().Add("Content-Type", contentType)

	f, err := os.Open(path)
	if err != nil {
		log.Println(err)
	} else {
		io.Copy(w, f)
	}
}

func any(mocks Mocks, staticRoot string) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var staticPath string
		if r.URL.Path == "/" {
			staticPath = staticRoot + "/index.html"
		} else {
			staticPath = staticRoot + r.URL.Path
		}

		if found, err := exists(staticPath); err != nil {
			log.Println(err)
			return
		} else if found {
			staticFile(w, staticPath)
		} else if request, response, found := mocks.ResponseFor(r); found {
			request.Called = true
			if response.Status != 0 {
				w.WriteHeader(response.Status)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			io.WriteString(w, response.Body)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func main() {
	port := flag.String("port", "8080", "TCP port to bind to")
	staticRoot := flag.String("root", ".", "Root directory")

	flag.Parse()

	mocks := Mocks{}

	http.HandleFunc("/mocks", mocksHandler(mocks))
	http.HandleFunc("/meta", mocksReporter(mocks))
	http.HandleFunc("/", any(mocks, *staticRoot))

	log.Println("Serving on port", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
