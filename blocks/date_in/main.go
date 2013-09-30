package main

import (
	"flag"
	"github.com/bitly/go-simplejson"
	"github.com/nytlabs/streamtools/streamtools"
	"log"
)

var (
	topic     = flag.String("topic", "", "topic to write to")
	fmtString = flag.String("format", "", "format string (use Mon Jan 2 15:04:05 -0700 MST 2006)")
	name      = flag.String("name", "date-in", "name of block")
)

func main() {
	flag.Parse()
	streamtools.SetupLogger(name)
	block := streamtools.NewOutBlock(streamtools.Date, *name)
	rule, err := simplejson.NewJson([]byte("{}"))
	if err != nil {
		log.Fatal(err.Error())
	}
	rule.Set("fmtString", *fmtString)
	block.RuleChan <- rule
	block.Run(*topic, "8081")
}
