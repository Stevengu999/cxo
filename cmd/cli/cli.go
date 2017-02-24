package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/peterh/liner"
)

const (
	DEFAULT_CXOD_ADDR = "http://127.0.0.1:6481"
	DEFAULT_TIMEOUT   = 0

	HISTORY_FILE   = ".cxo_cli_history"
	LOG_PREFIX     = "[cxo cli] "
	ERROR_BODY_LEN = 500
)

// - list subscriptions (number of peers, size of data for subscription)
// - add subscription
// - list connections for subscription
// - add connection (IP:port) for subscription
// - remove subscription
// - get data size for subscriptions

func main() {
	// initialize logger
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[cxo cli] ")

	// flags
	addr := flag.String("a", DEFAULT_CXOD_ADDR, "server address")
	timeout := flag.Duration("t", DEFAULT_TIMEOUT, "request/response timeout")
	help := flag.Bool("h", false, "show help")
	debug := flag.Bool("d", false, "print debug logs")
	flag.Parse()
	if *help {
		flag.PrintDefaults()
		return
	}

	// http client
	client := Client{
		addr: *addr,
		Client: http.Client{
			Timeout: *timeout,
		},
		debug: *debug,
	}

	// liner
	line := liner.NewLiner()
	defer line.Close()

	readHistory(line)
	defer storeHistory(line)

	line.SetCtrlCAborts(true)
	line.SetCompleter(autoComplite)
	line.SetTabCompletionStyle(liner.TabPrints)

	log.Print("starting client")
	log.Print("address:    ", *addr)
	if *timeout == 0 {
		log.Print("timeout:    no limits")
	} else {
		log.Print("timeout:    ", *timeout)
	}
	log.Print("debug logs: ", *debug)

	fmt.Println("enter 'help' to get help")
	var err error
	var inpt string
	// prompt loop
	for {
		inpt, err = line.Prompt("> ")
		if err != nil {
			log.Print("fatal: ", err)
			return
		}
		inpt = strings.TrimSpace(strings.ToLower(inpt))
		switch {

		case strings.HasPrefix(inpt, "list subscriptions"):
			client.listSubscriptions()

		case strings.HasPrefix(inpt, "list subscribers"):
			client.listSubscribers()

		case strings.HasPrefix(inpt, "list"):
			fmt.Println(`list what?
	- list subscriptions
	- list subscribers`)
			continue

		case strings.HasPrefix(inpt, "add subscription"):
			client.addSubscription(trim(inpt, "add subscription"))

		case strings.HasPrefix(inpt, "add"):
			fmt.Println(`do you mean 'add subscription'?`)
			continue

		case strings.HasPrefix(inpt, "remove subscription"):
			client.removeSubscription(trim(inpt, "remove subscription"))

		case strings.HasPrefix(inpt, "remove subscriber"):
			client.removeSubscriber(trim(inpt, "remove subscriber"))

		case strings.HasPrefix(inpt, "remove"):
			fmt.Println(`remove what?
	- remove subscription
	- remove subscriber`)
			continue

		case strings.HasPrefix(inpt, "stat"):
			client.getStat()

		case strings.HasPrefix(inpt, "info"):
			client.getNodeInfo()

		case strings.HasPrefix(inpt, "help"):
			printHelp()

		case strings.HasPrefix(inpt, "exit"):
			fallthrough

		case strings.HasPrefix(inpt, "quit"):
			fmt.Println("cya")
			return

		case inpt == "":
			continue // do noting properly

		default:
			fmt.Println("unknown command:", inpt)
			continue // no errors, no history

		}
		line.AppendHistory(inpt)
	}
}

// utility, printf and break line
func printf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

// historyFilePath returns path to ~/HISTORY_FILE or error if any
func historyFilePath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, HISTORY_FILE), nil
}

// readHistory from history file
func readHistory(line *liner.State) {
	// don't report errors
	pth, err := historyFilePath()
	if err != nil {
		return
	}
	if fl, err := os.Open(pth); err == nil {
		line.ReadHistory(fl)
		fl.Close()
	}
}

