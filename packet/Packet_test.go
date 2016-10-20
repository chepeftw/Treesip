package packet

import (
	"net"
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
	tmo := float32(1000.0)
	payload := AssembleAggregate( dest, outcome, observations, dad, me, tmo )

	if payload.Type != AggregateType {
		t.Fail()
	}

	if payload.Parent.String() != dad.String() || payload.Source.String() != me.String() {
		t.Fail()
	}

	if payload.Aggregate.Destination.String() != dest.String() {
		t.Fail()
	}

	if payload.Aggregate.Outcome != outcome || payload.Aggregate.Observations != observations {
		t.Fail()
	}
}

func TestAssembleQuery(t *testing.T) {
	// (payloadIn Packet, dad net.IP, me net.IP, tmo float32) Packet {

	me := net.ParseIP("127.0.0.1")
	dad := net.ParseIP("127.0.0.2")
	me2 := net.ParseIP("127.0.0.100")
	tmo := float32(1000.0)
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

	payload := AssembleQuery( prePayload, dad, me )

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


	payload2 := AssembleQuery( payload, me, me2 )
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