package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

var (
	appservers   = []string{}
	currentIndex = 0
	client       = http.Client{Transport: &transport, Timeout: 10 * time.Second}
	reverseProxy = []*httputil.ReverseProxy{}
	httpScheme   = "http"
	pingEndpoint = "/health/ping/"
)

func processRequests() {
	for {
		select {
		case request := <-requestCh:
			println("received a request...")
			if len(appservers) == 0 {
				request.w.WriteHeader(http.StatusInternalServerError)
				request.w.Write([]byte("No appservers were found."))
				request.doneCh <- struct{}{}
				continue
			}

			currentIndex++
			if currentIndex == len(appservers) {
				currentIndex = 0
			}
			// Get the next backend service information for load balancing the request.
			// We are following round-robin approach to distribute the requests.
			appserverURL := appservers[currentIndex]
			appserverProxy := reverseProxy[currentIndex]
			// Calling processingRequest to forward the received request to backend services.
			go processingRequest(appserverURL, appserverProxy, request)

		case host := <-registerCh:
			println("registering new backend server - " + host)
			isFound := false
			for _, h := range appservers {
				if host == h {
					isFound = true
					break
				}
			}
			if !isFound {
				appservers = append(appservers, host)
				serveUrl, e := url.Parse("http://" + host)

				if e != nil {
					fmt.Println("error ", e.Error())
				}
				// Creating a reverseProxy object for a new register backend server.
				tempProxy := httputil.NewSingleHostReverseProxy(serveUrl)
				tempProxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, e error) {
					fmt.Println("Error occured in registering the server - ")
					log.Printf(e.Error())
					writer.WriteHeader(http.StatusBadRequest)
					writer.Write([]byte("failed request"))
				}
				reverseProxy = append(reverseProxy, tempProxy)
				log.Println("Active Backend servers - ", strings.Join(appservers, ", "))
			}

		case host := <-unregisterCh:
			println("unregistering " + host)
			for i := len(appservers) - 1; i >= 0; i-- {
				if appservers[i] == host {
					// unregister a backend server for being inactive.
					//using the spread operator to spread out the remaining elements
					appservers = append(appservers[:i], appservers[i+1:]...)
					reverseProxy = append(reverseProxy[:i], reverseProxy[i+1:]...)
					log.Println("removed from active backend server - ", strings.Join(appservers, ", "))

				}
			}
		case <-heartbeat:
			log.Println("heartbeat check for below servers ...")
			log.Println(strings.Join(appservers, ", "))

			servers := appservers[:] //copy the slice
			go func(servers []string) {
				for _, host := range servers {
					resp, err := http.Get(httpScheme + "://" + host + pingEndpoint)
					// If response status from backend server is other than HTTP_204_NO_CONTENT,
					// then unregister that server for being inactive.
					if err != nil || resp.StatusCode != 204 {
						unregisterCh <- host
					}
				}
			}(servers)
		}
	}
}

func processingRequest(appserverURL string, appserverProxy *httputil.ReverseProxy, request *webRequest) {
	fmt.Println("Forwarding request to url - " + appserverURL)
	appserverProxy.ServeHTTP(request.w, request.r)
	request.doneCh <- struct{}{}
	return
}
