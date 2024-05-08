/*
An example of how to use go-icap.

Run this program and Squid on the same machine.
Put the following lines in squid.conf:

icap_enable on
icap_service service_req reqmod_precache icap://127.0.0.1:11344/golang
adaptation_access service_req allow all

(The ICAP server needs to be started before Squid is.)

Set your browser to use the Squid proxy.

Try browsing to http://gateway/ and http://java.com/
*/
package main

import (
	"fmt"
	"net/http"
	"os"
	"santoshsahu/ipcap/pkg/icap"
)

var ISTag = "\"GOLANG\""

func main() {
	// Set the files to be made available under http://gateway/
	http.Handle("/", http.FileServer(http.Dir(os.Getenv("HOME")+"/Sites")))

	icap.HandleFunc("/ext_cap/v1/icap/req", toGolang)
	icap.HandleFunc("/ext_cap/v1/icap/res", toGolang)
	//icap.ListenAndServe(":80", icap.HandlerFunc(toGolang))
	icap.ListenAndServeSSL(":443","cert.crt", "key.pem", icap.HandlerFunc(toGolang))
}

func toGolang(w icap.ResponseWriter, req *icap.Request) {
	switch req.Method {
	case "OPTIONS":
		w.WriteHeader(200, nil, false)
	case "REQMOD":
		// Return the request unmodified.
		fmt.Println("txid ", req.Header.Get("txid"))
		fmt.Println("Orig Request URL: ", req.Request.URL)
		fmt.Println("Orig Request Headers: ", req.Request.Header)
		buf := make([]byte, req.Request.ContentLength)
		req.Request.Body.Read(buf)
		fmt.Println("Orig Request body: ", string(buf))
		w.WriteHeader(204, nil, false)
	case "RESPMOD":
		// Return the request unmodified.
		fmt.Println("txid ", req.Header.Get("txid"))
		fmt.Println("Orig Response Code: ", req.Response.Status)
		fmt.Println("Orig Response Headers: ", req.Response.Header)
		buf := make([]byte, req.Response.ContentLength)
		req.Response.Body.Read(buf)
		fmt.Println("Orig Response body: ", string(buf))
		w.WriteHeader(204, nil, false)

	default:
		w.WriteHeader(405, nil, false)
		fmt.Println("Invalid request method")
	}
}
