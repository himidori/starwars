package starwars

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	bufferSize    = 10
	bufferTimeout = time.Millisecond * 250
)

type Response struct {
	Next    string    `json:"next"`
	Results []*Person `json:"results"`
}

type Person struct {
	Name string `json:"name"`
}

type Fetcher struct {
	sync.Mutex
	http    *http.Client
	buffer  []string
	ch      chan string
	ctx     context.Context
	cancelf func()
	done    chan struct{}
}

func NewFetcher() *Fetcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Fetcher{
		http:    &http.Client{Timeout: time.Second * 10},
		buffer:  make([]string, 0, bufferSize),
		ch:      make(chan string),
		ctx:     ctx,
		cancelf: cancel,
		done:    make(chan struct{}),
	}
}

func (f *Fetcher) Start() {
	go f.fetch()
	go f.process()
}

func (f *Fetcher) fetch() {
	url := "https://swapi.dev/api/people/?format=json"

	for {
		select {
		case <-f.ctx.Done():
			return

		default:
			log.Printf("INFO: fetching url %s", url)
			resp, err := f.getResponse(url)
			if err != nil {
				log.Printf("ERROR: failed to get response from api: %v", err)
				continue
			}
			if len(resp.Results) == 0 {
				return
			}

			for _, res := range resp.Results {
				f.ch <- res.Name
			}

			if resp.Next == "" {
				f.Stop()
				return
			}

			url = resp.Next
		}
	}
}

func (f *Fetcher) process() {
	ticker := time.NewTicker(bufferTimeout)

	for {
		select {
		case <-f.ctx.Done():
			ticker.Stop()
			f.flush()
			close(f.done)
			return

		case name := <-f.ch:
			f.insert(name)

		case <-ticker.C:
			f.flush()
			ticker.Reset(bufferTimeout)
		}
	}
}

func (f *Fetcher) insert(name string) {
	if len(f.buffer) >= bufferSize {
		f.flush()
	} else {
		f.buffer = append(f.buffer, name)
	}
}

func (f *Fetcher) flush() {
	if len(f.buffer) == 0 {
		return
	}

	names := f.buffer
	f.buffer = f.buffer[:0]

	for _, name := range names {
		if _, err := fmt.Fprintln(os.Stdout, name); err != nil {
			log.Printf("ERROR: failed to flush name to stdout: %v", err)
		}
	}

}

func (f *Fetcher) Stop() {
	f.cancelf()
	<-f.done
}

func (f *Fetcher) getResponse(url string) (*Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := f.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch persons: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code is not 200. code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	raw := &Response{}
	if err := json.Unmarshal(body, raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return raw, nil
}