// storeHistory to history file
func storeHistory(line *liner.State) {
	pth, err := historyFilePath()
	if err != nil {
		log.Print("error obtaining history file path: ", err)
		return
	}
	fl, err := os.Create(pth)
	if err != nil {
		log.Print("error creating histrory file: ", err)
		return
	}
	defer fl.Close()
	line.WriteHistory(fl)
}

var complets = []string{
	"list subscriptions ",
	"list subscribers ",
	"list ",
	"add subscription ",
	"remove subscription ",
	"remove subscriber",
	"stat ",
	"info",
	"help ",
	"exit ",
	"quit ",
}

func autoComplite(line string) (cm []string) {
	if line == "" {
		return complets
	}
	for _, c := range complets {
		if strings.HasPrefix(c, strings.ToLower(line)) {
			cm = append(cm, c)
		}
	}
	return
}

// TODO: help
func printHelp() {
	fmt.Print(`Available commands:

	list subscriptions
		list all subscriptions
	list subscribers
		list all subscribers
	add subscription <address> [desired public key]
		add subscription to given address, the public key is optional
	remove subscription <id or address>
		remove subscription by id or address
	remove subscriber <id or address>
		remove subscriber by id or address
	stat
		get statistic (total objects, memory) of all objects
	info
		print node id and address
	help
		show this help message
	exit or
	quit
		quit cli
`)
}

// trim cmd prefix from inpt and trim spaces
func trim(inpt string, cmd string) string {
	return strings.TrimSpace(strings.TrimPrefix(inpt, cmd))

}

// request/reply functions

// net/http.Client wrapper
type Client struct {
	http.Client
	addr  string
	debug bool
}

func (c *Client) Debug(args ...interface{}) {
	if c.debug {
		log.Print(args...)
	}
}

//
// request node id, ip and port
//

func (c *Client) getNodeInfo() (err error) {
	// GET /node
	// => {
	//     "address":   "[::]:34735",
	//     "listening": true,
	//     "pubKey":    "hex"
	//    }

	var node map[string]interface{} = make(map[string]interface{})

	resp, err := c.Get(c.addr + "/node")
	if err != nil {
		fmt.Println("error requesting node info: ", err)
		return
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&node); err != nil {
		fmt.Println("error decoding response: ", err)
		return
	}

	for _, k := range []string{"address", "listening", "pubKey"} {
		if v, ok := node[k]; ok {
			printf("%-20s %v", k+":", v)
			continue
		}
		printf("unable to obtain %s value", k)
	}

	return
}

//
// list
//

func (c *Client) getSubscriptionsList() (subscriptions []Item, err error) {
	// list subscriptions
	//
	// GET /manager/nodes/:node_id/subscriptions
	// => []Item

	subscriptions, err = c.getList(c.addr+
		"/manager/nodes/stub/subscriptions", true)
	return
}

func (c *Client) listSubscriptions() {
	subscriptions, err := c.getSubscriptionsList()
	if err != nil {
		fmt.Println("error requesting subscriptions:", err)
		return
	}
	// huminize the list
	if len(subscriptions) == 0 {
		fmt.Println("  there aren't subscriptions")
		return
	}
	for _, s := range subscriptions {
		printf("  %s %s",
			s.Address,
			s.PubKey)
	}
}

func (c *Client) getConnectionsList() (connections []Item, err error) {
	// list subscribers
	//
	// GET /manager/nodes/:node_id/subscribers
	// => []Item

	connections, err = c.getList(c.addr+
		"/manager/nodes/stub/subscribers", true)
	return
}

// listSubscribers requests list of subscribers
func (c *Client) listSubscribers() {
	subscribers, err := c.getConnectionsList()
	if err != nil {
		fmt.Println("error requesting connections:", err)
		return
	}

	// huminize the list
	if len(subscribers) == 0 {
		fmt.Println("  there aren't connections")
		return
	}
	for _, s := range subscribers {
		printf("  %s %s",
			s.Address,
			s.PubKey)
	}

}

//
// add
//

