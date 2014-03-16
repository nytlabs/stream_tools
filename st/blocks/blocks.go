package blocks

import(
	"time"
    "github.com/nytlabs/streamtools/st/loghub"
    "fmt"
)

type Msg struct {
	Msg   interface{}
	Route string
}

type AddChanMsg struct {
	Route   string
	Channel chan *Msg
}

type QueryMsg struct {
	Route    string
	RespChan chan interface{}
}

type BlockChans struct {
	InChan    chan *Msg
	QueryChan chan *QueryMsg
	AddChan   chan *AddChanMsg
	DelChan   chan *Msg
	ErrChan   chan error
	QuitChan  chan bool
}

type LogStreams struct {
	log			chan interface{}
	ui 			chan interface{}
}

type Block struct {
	Id        	string // the name of the block specifed by the user (like MyBlock)
	Kind        string // the kind of block this is (like count, toFile, fromSQS)
	inRoutes    map[string]chan interface{}
	queryRoutes map[string]chan chan interface{}
	broadcast   chan interface{}
	quit 		chan interface{}
	doesBroadcast bool
	BlockChans
	LogStreams
}

type BlockDef struct {
	Type string
	InRoutes []string
	QueryRoutes [] string
	OutRoutes []string
}

type BlockInterface interface {
	Setup()
	Run()
	CleanUp()
	Build(BlockChans)
	Quit() chan interface{}
	Broadcast() chan interface{}
	InRoute(string) chan interface{}
	QueryRoute(string) chan chan interface{}
	GetBlock() *Block
	GetDef() *BlockDef
	Log(interface{})
	Error(interface{})
	SetId(string)
}

func (b *Block) Build(c BlockChans) {
	// fuck can I do this all in one?
	b.InChan = c.InChan
	b.QueryChan = c.QueryChan
	b.AddChan = c.AddChan
	b.DelChan = c.DelChan
	b.ErrChan = c.ErrChan
	b.QuitChan = c.QuitChan

	// route maps
	b.inRoutes = make(map[string]chan interface{}) // necessary to stop locking...
	b.queryRoutes = make(map[string]chan chan interface{})

	// broadcast channel
	b.broadcast = make(chan interface{}, 10) // necessary to stop locking...

	// quit chan
	b.quit = make(chan interface{})

	b.ui = make(chan interface{})
	b.log = make(chan interface{})
}

func (b *Block) SetId(Id string){
	b.Id = Id
}

func (b *Block) InRoute(routeName string) chan interface{} {
	route := make(chan interface{}, 1000)
	b.inRoutes[routeName] = route
	return route
}

func (b *Block) QueryRoute(routeName string) chan chan interface{} {
	route := make(chan chan interface{}, 1000)
	b.queryRoutes[routeName] = route
	return route
}

func (b *Block) Broadcast() chan interface{} {
	b.doesBroadcast = true
	return b.broadcast
}

func (b *Block) Quit() chan interface{} {
	return b.quit
}

func (b *Block) GetBlock() *Block {
	return b
}

func (b *Block) GetDef() *BlockDef {
	inRoutes := []string{}
	queryRoutes := []string{}
	outRoutes := []string{}

	for k, _ := range b.inRoutes {
		inRoutes = append(inRoutes, k)
	}

	for k, _ := range b.queryRoutes {
		queryRoutes = append(queryRoutes, k)
	} 

	if b.doesBroadcast {
		outRoutes = []string{"out"}
	} 

	return &BlockDef{
		Type: b.Kind,
		InRoutes: inRoutes,
		QueryRoutes: queryRoutes, 
		OutRoutes: outRoutes,
	}
}

func (b *Block) CleanUp() {
	for route := range b.inRoutes {
		defer close(b.inRoutes[route])
	}
	for route := range b.queryRoutes {
		defer close(b.queryRoutes[route])
	}
	defer close(b.InChan)
	defer close(b.QueryChan)
	defer close(b.AddChan)
	defer close(b.DelChan)
	defer close(b.ErrChan)
	defer close(b.QuitChan)
	defer close(b.broadcast)

	go func(id string){
	    loghub.Log <- &loghub.LogMsg{
	        Type: loghub.INFO,
	        Data: fmt.Sprintf("Block %s Quitting...", b.Id),
	        Id: id,
	    }
	}(b.Id)
}

func (b *Block) Error(msg interface{}) {
	go func(id string){
	    loghub.Log <- &loghub.LogMsg{
	        Type: loghub.ERROR,
	        Data: msg,
	        Id: id,
	    }
	}(b.Id)
}

func (b *Block) Log(msg interface{}){
	go func(id string) {
	    loghub.Log <- &loghub.LogMsg{
	        Type: loghub.INFO,
	        Data: msg,
	        Id: id,
	    }
	}(b.Id)
}

