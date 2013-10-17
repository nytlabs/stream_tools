package daemon

import (
	"flag"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/nytlabs/streamtools/blocks"
	"log"
	"net/http"
	"strings"
)

var (
	// channel that returns the next ID
	idChan chan string
	// port that streamtools reuns on
	port = flag.String("port", "7070", "stream tools port")
)

// hub keeps track of all the blocks and connections
type hub struct {
	connectionMap map[string]blocks.Block
	blockMap      map[string]blocks.Block
}

// HANDLERS

// The rootHandler returns information about the whole system
func (self *hub) rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello! this is streamtools")
	fmt.Fprintln(w, "ID: BlockType")
	for id, block := range self.blockMap {
		fmt.Fprintln(w, id+":", block.GetBlockType())
	}
}

// The createHandler creates new blocks
func (self *hub) createHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("could not parse form on /create")
	}
	if blockType, ok := r.Form["blockType"]; ok {

		var id string
		if blockId, ok := r.Form["id"]; ok {
			id = blockId[0]
		} else {
			id = <-idChan
		}
		self.CreateBlock(blockType[0], id)

	} else {
		log.Println("no blocktype specified")
	}
}

// The connectHandler connects together two blocks
func (self *hub) connectHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("could not parse form on /connect")
	}
	from := r.Form["from"][0]
	to := r.Form["to"][0]
	log.Println("connecting", from, "to", to)
	self.CreateConnection(from, to)
}

// The routeHandler deals with any incoming message sent to an arbitrary block endpoint
func (self *hub) routeHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.Split(r.URL.Path, "/")[2]
	route := strings.Split(r.URL.Path, "/")[3]

	err := r.ParseForm()
	var respData string
	for k, _ := range r.Form {
		respData = k
	}
	msg, err := simplejson.NewJson([]byte(respData))
	if err != nil {
		msg = nil
	}
	ResponseChan := make(chan *simplejson.Json)
	blockRouteChan := self.blockMap[id].GetRouteChan(route)
	blockRouteChan <- blocks.RouteResponse{
		Msg:          msg,
		ResponseChan: ResponseChan,
	}
	blockMsg := <-ResponseChan
	out, err := blockMsg.MarshalJSON()
	if err != nil {
		log.Println(err.Error())
	}

	fmt.Fprint(w, string(out))
}

func (self *hub) libraryHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, libraryBlob)
}

func (self *hub) CreateConnection(from string, to string) {
	conn := library["connection"].blockFactory()
	conn.InitOutChans()
	id := <-idChan
	conn.SetID(id)

	fromChan := self.blockMap[from].CreateOutChan(conn.GetID())
	conn.SetInChan(fromChan)

	toChan := self.blockMap[to].GetInChan()
	conn.SetOutChan(to, toChan)

	self.connectionMap[conn.GetID()] = conn
	go conn.BlockRoutine()
}

func (self *hub) CreateBlock(blockType string, id string) {
	blockTemplate, ok := library[blockType]
	if !ok {
		log.Fatal("couldn't find block", blockType)
	}
	block := blockTemplate.blockFactory()
	block.InitOutChans()

	block.SetID(id)
	self.blockMap[id] = block

	routeNames := block.GetRoutes()
	for _, routeName := range routeNames {
		http.HandleFunc("/blocks/"+block.GetID()+"/"+routeName, self.routeHandler)
	}

	go block.BlockRoutine()
}

func (self *hub) Run() {

	// start the ID Service
	idChan = make(chan string)
	go IDService(idChan)

	// start the library service
	buildLibrary()

	// initialise the connection and block maps
	self.connectionMap = make(map[string]blocks.Block)
	self.blockMap = make(map[string]blocks.Block)

	// instantiate the base handlers
	http.HandleFunc("/", self.rootHandler)
	http.HandleFunc("/create", self.createHandler)
	http.HandleFunc("/connect", self.connectHandler)
	http.HandleFunc("/library", self.libraryHandler)

	// start the http server
	log.Println("starting stream tools on port", *port)
	err := http.ListenAndServe(":"+*port, nil)
	if err != nil {
		log.Fatalf(err.Error())
	}
}

func Run() {
	h := hub{}
	h.Run()
}
