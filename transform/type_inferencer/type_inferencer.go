package main

import (
	"flag"
	"github.com/bitly/go-simplejson"
	"github.com/bitly/nsq/nsq"
	"log"
	"reflect"
    "net/http"
    "net/url"
)

var (
	topic            = flag.String("topic", "monitor", "nsq topic")
	channel          = flag.String("channel", "monitorreader", "nsq topic")
	maxInFlight      = flag.Int("max-in-flight", 1000, "max number of messages to allow in flight")
	nsqTCPAddrs      = flag.String("nsqd-tcp-address", "127.0.0.1:4150", "nsqd TCP address")
	nsqHTTPAddrs     = flag.String("nsqd-http-address", "127.0.0.1:4151", "nsqd HTTP address")
	lookupdHTTPAddrs = flag.String("lookupd-http-address", "127.0.0.1:4161", "lookupd HTTP address")
)

// returns a map with keys = flatten keys of dictionary and type = corresponding JSON types
func FlattenType(d map[string]interface{}, p string) map[string]string {

	out := make(map[string]string)

	for key, value := range d {

		new_p := ""
		if len(p) > 0 {
			new_p = p + "." + key
		} else {
			new_p = key
		}

		if value == nil {
			// got JSON type null
			out[key] = "null"

		} else if reflect.TypeOf(value).Kind() == reflect.Map {
			// got an object
			s, ok := value.(map[string]interface{})
			if ok {
				for k, v := range FlattenType(s, new_p) {
					out[k] = v
				}
			} else {
				log.Fatalf("expected type map, got something else instead. key=%s, s=%s", key, s)
			}

		} else if reflect.TypeOf(value).Kind() == reflect.Slice {
			// got an array
			new_p += ".[]"
			s, ok := value.([]interface{})
			if ok {
				for _, d2 := range s {
					if reflect.TypeOf(d2).Kind() == reflect.Map {
						s2, ok := d2.(map[string]interface{})
						if ok {
							for k, v := range FlattenType(s2, new_p) {
								out[k] = v
							}
						} else {
							log.Fatalf("expected type map, got something else instead. key=%s, s2=%s", key, s2)
						}
					} else {
						// array here contains non-objects, so just save element type and break
						// note JSON doesn't require arrays have uniform type, but we'll assume it does
						out[key] = "Array[ " + prettyPrintJsonType(d2) + " ]"
						break
					}
				}
			} else {
				log.Fatalf("expected type []interface{}, got something else instead. key=%s, s=%s", key, s)
			}

		} else {
			// got a basic type: Number, Boolean, or String
			out[new_p] = prettyPrintJsonType(value)
		}
	}
	return out
}

func prettyPrintJsonType(value interface{}) string {
	switch t := value.(type) {
	case float64:
		return "Number"
	case bool:
		return "Boolean"
	case string:
		return "String"
	default:
		log.Fatalf("unexpected type %T", t)
	}
	return "UNKNOWN"
}

// MESSAGE HANDLER FOR THE NSQ READER
type MessageHandler struct {
	msgChan  chan *nsq.Message
	stopChan chan int
}

func (self *MessageHandler) HandleMessage(message *nsq.Message, responseChannel chan *nsq.FinishedMessage) {
	self.msgChan <- message
	responseChannel <- &nsq.FinishedMessage{message.Id, 0, true}
}

type FlatMessage struct {
	data         map[string]string
	responseChan chan bool
}

// reads from nsq, flattens and types the event, and puts it on writeChan
func jsonFlattener(mh MessageHandler, writeChan chan FlatMessage) {
	for {
		select {
		case m := <-mh.msgChan:

			log.Printf("nsq msg= %s", m.Body)

			blob, err := simplejson.NewJson(m.Body)
			if err != nil {
				log.Fatalf(err.Error())
			}

			mblob, err := blob.Map()
			if err != nil {
				log.Fatalln(err)
			}

			flat := FlattenType(mblob, "")

			responseChan := make(chan bool)

			msg := FlatMessage{
				data:         flat,
				responseChan: responseChan,
			}

			writeChan <- msg

			success := <-responseChan
			if !success {
				log.Fatalf("its broken")
			} else {
				log.Println("flattener heard success on the responseChan")
			}
		}
	}
}

func store(flatChan chan FlatMessage) {

    typeStore := make(map[string]string)

	for {
		select{
        case flat := <-flatChan:
            for k, v := range flat.data{
                typeStore[k] = v
            }
        }
    }
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
    reqParams, err := url.ParseQuery(r.URL.RawQuery)
}

func main() {

	flag.Parse()

	r, err := nsq.NewReader(*topic, *channel)
	if err != nil {
		log.Fatal(err.Error())
	}

	mh := MessageHandler{
		msgChan:  make(chan *nsq.Message, 5),
		stopChan: make(chan int),
	}

	fc := make(chan FlatMessage)
	go jsonFlattener(mh, fc)
	go store(fc)
	r.AddAsyncHandler(&mh)

	err = r.ConnectToNSQ(*nsqTCPAddrs)
	if err != nil {
		log.Fatalf(err.Error())
	}
	err = r.ConnectToLookupd(*lookupdHTTPAddrs)
	if err != nil {
		log.Fatalf(err.Error())
	}

    http.HandleFunc("/get", GetHandler)
    go func() {
        log.Fatal(http.ListenAndServe(*httpAddress, nil))
    }()


	<-mh.stopChan

}
