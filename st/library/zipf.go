package library

import (
	"errors"
	"github.com/nytlabs/streamtools/st/blocks" // blocks
	"github.com/nytlabs/streamtools/st/util"   // util
	"math/rand"
)

// specify those channels we're going to use to communicate with streamtools
type Zipf struct {
	blocks.Block
	queryrule chan chan interface{}
	inrule    chan interface{}
	inpoll    chan interface{}
	out       chan interface{}
	quit      chan interface{}
}

// we need to build a simple factory so that streamtools can make new blocks of this kind
func NewZipf() blocks.BlockInterface {
	return &Zipf{}
}

// Setup is called once before running the block. We build up the channels and specify what kind of block this is.
func (b *Zipf) Setup() {
	b.Kind = "Zipf"
	b.inrule = b.InRoute("rule")
	b.queryrule = b.QueryRoute("rule")
	b.inpoll = b.InRoute("poll")
	b.quit = b.Quit()
	b.out = b.Broadcast()
}

// Run is the block's main loop. Here we listen on the different channels we set up.
// this is actually the Zipf-Manadlebrot "law".
// http://en.wikipedia.org/wiki/Zipf%E2%80%93Mandelbrot_law
// the parameter `v` is denoted `q` on wikipedia.
func (b *Zipf) Run() {
	var err error
	var s, v, imax float64
	r := rand.New(rand.NewSource(12345))
	sampler := rand.NewZipf(r, 2, 5, 99)
	for {
		select {
		case ruleI := <-b.inrule:
			// set a parameter of the block
			rule, ok := ruleI.(map[string]interface{})
			if !ok {
				b.Error(errors.New("couldn't assert rule to map"))
			}
			s, err = util.ParseFloat(rule, "s")
			if err != nil {
				b.Error(err)
			}
			v, err = util.ParseFloat(rule, "v")
			if err != nil {
				b.Error(err)
			}
			imax, err = util.ParseFloat(rule, "N")
			if err != nil {
				b.Error(err)
			}
			sampler = rand.NewZipf(r, s, v, uint64(imax))
		case <-b.quit:
			// quit the block
			return
		case <-b.inpoll:
			// deal with a poll request
			b.out <- map[string]interface{}{
				"sample": float64(sampler.Uint64()),
			}
		case respChan := <-b.queryrule:
			// deal with a query request
			out := map[string]interface{}{
				"s": s,
				"v": v,
				"N": imax,
			}
			respChan <- out
		}
	}
}
