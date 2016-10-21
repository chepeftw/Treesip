package packet

import (
    "net"
    // "github.com/op/go-logging"
)


// +++++++++ Constants
const (
    StartType         = "start"
    TimeoutType       = "timeout"
    QueryType         = "query"
    AggregateType     = "aggregate"
    AggregateFwdType  = "aggregateForward"
    AggregateRteType  = "aggregateRoute"

    HelloType         = "hello"
    HelloReplyType    = "helloReply"
    RouteByTableType  = "routeByTable"
    RouteByGossipType = "routeByGossip"
)


// +++++++++ Packet structure
type Packet struct {
    Type      string        `json:"tp"`
    Parent    net.IP        `json:"prnt"`
    Source    net.IP        `json:"src"`
    Timeout   int           `json:"tmo"`
    Query     *Query        `json:"qry,omitempty"`
    Aggregate *Aggregate    `json:"agt,omitempty"`

    Timestamp    string     `json:"ts,omitempty"`
    TimeToLive   int        `json:"ttl,omitempty"`
    Hops         int        `json:"hps,omitempty"`
}

type Query struct {
    Function  string    `json:"fct,omitempty"`
    RelaySet  []*net.IP `json:"rSt,omitempty"`
}

type Aggregate struct {
    Destination  net.IP `json:"dst,omitempty"`
    Gateway      net.IP `json:"gw,omitempty"`
    Outcome      float32 `json:"otc,omitempty"`
    Observations int    `json:"obs,omitempty"`
}


// Function to calculate the RelaySet
// the idea is to always have 3 elements. Seen as a tree, the closer 3 parents.
func calculateRelaySet( newItem net.IP, receivedRelaySet []*net.IP ) []*net.IP {
    slice := []*net.IP{&newItem}

    if len(receivedRelaySet) < 3 {
        return append(slice, receivedRelaySet...)
    }

    return append(slice, receivedRelaySet[:len(receivedRelaySet)-1]...)
}


func AssembleTimeout() Packet {
    payload := Packet{
        Type: TimeoutType
    }

    return payload
}

func AssembleAggregate(dest net.IP, out float32, obs int, dad net.IP, me net.IP, tmo int) Packet {
    aggregate := Aggregate{
            Destination: dest,
            Outcome: out,
            Observations: obs,
        }

    payload := Packet{
        Type: AggregateType,
        Parent: dad,
        Source: me,
        Timeout: tmo,
        Aggregate: &aggregate,
    }

    return payload
}

func AssembleQuery(payloadIn Packet, dad net.IP, me net.IP) Packet {
	relaySet := []*net.IP{}
	if payloadIn.Type != StartType {
        relaySet = calculateRelaySet(payloadIn.Source, payloadIn.Query.RelaySet)
    }

    query := Query{
            Function: payloadIn.Query.Function,
            RelaySet: relaySet,
        }

    payload := Packet{
        Type: QueryType,
        Parent: dad,
        Source: me,
        Timeout: payloadIn.Timeout,
        Query: &query,
    }

    return payload
}