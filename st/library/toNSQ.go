package library

import (
	"encoding/json"
	"github.com/bitly/go-nsq"
	"github.com/nytlabs/streamtools/st/blocks" // blocks
	"github.com/nytlabs/streamtools/st/util"
)

// specify those channels we're going to use to communicate with streamtools
type ToNSQ struct {
	blocks.Block
	queryrule    chan chan interface{}
	inrule       chan interface{}
	in           chan interface{}
	out          chan interface{}
	quit         chan interface{}
	nsqdTCPAddrs string
	topic        string
}

// a bit of boilerplate for streamtools
func NewToNSQ() blocks.BlockInterface {
	return &ToNSQ{}
}

func (b *ToNSQ) Setup() {
	b.Kind = "ToNSQ"
	b.in = b.InRoute("in")
	b.inrule = b.InRoute("rule")
	b.queryrule = b.QueryRoute("rule")
	b.quit = b.Quit()
	b.out = b.Broadcast()
}

// connects to an NSQ topic and emits each message into streamtools.
func (b *ToNSQ) Run() {
	var writer *nsq.Writer

	for {
		select {
		case ruleI := <-b.inrule:
			//rule := ruleI.(map[string]interface{})

			topic, err := util.ParseString(ruleI, "Topic")
			if err != nil {
				b.Error(err)
				break
			}

			nsqdTCPAddrs, err := util.ParseString(ruleI, "NsqdTCPAddrs")
			if err != nil {
				b.Error(err)
				break
			}

			writer = nsq.NewWriter(nsqdTCPAddrs)

			b.topic = topic
			b.nsqdTCPAddrs = nsqdTCPAddrs

		case msg := <-b.in:
			msgStr, err := json.Marshal(msg)
			if err != nil {
				b.Error(err)
			}
			_, _, err = writer.Publish(b.topic, []byte(msgStr))
			if err != nil {
				b.Error(err)
			}

		case <-b.quit:
			return
		case c := <-b.queryrule:
			c <- map[string]interface{}{
				"Topic":        b.topic,
				"NsqdTCPAddrs": b.nsqdTCPAddrs,
			}
		}
	}
}