func BlockRoutine(bi BlockInterface) {
	outChans := make(map[string]chan *Msg)

	b := bi.GetBlock()
	bi.Setup()
	go bi.Run()

	for {
		select {
		case msg := <-b.InChan:
			_, ok := b.inRoutes[msg.Route]
			if !ok {
				break
			}

			// every in channel is buffered a 1000 messages.
			// if we cannot immediately send to that in channel we drop the msg
			// and notify the user that the block routine's buffer has 
			// overflowed. 

			// in the future we may want some kind of graceful solution that backs
			// up to memory, but in the current solution we don't have to check 
			// to see if run() is still alive. 

			select {
				case b.inRoutes[msg.Route] <- msg.Msg:
				default:
					go func(id string){
						loghub.Log <- &loghub.LogMsg{
							Type: loghub.ERROR,
							Data: "critical: block is dropping messages",
							Id:   id,
						}
					}(b.Id)
			}

			if msg.Route == "rule" {
				go func(id string){
					loghub.UI <- &loghub.LogMsg{
						Type: loghub.RULE_UPDATED,
						Data: map[string]interface{}{},
						Id:   id,
					}
				}(b.Id)
			}

		case msg := <-b.QueryChan:
			_, ok := b.queryRoutes[msg.Route]
			if !ok {
				break
			}

			select {
				case b.queryRoutes[msg.Route] <- msg.RespChan:
				default:
					msg.RespChan <- map[string]interface{}{
						"error": "block has timed out",
					}
					// note that we can't query right now
					go func(id string){
						loghub.Log <- &loghub.LogMsg{
							Type: loghub.ERROR,
							Data: "critical: block is dropping messages",
							Id:   id,
						}
					}(b.Id)
			}

		case msg := <-b.AddChan:
			outChans[msg.Route] = msg.Channel
		case msg := <-b.DelChan:
			delete(outChans, msg.Route)
		case msg := <-b.broadcast:
			for _, v := range outChans {
				v <- &Msg{
					Msg:   msg,
					Route: "",
				}
			}
		case <-b.QuitChan:
			b.quit <- true
			b.CleanUp()
			return
		}
	}
}

type Connection struct {
	Id 	string
	ToRoute string
	BlockChans
	LogStreams
}

func (c *Connection) SetId(Id string){
	c.Id = Id
}

func (c *Connection) Build(chans BlockChans){
	c.InChan = chans.InChan
	c.QueryChan = chans.QueryChan
	c.AddChan = chans.AddChan
	c.DelChan = chans.DelChan
	c.QuitChan = chans.QuitChan
}

func (c *Connection) CleanUp(){
	defer close(c.InChan)
	defer close(c.QueryChan)
	defer close(c.AddChan)
	defer close(c.DelChan)
	defer close(c.QuitChan)

    loghub.Log <- &loghub.LogMsg{
        Type: loghub.INFO,
        Data: fmt.Sprintf("Connection %s Quitting...", c.Id),
        Id:   c.Id,
    }
}

func ConnectionRoutine(c *Connection){
	var last interface{}
	var rate float64

	outChans := make(map[string]chan *Msg)
	times := make([]int64,100,100)
	timesIdx := len(times)
	rateReport := time.NewTicker(200 * time.Millisecond)

	for{
		select{
		case <- rateReport.C:
			if timesIdx == len(times) {
				rate = 0
			} else {
				rate = 1000000000.0 * float64(len(times) - timesIdx)/float64(time.Now().UnixNano() - times[timesIdx])
			}

			go func(id string, r float64){
			    loghub.UI <- &loghub.LogMsg{
			        Type: loghub.UPDATE_RATE,
			        Data: map[string]interface{}{
			        	"Rate": r,
			        },
			        Id:   id,
			    }
		    }(c.Id, rate)

		case msg := <- c.InChan:
			last = msg.Msg
			for _, v := range outChans {
				v <- &Msg{
					Msg:   msg.Msg,
					Route: c.ToRoute,
				}
			}

			times = times[1:]
			times = append(times, time.Now().UnixNano())

			if timesIdx > 0 {
				timesIdx--
			}

		case msg := <- c.QueryChan:
			switch msg.Route {
			case "rate":
				msg.RespChan <- map[string]interface{}{
					"Rate" : rate,
				}
			case "last":
				msg.RespChan <- map[string]interface{}{
					"Last" : last,
				}
			}
		case msg := <- c.AddChan:
			outChans[msg.Route] = msg.Channel
		case msg := <- c.DelChan:
			delete(outChans, msg.Route)
		case <- c.QuitChan:
			c.CleanUp()
			return
		}
	}
}
