package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type Endpoint struct {
	Name    string            `yaml:"name"`
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
}

type DomainStats struct {
	Success uint32
	Total   uint32
}

var stats = make(map[string]*DomainStats)

func checkHealth(endpoint Endpoint, wg *sync.WaitGroup) {
	defer wg.Done()
	var client = &http.Client{
		Timeout: 500 * time.Millisecond,
	}

	var reqBody io.Reader
	if endpoint.Body != "" && endpoint.Method != "GET" {
		reqBody = bytes.NewBuffer([]byte(endpoint.Body))
	}

	req, err := http.NewRequest(endpoint.Method, endpoint.URL, reqBody)
	if err != nil {
		log.Println("Error creating request:", err)
		return
	}

	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	domain := extractDomain(endpoint.URL)

	atomic.AddUint32(&stats[domain].Total, 1)
	if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		atomic.AddUint32(&stats[domain].Success, 1)
	}
}

func extractDomain(url string) string {
	urlSplit := strings.Split(url, "//")
	domain := strings.Split(urlSplit[len(urlSplit)-1], "/")[0]
	return domain
}

func monitorEndpoints(endpoints []Endpoint, ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for _, endpoint := range endpoints {
		domain := extractDomain(endpoint.URL)
		if stats[domain] == nil {
			stats[domain] = &DomainStats{}
		}
	}

out:
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Shutting down monitor endpoints...")
			break out
		default:
			for _, endpoint := range endpoints {
				wg.Add(1)
				go checkHealth(endpoint, wg)
			}
			wg.Add(1)
			go logResults(wg)
			time.Sleep(3 * time.Second)
		}
	}
}

func logResults(wg *sync.WaitGroup) {
	defer wg.Done()
	for domain, stat := range stats {
		total := atomic.LoadUint32(&stat.Total)
		success := atomic.LoadUint32(&stat.Success)

		if total == uint32(0) {
			fmt.Printf("%s has not had any requests return yet...\n", domain)
			continue
		}
		percentage := int(math.Round(100 * float64(success) / float64(total)))
		fmt.Printf("%s has %d%% availability\n", domain, percentage)
	}
}

var wg sync.WaitGroup

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <config_file>")
	}

	filePath := os.Args[1]
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal("Error reading file:", err)
	}

	var endpoints []Endpoint
	if err := yaml.Unmarshal(data, &endpoints); err != nil {
		log.Fatal("Error parsing YAML:", err)
	}

	for i := range endpoints {
		if len(endpoints[i].Method) == 0 {
			endpoints[i].Method = "GET"
		}
	}

	wg.Add(1)
	go monitorEndpoints(endpoints, ctx, &wg)

	sig := <-ctx.Done()
	fmt.Printf("\nGot shutdown signal: %v\n", sig)
	fmt.Println("Shutting down....")

	wg.Wait()
	for domain, stat := range stats {
		total := atomic.LoadUint32(&stat.Total)
		success := atomic.LoadUint32(&stat.Success)
		fmt.Printf("%s had %d total requests and %d total successes\n", domain, total, success)
	}
	fmt.Println("Gracefully shutdown")

}
