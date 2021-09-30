package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/obito/cclient"
	tls "github.com/refraction-networking/utls"
)

func main() {
	fmt.Print("Starting zTLS...")
	http.HandleFunc("/", handleReq)
	err := http.ListenAndServe(":3008", nil)
	if err != nil {
		log.Fatalf("Error starting the app; %v", err)
	}
}

func handleReq(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	pageUrl := r.Header.Get("zTls-url")
	if pageUrl == "" {
		http.Error(w, "header zTls-url is empty", http.StatusBadRequest)
		return
	}
	r.Header.Del("zTls-url")

	// proxy and user-agent
	userAgent := r.Header.Get("User-Agent")
	if userAgent == "" {
		http.Error(w, "header User-Agent is empty", http.StatusBadRequest)
		return
	}
	r.Header.Del("User-Agent")

	proxy := r.Header.Get("zTls-proxy")
	r.Header.Del("zTls-proxy")

	/*
		// redirect and timeout
		redirect := r.Header.Get("zTls-allowRedirect")
		if redirect == "" {
			http.Error(w, "header zTls-allowRedirect is empty", http.StatusBadRequest)
			return
		}

		allowRedirect, err := strconv.ParseBool(redirect)
		if err != nil {
			http.Error(w, "couldn't parse zTls-allowRedirect header, must be true or false", http.StatusBadRequest)
			return
		}
		r.Header.Del("zTls-allowRedirect")

	*/

	// manage tls fingerprint
	userAgentLowerCase := strings.ToLower(userAgent)

	var tlsHello tls.ClientHelloID

	switch userAgentLowerCase {
	case "firefox":
		tlsHello = tls.HelloFirefox_Auto
	case "chrome":
		tlsHello = tls.HelloChrome_Auto
	default:
		tlsHello = tls.HelloIOS_Auto
	}

	// create new tls client
	client, err := cclient.NewClient(tlsHello, true, proxy)

	if err != nil {
		http.Error(w, fmt.Sprintf("error while creating new client: %v", err), http.StatusBadRequest)
		return
	}

	// forward query params
	var addedQuery string
	for k, v := range r.URL.Query() {
		addedQuery += "&" + k + "=" + v[0]
	}

	endpoint := pageUrl + "?" + addedQuery
	if strings.Contains(pageUrl, "?") {
		endpoint = pageUrl + addedQuery
	} else if addedQuery != "" {
		endpoint = pageUrl + "?" + addedQuery[1:]
	}
	req, err := http.NewRequest(r.Method, ""+endpoint, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("error while creating new request: %v", err), http.StatusBadRequest)
		return
	}

	//set our Host header
	u, err := url.Parse(endpoint)
	if err != nil {
		http.Error(w, fmt.Sprintf("error while parsing endpoint: %v", err), http.StatusBadRequest)
		return
	}

	// append every header not custom to the req
	for k := range r.Header {
		if k != "Content-Length" {
			fmt.Println("adding header: " + r.Header.Get(k))
			v := r.Header.Get(k)
			req.Header.Set(k, v)
		}
	}

	// do req
	req.Header.Set("Host", u.Host)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	// forward response's headers and status code
	for k, v := range resp.Header {
		if k != "Content-Length" {
			w.Header().Set(k, v[0])
		}
	}
	w.WriteHeader(resp.StatusCode)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading body: %v", err), http.StatusBadRequest)
	}

	// send raw body
	if _, err := fmt.Fprint(w, body); err != nil {
		http.Error(w, fmt.Sprintf("Error writing body: %v", err), http.StatusBadRequest)
	}
	resp.Body.Close()
}

func readAndClose(r io.ReadCloser) ([]byte, error) {
	readBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return readBytes, r.Close()
}
