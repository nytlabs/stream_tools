package library

import (
    "github.com/nytlabs/streamtools/st/blocks" // blocks
)

// specify those channels we're going to use to communicate with streamtools
type ToLog struct {
    blocks.Block
    in        chan interface{}
    quit      chan interface{}
}

// we need to build a simple factory so that streamtools can make new blocks of this kind
func NewToLog() blocks.BlockInterface {
    return &ToLog{}
}

// Setup is called once before running the block. We build up the channels and specify what kind of block this is.
func (b *ToLog) Setup() {
    b.Kind = "ToLog"
    b.in = b.InRoute("in")
    b.quit = b.Quit()
}

// Run is the block's main loop. Here we listen on the different channels we set up.
func (b *ToLog) Run() {
    for {
        select {
        case <-b.quit:
            return
        case msg := <-b.in:
            b.Log(msg)
        }
    }
}
