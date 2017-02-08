package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/cli"
)

func httpCommandFactory() (cli.Command, error) {
	ui := &cli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr}
	return httpCommand{
		ui: ui,
	}, nil
}

type httpCommand struct {
	ui cli.Ui
}

func (c httpCommand) Help() string {
	helpText := `
Send points using http protocol

Accepted arguments:

 -host:    hostname of Mycenae (REQUIRED)
 -port:    http port of Mycenae (defaults to 8080)
 -timeout: timeout duration (defaults to 5s)
 -iter:    number of requests to send (defaluts to infity)
 -size:    number of points per request (defaults to 750)
 -ts:      number of unique timeseries (defaults to max int64)
 -ks:      path to keyspaces file (defaults to keyspaces.json)
 -debug:   print information about the request

 `
	return strings.TrimSpace(helpText)
}

func (c httpCommand) Synopsis() string {
	return `Send points using http protocol`
}

func (c httpCommand) Run(args []string) int {

	var address, ksp string
	var port, size, iter int
	var uts int64
	var timeout time.Duration
	var debug bool

	d, _ := time.ParseDuration("5s")

	cmdFlags := flag.NewFlagSet("http", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.ui.Output(c.Help()) }

	cmdFlags.StringVar(&address, "host", "", "")
	cmdFlags.StringVar(&ksp, "ks", "keyspaces.json", "")
	cmdFlags.IntVar(&port, "port", 8080, "")
	cmdFlags.IntVar(&size, "size", 750, "")
	cmdFlags.IntVar(&iter, "iter", 0, "")
	cmdFlags.Int64Var(&uts, "ts", 9223372036854775807, "")
	cmdFlags.BoolVar(&debug, "debug", false, "")
	cmdFlags.DurationVar(&timeout, "timeout", d, "")
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	if address == "" {
		c.ui.Error(c.Help())
		return 1
	}

	if size < 0 {
		c.ui.Error(c.Help())
		return 1
	}

	if uts < 0 {
		c.ui.Error(c.Help())
		return 1
	}

	if iter < 0 {
		c.ui.Error(c.Help())
		return 1
	}

	f, err := os.Open(ksp)
	if err != nil {
		log.Println(err)
		return 2
	}

	dec := json.NewDecoder(f)

	keyspaces := []string{}

	err = dec.Decode(&keyspaces)
	if err != nil {
		log.Println(err)
		return 2
	}

	if iter == 0 {
		for {
			code := execute(address, timeout, port, size, keyspaces, uts, debug)
			if code != 0 {
				return code
			}
		}
	} else {
		for i := 1; i <= iter; i++ {
			code := execute(address, timeout, port, size, keyspaces, uts, debug)
			if code != 0 {
				return code
			}
		}
	}

	return 0
}

func execute(address string, timeout time.Duration, port, size int, keyspaces []string, uts int64, debug bool) int {

	now := time.Now()

	rs := rand.NewSource(now.UnixNano())
	r := rand.New(rs)

	points := make([]Point, size)

	for i := 0; i < size; i++ {

		ks := keyspaces[0]

		if len(ks) > 1 {
			index := r.Intn(len(keyspaces))
			if index != 0 {
				index--
			}
			ks = keyspaces[index]
		}

		p := Point{
			Value:  r.Float64(),
			Metric: fmt.Sprintf("sendPoints-%d", now.Unix()),
			Tags: map[string]string{
				"ksid": ks,
				"host": fmt.Sprintf("h-%d", r.Int63n(uts)),
			},
			Timestamp: time.Now().UnixNano() / 1e6,
		}

		points[i] = p
	}

	b, err := json.Marshal(points)
	if err != nil {
		log.Println(err)
		return 2
	}

	if debug {
		fmt.Println(string(b))
	}

	body := &bytes.Buffer{}

	_, err =body.Write(b)
	if err != nil {
		log.Println(err)
		return 2
	}

	sendHTTP(address, port, timeout, body)

	return 0
}

func sendHTTP(addr string, port int, timeout time.Duration, body io.Reader) {

	client := &http.Client{
		Timeout: timeout,
	}

	url := fmt.Sprintf("http://%s:%d/api/put", addr, port)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		log.Println(err)
		return
	}

	startTime := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return
	}

	elapsedTime := time.Since(startTime)

	defer resp.Body.Close()

	code := resp.StatusCode

	fmt.Println("request returned code:", code, "and took:", elapsedTime)

	if code == http.StatusNoContent {
		return
	}

	reqResponse, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	fmt.Println(string(reqResponse))
}