func (c *Client) addSubscription(args string) {
	// POST "/manager/nodes/:node_id/subscriptions"
	//   {"ip": "host:port", "pubKey": "theKey"}
	var reqp, reqb string // requset URL and request body
	switch ss := strings.Fields(args); len(ss) {
	case 0:
		fmt.Println("to few arguments, want <host:port> [pub key]")
		return
	case 1:
		reqp = c.addr + "/manager/nodes/stub/subscriptions"
		reqb = fmt.Sprintf(`{"ip":%q,"pubKey":""}`, ss[0])
	case 2:
		reqp = c.addr + "/manager/nodes/stub/subscriptions"
		reqb = fmt.Sprintf(`{"ip":%q,"pubKey":%q}`, ss[0], ss[1])
	default:
		fmt.Println("to many arguments, want <host:port> [pub key]")
		return
	}
	//
	c.Debug("[POST] ", reqp, reqb)
	resp, err := c.Post(reqp, "application/json", strings.NewReader(reqb))
	if err != nil {
		fmt.Println("request error:", err)
		return
	}
	c.Debug("response status: ", resp.Status)
	defer resp.Body.Close()
	// anyway it's JSONResponse
	jr, err := readResponse(resp)
	if err != nil {
		fmt.Println("error reading response: ", err)
		return
	}
	// detailed error or success message
	fmt.Println(" ", jr.Detail)
}

//
// remove
//

func (c *Client) removeSubscription(args string) {
	// DELETE "/manager/nodes/:node_id/subscriptions/:subscription_id"

	var (
		reqs    string // remove using id
		address string // or remove using address

		err error
	)
	switch ss := strings.Fields(args); len(ss) {
	case 0:
		fmt.Println("to few argumets, want: <id or ip:port>")
		return
	case 1:
		if strings.Contains(ss[0], ":") {
			address = ss[0]
			break
		}
		reqs = c.addr + "/manager/nodes/stub/subscriptions/" +
			url.QueryEscape(ss[0])
	default:
		fmt.Println("to many argumets, want: <id or ip:port>")
		return
	}

	// request list of subscriptions and determine id by host:port
	if reqs == "" {
		var subscriptions []Item
		subscriptions, err = c.getSubscriptionsList()
		if err != nil {
			fmt.Println("error requesting subscriptions list", err)
			return
		}
		for _, s := range subscriptions {
			if s.Address == address {
				reqs = c.addr + "/manager/nodes/stub/subscriptions/" +
					s.PubKey
				goto Request
			}
		}
		// not found
		printf("subscription %s not found", address)
		return
	}
Request:
	req, err := http.NewRequest("DELETE", reqs, nil)
	if err != nil {
		log.Print("request creating error:", err) // BUG
		return
	}

	c.Debug("[DELETE] ", reqs)
	resp, err := c.Do(req)
	if err != nil {
		fmt.Println("request error:", err)
		return
	}

	c.Debug("response status: ", resp.Status)
	defer resp.Body.Close()
	// anyway it's JSONResponse
	jr, err := readResponse(resp)
	if err != nil {
		fmt.Println("error reading response: ", err)
		return
	}
	// detailed error or success message
	fmt.Println(" ", jr.Detail)

}

