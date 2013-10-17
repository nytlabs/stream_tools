package streamtools

import (
	"github.com/bitly/go-simplejson"
	"log"
)

type ToLogBlock struct {
	AbstractBlock
}

func (b ToLogBlock) blockRoutine() {
	log.Println("starting to log block")
	for {
		select {
		case msg := <-b.inChan:
			msgStr, err := msg.MarshalJSON()
			if err != nil {
				log.Println("wow bad json")
			}
			log.Println(string(msgStr))
		}
	}
}

func NewToLog() Block {
	// create an empty ticker
	b := new(ToLogBlock)
	// specify the type for library
	b.blockType = "tolog"
	// make the outChan
	b.inChan = make(chan *simplejson.Json)
	b.outChan = make(chan *simplejson.Json)
	return b
}
