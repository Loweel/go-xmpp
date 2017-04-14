package xmpp

import (
	"encoding/xml"
	"strings"
)

const (
	NSDiscoInfo  = "http://jabber.org/protocol/disco#info"
	NSDiscoItems = "http://jabber.org/protocol/disco#items"
)

// Service Discovery (XEP-0030) protocol. "Wraps" XMPP instance to provide a
// more convenient API for Disco clients.
type Disco struct {
	XMPP *XMPP
}

// IQ get/result payload for "info" requests.
type DiscoInfo struct {
	XMLName  xml.Name        `xml:"http://jabber.org/protocol/disco#info query"`
	Node     string          `xml:"node,attr"`
	Identity []DiscoIdentity `xml:"identity"`
	Feature  []DiscoFeature  `xml:"feature"`
}

// Identity
type DiscoIdentity struct {
	Category string `xml:"category,attr"`
	Type     string `xml:"type,attr"`
	Name     string `xml:"name,attr"`
}

// Feature
type DiscoFeature struct {
	Var string `xml:"var,attr"`
}

// IQ get/result payload for "items" requests.
type DiscoItems struct {
	XMLName xml.Name    `xml:"http://jabber.org/protocol/disco#items query"`
	Node    string      `xml:"node,attr"`
	Item    []DiscoItem `xml:"item"`
}

// Item.
type DiscoItem struct {
	JID  string `xml:"jid,attr"`
	Node string `xml:"node,attr"`
	Name string `xml:"name,attr"`
}

// Request information about the service identified by 'to'.
func (disco *Disco) Info(to, from string) (*DiscoInfo, error) {

	if from == "" {
		from = disco.XMPP.JID.Full()
	}

	req := &IQ{ID: UUID4(), Type: IQTypeGet, To: to, From: from}
	req.PayloadEncode(&DiscoInfo{})

	resp, err := disco.XMPP.SendRecv(req)
	if err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, resp.Error
	}

	info := &DiscoInfo{}
	resp.PayloadDecode(info)

	return info, err
}

// Request items in the service identified by 'to'.
func (disco *Disco) Items(to, from, node string) (*DiscoItems, error) {

	if from == "" {
		from = disco.XMPP.JID.Full()
	}

	req := &IQ{ID: UUID4(), Type: IQTypeGet, To: to, From: from}
	req.PayloadEncode(&DiscoItems{Node: node})

	resp, err := disco.XMPP.SendRecv(req)
	if err != nil {
		return nil, err
	} else if resp.Error != nil {
		return nil, resp.Error
	}

	items := &DiscoItems{}
	resp.PayloadDecode(items)

	return items, err
}

var discoNamespacePrefix = strings.Split(NSDiscoInfo, "#")[0]

// Matcher instance to match <iq/> stanzas with a disco payload.
var DiscoPayloadMatcher = MatcherFunc(
	func(v interface{}) bool {
		iq, ok := v.(*IQ)
		if !ok {
			return false
		}
		ns := strings.Split(iq.PayloadName().Space, "#")[0]
		return ns == discoNamespacePrefix
	},
)