func (c *Client) removeSubscriber(args string) {
	// DELETE "/manager/nodes/:node_id/subscribers/:subscriber_id

	var (
		reqs    string // remove using id
		address string // or remove using address

		err error
	)
	switch ss := strings.Fields(args); len(ss) {
	case 0:
		fmt.Println("to few argumets, want: <id or ip:port>")
		return
	case 1:
		if strings.Contains(ss[0], ":") {
			address = ss[0]
			break
		}
		reqs = c.addr + "/manager/nodes/stub/subscribers/" +
			url.QueryEscape(ss[0])
	default:
		fmt.Println("to many argumets, want: <id or ip:port>")
		return
	}

	// request list of subscribers and determine id by host:port
	if reqs == "" {
		var subscribers []Item
		subscribers, err = c.getConnectionsList()
		if err != nil {
			fmt.Println("error requesting subscribers list", err)
			return
		}
		for _, s := range subscribers {
			if s.Address == address {
				reqs = c.addr + "/manager/nodes/stub/subscribers/" + s.PubKey
				goto Request
			}
		}
		// not found
		printf("connection %s not found", address)
		return
	}
Request:
	req, err := http.NewRequest("DELETE", reqs, nil)
	if err != nil {
		log.Print("request creating error:", err) // BUG
		return
	}

	c.Debug("[DELETE] ", reqs)
	resp, err := c.Do(req)
	if err != nil {
		fmt.Println("request error:", err)
		return
	}

	c.Debug("response status: ", resp.Status)
	defer resp.Body.Close()
	// anyway it's JSONResponse
	jr, err := readResponse(resp)
	if err != nil {
		fmt.Println("error reading response: ", err)
		return
	}
	// detailed error or success message
	fmt.Println(" ", jr.Detail)

}

//
// stat
//

func (c *Client) getStat() {
	// GET "/object1/_stat"

	var req string = c.addr + "/object1/_stat"
	c.Debug("[GET] ", req)
	resp, err := c.Get(req)
	if err != nil {
		fmt.Println("request error:", err)
		return
	}
	c.Debug("response status: ", resp.Status)
	defer resp.Body.Close()
	// no error descripto
	if resp.StatusCode != 200 {
		fmt.Println("response error:", resp.Status)
		return
	}
	// read stat
	var stat Statistic
	if err = json.NewDecoder(resp.Body).Decode(&stat); err != nil {
		fmt.Println("error decoding response:", err)
		return
	}
	// print the stat
	fmt.Println("total objects:", stat.Total)
	fmt.Println("memory:       ", humanMemory(stat.Memory))
}

//
// helpers
//

func (c *Client) getList(url string, jerr bool) (li []Item, err error) {
	c.Debug("[GET] ", url)

	var resp *http.Response
	if resp, err = c.Get(url); err != nil {
		return
	}
	c.Debug("response status: ", resp.Status)

	defer resp.Body.Close()

	// on success we've got 200 or 201
	if !(resp.StatusCode == 200 || resp.StatusCode == 201) {
		// jerr indicate JSONResponse error report
		if !jerr {
			err = fmt.Errorf("invalid response status: %s", resp.Status)
			return
		}
		// error returns JSONResponse
		jr, e := readResponse(resp)
		if e != nil {
			e = fmt.Errorf("error decoding response: %s", err.Error())
			return
		}
		err = fmt.Errorf("response error: %s", jr.Detail)
		return
	}

	err = json.NewDecoder(resp.Body).Decode(&li)
	return
}

func readResponse(resp *http.Response) (jr JSONResponse, err error) {
	err = json.NewDecoder(resp.Body).Decode(&jr)
	return
}

// humanMemory returns human readable memory string
func humanMemory(bytes int) string {
	var fb float64 = float64(bytes)
	var ms string = "B"
	for _, m := range []string{"KiB", "MiB", "GiB"} {
		if fb > 1024.0 {
			fb = fb / 1024.0
			ms = m
			continue
		}
		break
	}
	if ms == "B" {
		return fmt.Sprintf("%.0fB", fb)
	}
	// 2.00 => 2
	// 2.10 => 2.1
	// 2.53 => 2.53
	return strings.TrimRight(
		strings.TrimRight(fmt.Sprintf("%.2f", fb), "0"),
		".") + ms
}

// nessesary JSON-structures

// skycoin/cxo/gui/errors.go
type JSONResponse struct {
	Code   string                  `json:"code,omitempty"`
	Status int                     `json:"status,omitempty"`
	Detail string                  `json:"detail,omitempty"`
	Meta   *map[string]interface{} `json:"meta,omitempty"`
}

// list nodes or list subscriptions
type Item struct {
	Address string `json:"addr"`
	PubKey  string `json:"pub"`
}

// stat cxo/data/db.go
type Statistic struct {
	Total  int `json:"total"`
	Memory int `json:"memory"`
}