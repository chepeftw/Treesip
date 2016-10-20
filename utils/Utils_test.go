package utils

import (
	"net"
	"regexp"
    "testing"

    "github.com/op/go-logging"
)

func TestSelfie(t *testing.T) {
	ValidIpAddressRegex := `^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`
	regEx, _ := regexp.Compile(ValidIpAddressRegex)
	selfie := SelfieIP().String()
	if !regEx.MatchString(selfie) {
		t.Fail()
	}
}

func TestContains(t *testing.T) {
	arr := []string{ "1", "2", "3"}
	tof := "1"

	if !Contains(arr, tof) {
		t.Fail()
	}

	tof = "4"
	if Contains(arr, tof) {
		t.Fail()
	}
}

func TestRemove(t *testing.T) {
	arr := []net.IP{ net.ParseIP("127.0.0.1") }
	local := net.ParseIP("127.0.0.1")

	arr = RemoveFromList(local, arr)
	if len(arr) != 0 {
		t.Fail()
	}

	arr = []net.IP{ net.ParseIP("127.0.0.2") }
	arr = RemoveFromList(local, arr)
	if len(arr) != 1 {
		t.Fail()
	}
}

func TestRoutes(t *testing.T) {
    log := logging.MustGetLogger("test")
	routes := ParseRoutes(log)

	if len(routes) != 0 {
		t.Fail()	
	}
}