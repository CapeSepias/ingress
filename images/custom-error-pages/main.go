/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// FormatHeader name of the header used to extract the format
	FormatHeader = "X-Format"

	// CodeHeader name of the header used as source of the HTTP statu code to return
	CodeHeader = "X-Code"

	// ContentType name of the header that defines the format of the reply
	ContentType = "Content-Type"

	ServiceName = "X-Service-Name"
)

func main() {
	path := "/www"
	if os.Getenv("ERRPATH") != "" {
		path = os.Getenv("ERRPATH")
	}

	http.HandleFunc("/", errorHandler(path))

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(fmt.Sprintf(":8080"), nil)
}

func errorHandler(path string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path = "/www"
		start := time.Now()
		ext := "html"

		format := r.Header.Get(FormatHeader)
		if format == "" {
			format = "text/html"
			log.Printf("format not specified. Using %v", format)
		}

		cext, err := mime.ExtensionsByType(format)
		if err != nil {
			//log.Printf("unexpected error reading media type extension: %v. Using %v\n", err, ext)
			format = "text/html"
		} else if len(cext) == 0 {
			//log.Printf("couldn't get media type extension. Using %v\n", ext)
		} else {
			ext = cext[0]
		}
		service:= r.Header.Get(ServiceName)

		w.Header().Set(ContentType, format)

		errCode := r.Header.Get(CodeHeader)
		code, err := strconv.Atoi(errCode)
		if err != nil {
			//fmt.Println("err: " + err.Error())
			code = 404
			path = "/www2"
		}
		if code == 503 && !strings.Contains(service, "cms") {
			//fmt.Println("service: " + r.Header.Get(ServiceName))
			//fmt.Println("code: " + r.Header.Get(CodeHeader))
			path = "/www2"
		}

		if code == 503 && strings.Contains(r.Header.Get(ServiceName), "cms") {
			ext = "html"
		}
		w.WriteHeader(code)

		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		file := fmt.Sprintf("%v/%v%v", path, code, ext)
		f, err := os.Open(file)
		if err != nil {
			log.Printf("unexpected error opening file: %v\n", err)
			scode := strconv.Itoa(code)
			file := fmt.Sprintf("%v/%cxx%v", path, scode[0], ext)
			f, err := os.Open(file)
			if err != nil {
				log.Printf("unexpected error opening file: %v\n", err)
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			//log.Printf("serving custom error response for code %v and format %v from file %v\n", code, format, file)
			io.Copy(w, f)
			return
		}
		defer f.Close()
		//log.Printf("serving custom error response for code %v and format %v from file %v\n", code, format, file)
		io.Copy(w, f)

		duration := time.Now().Sub(start).Seconds()

		proto := strconv.Itoa(r.ProtoMajor)
		proto = fmt.Sprintf("%s.%s", proto, strconv.Itoa(r.ProtoMinor))

		requestCount.WithLabelValues(proto).Inc()
		requestDuration.WithLabelValues(proto).Observe(duration)
	}
}
