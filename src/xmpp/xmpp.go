package xmpp

import (
	"fmt"
	"log"
	"sync"
)

// Handles XMPP conversations over a Stream. Use NewClientXMPP or
// NewComponentXMPP to create and configure a XMPP instance.
type XMPP struct {
	// JID associated with the stream. Note: this may be negotiated with the
	// server during setup and so must be used for all messages.
	JID JID
	stream *Stream

	// Stanza channels.
	in chan interface{}
	out chan interface{}

	// Incoming stanza filters.
	filterLock sync.Mutex
	nextFilterId FilterId
	filters []filter
}

func newXMPP(jid JID, stream *Stream) *XMPP {
	x := &XMPP{
		JID: jid,
		stream: stream,
		in: make(chan interface{}),
		out: make(chan interface{}),
	}
	go x.sender()
	go x.receiver()
	return x
}

// Send a stanza.
func (x *XMPP) Send(v interface{}) {
	x.out <- v
}

// Return the next stanza.
func (x *XMPP) Recv() (interface{}, error) {
	v := <-x.in
	if err, ok := v.(error); ok {
		return nil, err
	}
	return v, nil
}

func (x *XMPP) SendRecv(iq *Iq) (*Iq, error) {

	fid, ch := x.AddFilter(IqResult(iq.Id))
	defer x.RemoveFilter(fid)

	x.Send(iq)

	stanza := <-ch
	reply, ok := stanza.(*Iq)
	if !ok {
		return nil, fmt.Errorf("Expected Iq, for %T", stanza)
	}
	return reply, nil
}

type FilterId int64

func (fid FilterId) Error() string {
	return fmt.Sprintf("Invalid filter id: %d", fid)
}

func (x *XMPP) AddFilter(m Matcher) (FilterId, chan interface{}) {

	// Protect against concurrent access.
	x.filterLock.Lock()
	defer x.filterLock.Unlock()

	// Allocate chan and id.
	ch := make(chan interface{})
	id := x.nextFilterId
	x.nextFilterId ++

	// Insert at head of filters list.
	filters := make([]filter, len(x.filters)+1)
	filters[0] = filter{id, m, ch}
	copy(filters[1:], x.filters)
	x.filters = filters

	return id, ch
}

func (x *XMPP) RemoveFilter(id FilterId) error {

	// Protect against concurrent access.
	x.filterLock.Lock()
	defer x.filterLock.Unlock()

	// Find filter.
	for i, f := range x.filters {
		if f.id != id {
			continue
		}

		// Close the channel.
		close(f.ch)

		// Remove from list.
		filters := make([]filter, len(x.filters)-1)
		copy(filters, x.filters[:i])
		copy(filters[i:], x.filters[i+1:])
		x.filters = filters

		return nil
	}

	// Filter not found.
	return id
}

func IqResult(id string) Matcher {
	return MatcherFunc(
		func(v interface{}) bool {
			iq, ok := v.(*Iq)
			if !ok {
				return false
			}
			if iq.Id != id {
				return false
			}
			return true
		},
	)
}

// Interface used to test if a stanza matches some application-defined
// conditions.
type Matcher interface {
	// Return true if the stanza, v, matches.
	Match(v interface{}) (match bool)
}

// Adapter to allow a plain func to be used as a Matcher.
type MatcherFunc func(v interface{}) bool

// Implement Matcher by calling the adapted func.
func (fn MatcherFunc) Match(v interface{}) bool {
	return fn(v)
}

type filter struct {
	id FilterId
	m Matcher
	ch chan interface{}
}

func (x *XMPP) sender() {
	for v := range x.out {
		x.stream.Send(v)
	}
}

func (x *XMPP) receiver() {

	defer close(x.in)

	for {
		start, err := x.stream.Next()
		if err != nil {
			x.in <- err
			return
		}

		var v interface{}
		switch start.Name.Local {
		case "error":
			v = &Error{}
		case "iq":
			v = &Iq{}
		case "message":
			v = &Message{}
		case "presence":
			v = &Presence{}
		default:
			log.Fatal("Unexected element: %T %v", start, start)
		}

		err = x.stream.Decode(v, start)
		if err != nil {
			log.Fatal(err)
		}

		filtered := false
		for _, filter := range x.filters {
			if filter.m.Match(v) {
				filter.ch <- v
				filtered = true
			}
		}

		if !filtered {
			x.in <- v
		}
	}
}

// BUG(matt): Filter channels are not closed when the stream is closed.
