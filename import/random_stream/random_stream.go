// Fakes a stream of data at your convenience.
// This contains a random time stamp, and random length array of random integers

package main

import (
	"flag"
	"github.com/bitly/go-simplejson"
	"math/rand"
	"time"
	//"strconv"
	"bytes"
	"log"
	"net/http"
)

var (
	topic       = flag.String("topic", "random", "nsq topic")
	jsonMsgPath = flag.String("file", "test.json", "json file to send")
	timeKey     = flag.String("key", "t", "key that holds time")

	nsqHTTPAddrs = "127.0.0.1:4151"
)

func writer() {
	msgJson, _ := simplejson.NewJson([]byte("{}"))
	client := &http.Client{}

	c := time.Tick(5 * time.Second)
	r := rand.New(rand.NewSource(99))

	for now := range c {
		a := int64(r.Float64() * 10000000000)
		strTime := now.UnixNano() - a
		msgJson.Set(*timeKey, int64(strTime/1000000))

		msgJson.Set("a", 10)

		b := make([]int, rand.Intn(10))

		for i, _ := range b {
			b[i] = rand.Intn(100)
		}

		msgJson.Set("b", b)

		outMsg, _ := msgJson.Encode()
		msgReader := bytes.NewReader(outMsg)
		resp, err := client.Post("http://"+nsqHTTPAddrs+"/put?topic="+*topic, "data/multi-part", msgReader)
		if err != nil {
			log.Fatalf(err.Error())
		}
		resp.Body.Close()
	}
}

func main() {

	flag.Parse()

	stopChan := make(chan int)

	go writer()

	<-stopChan
}
