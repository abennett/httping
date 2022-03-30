package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"
)

var (
	MinimumFreq = 500 * time.Millisecond
	header      = []string{
		"timestamp",
		"status_code",
		"latency",
	}
)

type HTTPing struct {
	Timestamp  time.Time
	StatusCode int
	Latency    time.Duration
}

func (hp *HTTPing) SSlice() []string {
	out := make([]string, 3)
	out[0] = hp.Timestamp.Format(time.StampMilli)
	out[1] = strconv.Itoa(hp.StatusCode)
	out[2] = hp.Latency.Truncate(time.Millisecond).String()
	return out
}

func httpinger(ctx context.Context, u string, freq time.Duration) <-chan *HTTPing {
	hpCh := make(chan *HTTPing, 1)
	go func() {
		ticker := time.NewTicker(freq)
		for {
			select {
			case <-ticker.C:
				start := time.Now()
				resp, err := http.Get(u)
				if err != nil {
					log.Println(err)
				}
				hpCh <- &HTTPing{
					Timestamp:  start,
					StatusCode: resp.StatusCode,
					Latency:    time.Now().Sub(start),
				}
			case <-ctx.Done():
				close(hpCh)
				return
			}
		}
	}()
	return hpCh
}

func parseArgs() (string, time.Duration, error) {
	var d time.Duration
	if len(os.Args) != 3 {
		return "", d, errors.New("two args required: httping <url> <frequency> (1s, 500ms, etc.)")
	}
	u, err := url.Parse(os.Args[1])
	if err != nil {
		return "", d, fmt.Errorf("%s is not a valid url", os.Args[1])
	}
	if u.Scheme == "" {
		return "", d, errors.New("protocol scheme required")
	}
	d, err = time.ParseDuration(os.Args[2])
	if err != nil {
		return "", d, fmt.Errorf("%s is not a valid duration", os.Args[2])
	}
	if d < MinimumFreq {
		return "", d, fmt.Errorf("%s is below the minimum frequency of %s", d, MinimumFreq)
	}
	return u.String(), d, nil
}

func main() {
	u, freq, err := parseArgs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	ctx := context.Background()
	ctx, _ = signal.NotifyContext(ctx, os.Interrupt)
	writer := csv.NewWriter(os.Stdout)
	if err = writer.Write(header); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	hpCh := httpinger(ctx, u, freq)
	for hp := range hpCh {
		writer.Write(hp.SSlice())
		writer.Flush()
	}
	fmt.Println()
}
