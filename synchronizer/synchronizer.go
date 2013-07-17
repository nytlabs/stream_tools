package main

import (
	"container/heap"
	"flag"
	"github.com/bitly/go-simplejson"
	"github.com/bitly/nsq/nsq"
	"log"
	"strconv"
	"time"
)

var (
	topic            = flag.String("topic", "", "nsq topic")
	channel          = flag.String("channel", "", "nsq topic")
	maxInFlight      = flag.Int("max-in-flight", 10, "max number of messages to allow in flight")
    nsqTCPAddrs      = flag.String("nsqd-tcp-address", "127.0.0.1:4150", "nsqd TCP address")
    nsqHTTPAddrs     = flag.String("nsqd-http-address", "127.0.0.1:4151", "nsqd HTTP address")
    lookupdHTTPAddrs = flag.String("lookupd-http-address", "127.0.0.1:4161", "lookupd HTTP address")
	lag_time         = flag.Int("lag", 10, "lag before emitting in seconds")
    timeKey          = flag.String("key","","key that holds time")
)


// MESSAGE HANDLER FOR THE NSQ READER
type MessageHandler struct {
	msgChan  chan *nsq.Message
	stopChan chan int
}

func (self *MessageHandler) HandleMessage(message *nsq.Message, responseChannel chan *nsq.FinishedMessage) {
	self.msgChan <- message
	responseChannel <- &nsq.FinishedMessage{message.Id, 0, true}
}

type WriteMessage struct {
	val          []byte
	t            time.Time
	responseChan chan bool
}

type PQMessage struct {
	val          []byte
	t            time.Time
	index        int
	killChan     chan bool
	responseChan chan bool
}

// PRIORITY QUEUE
// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*PQMessage

func (pq PriorityQueue) Len() int {
	return len(pq)
}

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].t.Before(pq[j].t)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PQMessage)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// update modifies the priority and value of an Item in the queue.
func (pq *PriorityQueue) update(item *PQMessage, val []byte, time time.Time) {
	heap.Remove(pq, item.index)
	item.val = val
	item.t = time
	heap.Push(pq, item)
}

func store(writeChan chan WriteMessage, pq *PriorityQueue, lag time.Duration) {

    var emit_time time.Time
    nextMsg := &PQMessage{
        t: time.Now(),
    }

    getNext := make(chan bool)

    emitter := time.AfterFunc(24 * 365 * time.Hour, func(){
        log.Println("...")
    })

    const layout = "2006-01-02 15:04:05 -0700"

    count := 0
    heapCount := 0

    for {
        select {
            case inMsg := <-writeChan:
                
                outMsg := &PQMessage{
                    val:          inMsg.val,
                    t:            inMsg.t,
                }
            
                outTime := outMsg.t.Add(lag)
                outDur := outTime.Sub(time.Now()) 

                if outDur > time.Duration(0 * time.Second) {
                    heap.Push(pq, outMsg) 
                    
                    heapCount ++

                    if heapCount % 500 == 0{
                        log.Println( "HEAP: " + strconv.Itoa(pq.Len()))
                    }

                    if outMsg.t.Before(nextMsg.t) {
                        heap.Push(pq, nextMsg)
                        nextMsg = heap.Pop(pq).(*PQMessage)
                        emit_time = nextMsg.t.Add(lag)
                        duration := emit_time.Sub(time.Now()) 

                        emitter.Stop()

                        emitter = time.AfterFunc(duration, func() {
                            count = count + 1
                            if count % 40 == 0 {
                                diff := nextMsg.t.Sub( time.Now() )
                                log.Println("POP: " + diff.String() + "IN QUEUE:" + strconv.Itoa(pq.Len()) )
                            }
                            getNext<- true
                        })
                    } 
                } else {
                    log.Println("error: " + outDur.String() + " message reads: " + outMsg.t.Format(layout) )

                }

                inMsg.responseChan <- true

            case <-getNext:
                if pq.Len() > 0 {
                    nextMsg = heap.Pop(pq).(*PQMessage) 
                    emit_time = nextMsg.t.Add(lag)
                    duration := emit_time.Sub(time.Now()) 

                    emitter = time.AfterFunc(duration, func() {
                        count = count + 1
                        if count % 40 == 0 {
                            diff := nextMsg.t.Sub( time.Now() )
                            log.Println("POP: " + diff.String() + "IN QUEUE:" + strconv.Itoa(pq.Len()) )
                        }
                        getNext<- true
                    })
                }
        }
    }
}


// function to read an NSQ channel and write to the key value store
func writer(mh MessageHandler, writeChan chan WriteMessage, timeKey string) {
	for {
		select {
		case m := <-mh.msgChan:

			blob, err := simplejson.NewJson(m.Body)

			if err != nil {
				log.Fatalf(err.Error())
			}

			msg_time, err := blob.Get(timeKey).Int64()

			if err != nil {
				log.Fatalf(err.Error())
			}

			t := time.Unix(0, msg_time * 1000 * 1000)
			mblob, err := blob.MarshalJSON()

			if err != nil {
				log.Fatalf(err.Error())
			}

			responseChan := make(chan bool)

			msg := WriteMessage{
				t:            t,
				val:          mblob,
				responseChan: responseChan,
			}

			writeChan <- msg

			success := <-responseChan

			if !success {
				// TODO learn about err.Error()
				log.Fatalf("its broken")
			}
		}
	}
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

	wc := make(chan WriteMessage)

	pq := &PriorityQueue{}
	heap.Init(pq)

	lag := time.Duration(time.Duration(*lag_time) * time.Second)

	go store(wc, pq, lag)
	go writer(mh, wc, *timeKey)

	r.AddAsyncHandler(&mh)
    r.SetMaxInFlight(*maxInFlight)

	err = r.ConnectToLookupd(*lookupdHTTPAddrs)
	if err != nil {
		log.Fatalf(err.Error())
	}

	<-mh.stopChan
}