package main

import (
	"net"
	"time"
	"strings"
	"strconv"
    "testing"
)

func TestRelaySet(t *testing.T) {
	ip1 := net.ParseIP("127.0.0.1")
	ip2 := net.ParseIP("127.0.0.2")
	relaySet := []*net.IP{ &ip1, &ip2 }
	relaySet  = calculateRelaySet( net.ParseIP("127.0.0.3"), relaySet )

	if len(relaySet) != 3 {
		t.Fail()
	}

	relaySet  = calculateRelaySet( net.ParseIP("127.0.0.4"), relaySet )

	if len(relaySet) > 3 {
		t.Fail()
	}

	relaySet  = calculateRelaySet( net.ParseIP("127.0.0.5"), relaySet )

	if len(relaySet) > 3 {
		t.Fail()
	}
}

func TestAssembleAggregate(t *testing.T) {
	dest := net.ParseIP("127.0.0.3")
	outcome := float32(1)
	observations := 1
	dad := net.ParseIP("127.0.0.2")
	me := net.ParseIP("127.0.0.1")
	tmo := 1000
	stamp := strings.Replace(myIP.String(), ".", "", -1) + "_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	payload := assembleAggregate( dest, outcome, observations, dad, me, tmo, stamp )
	// func assembleAggregate(dest net.IP, out float32, obs int, dad net.IP, me net.IP, tmo int, stamp string) Packet {

	if payload.Type != AggregateType {
		t.Fail()
	}

	if payload.Parent.String() != dad.String() || payload.Source.String() != me.String() {
		t.Fail()
	}

	if payload.Destination.String() != dest.String() {
		t.Fail()
	}

	if payload.Aggregate.Outcome != outcome || payload.Aggregate.Observations != observations {
		t.Fail()
	}


	gw := net.ParseIP("127.0.0.9")
	route := assembleRoute( gw, payload )

	if route.Type != RouteByGossipType {
		t.Fail()
	}

	if route.Parent.String() != dad.String() || route.Source.String() != me.String() {
		t.Fail()
	}

	if route.Destination.String() != dest.String() && route.Gateway.String() != gw.String() {
		t.Fail()
	}

	if route.Aggregate.Outcome != outcome || route.Aggregate.Observations != observations {
		t.Fail()
	}

	if route.Hops != 1 {
		t.Fail()
	}
}

func TestAssembleQuery(t *testing.T) {
	// (payloadIn Packet, dad net.IP, me net.IP, tmo float32) Packet {

	me := net.ParseIP("127.0.0.1")
	dad := net.ParseIP("127.0.0.2")
	me2 := net.ParseIP("127.0.0.100")
	tmo := 1000
	fct := "avg"

	query := Query{
            Function: fct,
            RelaySet: []*net.IP{},
        }

    prePayload := Packet{
        Type: StartType,
        Source: me,
        Timeout: tmo,
        Query: &query,
    }

	payload := assembleQuery( prePayload, dad, me )

	if payload.Type != QueryType {
		t.Fail()
	}

	if payload.Parent.String() != dad.String() || payload.Source.String() != me.String() {
		t.Fail()
	}

	if payload.Query.Function != fct {
		t.Fail()
	}

	if len(payload.Query.RelaySet) > 0 {
		t.Fail()
	}


	payload2 := assembleQuery( payload, me, me2 )
	if payload2.Type != QueryType {
		t.Fail()
	}

	if payload2.Parent.String() != me.String() || payload2.Source.String() != me2.String() {
		t.Fail()
	}

	if payload2.Query.Function != fct {
		t.Fail()
	}

	if len(payload2.Query.RelaySet) != 1 {
		t.Fail()
	}

}