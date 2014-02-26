package library

import (
	"encoding/json"
	"github.com/nikhan/go-sqsReader"           //sqsReader
	"github.com/nytlabs/streamtools/st/blocks" // blocks
	"github.com/nytlabs/streamtools/st/util"
)

// specify those channels we're going to use to communicate with streamtools
type FromSQS struct {
	blocks.Block
	queryrule    chan chan interface{}
	inrule       chan interface{}
	out          chan interface{}
	fromReader   chan []byte
	quit         chan interface{}
	SQSEndpoint  string
	AccessKey    string
	AccessSecret string
}

// we need to build a simple factory so that streamtools can make new blocks of this kind
func NewFromSQS() blocks.BlockInterface {
	return &FromSQS{}
}

// Setup is called once before running the block. We build up the channels and specify what kind of block this is.
func (b *FromSQS) Setup() {
	b.Kind = "fromSQS"
	b.inrule = b.InRoute("rule")
	b.queryrule = b.QueryRoute("rule")
	b.quit = b.Quit()
	b.out = b.Broadcast()
	b.fromReader = make(chan []byte)
}

// Run is the block's main loop. Here we listen on the different channels we set up.
func (b *FromSQS) Run() {
	for {
		select {
		case msgI := <-b.inrule:
			SQSEndpoint, err := util.ParseString(msgI, "SQSEndpoint")
			if err != nil {
				b.Error(err)
				break
			}
			b.SQSEndpoint = SQSEndpoint

			AccessKey, err := util.ParseString(msgI, "AccessKey")
			if err != nil {
				b.Error(err)
				break
			}
			b.AccessKey = AccessKey

			AccessSecret, err := util.ParseString(msgI, "AccessSecret")
			if err != nil {
				b.Error(err)
				break
			}
			b.AccessSecret = AccessSecret

			r := sqsReader.NewReader(b.SQSEndpoint, b.AccessKey, b.AccessSecret, b.fromReader)
			go r.Start()
		case msg := <-b.fromReader:
			var outMsg interface{}
			err := json.Unmarshal(msg, &outMsg)
			if err != nil {
				b.Error(err)
				continue
			}
			b.out <- outMsg
		case <-b.quit:
			// quit the block
			return
		case respChan := <-b.queryrule:
			// deal with a query request
			respChan <- map[string]interface{}{
				"SQSEndpoint":  b.SQSEndpoint,
				"AccessKey":    b.AccessKey,
				"AccessSecret": b.AccessSecret,
			}
		}
	}
}
