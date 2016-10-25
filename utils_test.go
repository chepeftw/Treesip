package main

import (
	"net"
	"regexp"
    "testing"
    // "github.com/op/go-logging"
)

func TestSelfie(t *testing.T) {
	ValidIpAddressRegex := `^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`
	regEx, _ := regexp.Compile(ValidIpAddressRegex)
	selfie := selfieIP().String()
	if !regEx.MatchString(selfie) {
		t.Fail()
	}
}

func TestContains(t *testing.T) {
	arrNil := []string{}
	tofNil := ""

	if contains(arrNil, tofNil) {
		t.Fail()
	}

	arr := []string{ "1", "2", "3"}
	tof := "1"

	if !contains(arr, tof) {
		t.Fail()
	}

	tof = "4"
	if contains(arr, tof) {
		t.Fail()
	}

	arrIP := []net.IP{ net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2")}
	tofIP := net.ParseIP("127.0.0.2")

	if !containsIP(arrIP, tofIP) {
		t.Fail()
	}

	tofIP = net.ParseIP("127.0.0.3")
	if containsIP(arrIP, tofIP) {
		t.Fail()
	}
}

func TestRemove(t *testing.T) {
	arr := []net.IP{ net.ParseIP("127.0.0.1") }
	local := net.ParseIP("127.0.0.1")

	arr = removeFromList(local, arr)
	if len(arr) != 0 {
		t.Fail()
	}

	arr = []net.IP{ net.ParseIP("127.0.0.2") }
	arr = removeFromList(local, arr)
	if len(arr) != 1 {
		t.Fail()
	}
}

// func TestRoutes(t *testing.T) {
//     log := logging.MustGetLogger("test")
// 	routes := parseRoutes(log)

// 	if len(routes) != 0 {
// 		t.Fail()	
// 	}
// }