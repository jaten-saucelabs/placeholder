package main

// derived from https://github.com/glycerine/goq/blob/master/web.go
// license: https://github.com/glycerine/goq/blob/master/LICENSE

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" // side-effect: installs handlers for /debug/pprof
	"os"
	"time"

	"github.com/glycerine/go-tigertonic"
	"github.com/saucelabs/esflow/fireant"
)

type WebServer struct {
	Addr        string
	ServerReady chan bool // closed once server is listening on Addr
	Done        chan bool // closed when server shutdown.

	requestStop chan bool // private. Users should call Stop().
	tts         *tigertonic.Server
	started     bool
}

func NewWebServer(host, port, msg string) *WebServer {

	addr := fmt.Sprintf("%s:%s", host, port)
	log.Printf("starting webserver on '%s' for colorCode '%s'\n", addr, msg)

	s := &WebServer{
		Addr:        addr,
		ServerReady: make(chan bool),
		Done:        make(chan bool),
		requestStop: make(chan bool),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "200 OK %v\n", msg)
	})

	s.tts = tigertonic.NewServer(addr, http.DefaultServeMux) // supply debug/pprof diagnostics
	s.Start()
	return s
}

func (s *WebServer) Start() {
	if s.started {
		return
	}
	s.started = true

	go func() {
		err := s.tts.ListenAndServe()
		if nil != err {
			//log.Println(err) // accept tcp 127.0.0.1:3000: use of closed network connection
		}
		close(s.Done)
	}()

	WaitUntilServerUp(s.Addr)
	close(s.ServerReady)
}

func (s *WebServer) Stop() {
	close(s.requestStop)
	s.tts.Close()
	log.Printf("in WebServer::Stop() after s.tts.Close()\n")
	<-s.Done
	log.Printf("in WebServer::Stop() after <-s.Done(): s.Addr = '%s'\n", s.Addr)

	WaitUntilServerDown(s.Addr)
}

func (s *WebServer) IsStopRequested() bool {
	select {
	case <-s.requestStop:
		return true
	default:
		return false
	}
}

func WaitUntilServerUp(addr string) {
	attempt := 1
	for {
		if PortIsBound(addr) {
			return
		}
		time.Sleep(500 * time.Millisecond)
		attempt++
		if attempt > 40 {
			panic(fmt.Sprintf("could not connect to server at '%s' after 40 tries of 500msec", addr))
		}
	}
}

func WaitUntilServerDown(addr string) {
	attempt := 1
	for {
		if !PortIsBound(addr) {
			return
		}
		//fmt.Printf("WaitUntilServerUp: on attempt %d, sleep then try again\n", attempt)
		time.Sleep(500 * time.Millisecond)
		attempt++
		if attempt > 40 {
			panic(fmt.Sprintf("could always connect to server at '%s' after 40 tries of 500msec", addr))
		}
	}
}

func PortIsBound(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func FetchUrl(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return []byte{}, err
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return []byte{}, err
		}
		return contents, nil
	}
}

var healthWebServer *WebServer

const ProgramName = "placeholder"

func main() {
	var colorCode string
	var port int64
	fs := flag.NewFlagSet(ProgramName, flag.ExitOnError)

	fs.StringVar(&colorCode, "color", "blue", "colorCode to report as live")
	fs.Int64Var(&port, "port", 7701, "port to listen on and provide health reports")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		panic(err)
	}
	fireant.NewWebServer("0.0.0.0", fmt.Sprintf("%d", port), colorCode, log.New(os.Stderr, "", log.LUTC|log.LstdFlags|log.Lmicroseconds))
	select {}
}
