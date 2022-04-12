package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type webRequest struct {
	r      *http.Request
	w      http.ResponseWriter
	doneCh chan struct{}
}

/* To know more about the channels - https://gobyexample.com/channels
   - https://go.dev/ref/spec#Making_slices_maps_and_channels
*/
var (
	requestCh    = make(chan *webRequest)
	registerCh   = make(chan string)
	unregisterCh = make(chan string)
	heartbeat    = time.Tick(5 * time.Second)
)

// Enable this for TLS/SSL communication
var (
	transport = http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
)

/*
var (
	transport = http.Transport{
		MaxIdleConns:        100,
		MaxConnsPerHost:     100,
		MaxIdleConnsPerHost: 100,
	}
)
*/
// Configure HTTPClient with above-mentioned (transport) properties.
func init() {
	http.DefaultClient = &http.Client{Transport: &transport}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		doneCh := make(chan struct{})

		request := webRequest{
			r:      r,
			w:      w,
			doneCh: doneCh,
		}

		requestCh <- &request

		<-doneCh
	})
	go processRequests()
	// HTTP endpoint for receiving the backend requests.
	go http.ListenAndServeTLS(":8000", "cert/server.crt", "cert/server.key", nil)
	// HTTP endpoint for receiving a backend server registration requests.
	go http.ListenAndServe(":8002", new(appserverHandler))
	println("Catamaran in action ...")
	fmt.Scanln()
}

type appserverHandler struct{}

func (h *appserverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip := ""

	if strings.HasPrefix(r.RemoteAddr, "[::1]") {
		ip = "localhost"
	} else {
		ip = strings.Split(r.RemoteAddr, ":")[0]
	}

	port := r.URL.Query().Get("port")
	switch r.URL.Path {
	case "/register":
		registerCh <- ip + ":" + port

	case "/unregister":
		unregisterCh <- ip + ":" + port
	}
}
