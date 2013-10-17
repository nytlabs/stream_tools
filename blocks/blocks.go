package blocks

import (
	"github.com/bitly/go-simplejson"
)

// Block is the basic interface for processing units in streamtools
type Block interface {
	// BlockRoutine is the central processing routine for a block. All the work gets done in here
	BlockRoutine()
	// init routines TODO should just be init
	InitOutChans()
	// a set of accessors are provided so that a block creator can access certain aspects of a block
	GetID() string
	GetBlockType() string
	GetInChan() chan *simplejson.Json
	getOutChans() map[string]chan *simplejson.Json
	GetRouteChan(string) chan RouteResponse
	GetRoutes() []string
	// some aspects of a block can also be set by the block creator
	SetInChan(chan *simplejson.Json)
	SetID(string)
	SetOutChan(string, chan *simplejson.Json)
	CreateOutChan(string) chan *simplejson.Json
}

// The AbstractBlock struct defines the attributes a block must have
type AbstractBlock struct {
	// the ID is the unique key by which streamtools refers to the block
	ID string
	// blockType defines what kind of block this
	blockType string
	// the inChan passes messages from elsewhere into this block
	inChan chan *simplejson.Json
	// the outChan sends messages from this block elsewhere
	outChans map[string]chan *simplejson.Json
	// the routes map is used to define arbitrary streamtools endpoints for this block
	routes map[string]chan RouteResponse
}

// RouteResponse is passed into a block to query via established handlers
type RouteResponse struct {
	Msg          *simplejson.Json
	ResponseChan chan *simplejson.Json
}

// SIMPLE GETTERS AND SETTERS

func (self *AbstractBlock) GetID() string {
	return self.ID
}

func (self *AbstractBlock) GetBlockType() string {
	return self.blockType
}

func (self *AbstractBlock) GetInChan() chan *simplejson.Json {
	return self.inChan
}

func (self *AbstractBlock) getOutChans() map[string]chan *simplejson.Json {
	return self.outChans
}

// ROUTES

// returns a channel specified by an endpoint name
func (self *AbstractBlock) GetRouteChan(name string) chan RouteResponse {
	if val, ok := self.routes[name]; ok {
		return val
	}
	// TODO return a proper error on this if key is not found.
	return nil
}

// GetRoutes returns all of the route names specified by the block
func (self *AbstractBlock) GetRoutes() []string {
	routeNames := make([]string, len(self.routes))
	i := 0
	for name, _ := range self.routes {
		routeNames[i] = name
		i += 1
	}
	return routeNames
}

func (self *AbstractBlock) SetInChan(inChan chan *simplejson.Json) {
	self.inChan = inChan
}

func (self *AbstractBlock) SetID(id string) {
	self.ID = id
}

func (self *AbstractBlock) SetOutChan(toBlockID string, outChan chan *simplejson.Json) {
	self.outChans[toBlockID] = outChan
}

func (self *AbstractBlock) CreateOutChan(toBlockID string) chan *simplejson.Json {
	outChan := make(chan *simplejson.Json)
	self.outChans[toBlockID] = outChan
	return outChan
}

func (self *AbstractBlock) InitOutChans() {
	self.outChans = make(map[string]chan *simplejson.Json)
}
