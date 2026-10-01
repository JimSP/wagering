// Command healthcheck works in the distroless image without a shell or curl.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	endpoint := "http://127.0.0.1:8080/health/ready"
	if len(os.Args) == 2 {
		endpoint = os.Args[1]
	} else if len(os.Args) != 1 {
		os.Exit(2)
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(endpoint)
	if err != nil {
		fmt.Fprintln(os.Stderr, "readiness request failed")
		os.Exit(1)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "readiness status:", res.StatusCode)
		os.Exit(1)
	}
}
