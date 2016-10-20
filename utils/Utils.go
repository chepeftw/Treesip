package utils

import (
    "net"
    "bufio"
    "regexp"
    "strings"
    "os/exec"

    "github.com/op/go-logging"
)

// A Simple function to verify error
func CheckError(err error, log *logging.Logger) {
    if err  != nil {
        log.Error("Error: ", err)
    }
}

// Getting my own IP, first we get all interfaces, then we iterate
// discard the loopback and get the IPv4 address, which should be the eth0
func SelfieIP() net.IP {
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        panic(err)
    }

    for _, a := range addrs {
        if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
                return ipnet.IP
            }
        }
    }

    return net.ParseIP("127.0.0.1")
}

// Checking if a string is contained in a string list
// returns true if is contained, and false if not ... you kidding right?
func Contains(s []string, e string) bool {
    for _, a := range s {
        if a == e {
            return true
        }
    }
    return false
}

func RemoveFromList(del net.IP, list []net.IP) []net.IP {
    index := -1
    for i, b := range list {
        if b.Equal(del) {
            index = i
            break
        }
    }

    if index >= 0 {
        list = append(list[:index], list[index+1:]...)
    }

    return list
}

func ParseRoutes(log *logging.Logger) map[string]string {
    out, err := exec.Command("route", "-n").Output()
    CheckError(err, log)

    routes := make(map[string]string)
    scanner := bufio.NewScanner(strings.NewReader(string(out[:])))

    i := 0
    for scanner.Scan() {
        if i < 2 {
            i++
            continue
        }

        s := scanner.Text()
        re_leadclose_whtsp := regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
        re_inside_whtsp := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
        final := re_leadclose_whtsp.ReplaceAllString(s, "")
        final = re_inside_whtsp.ReplaceAllString(final, " ")

        arr := strings.Split(final, " ")
        // fmt.Println("Destination: %s - Gateway: %s", arr[0], arr[1])

        routes[arr[0]] = arr[1]
        // router <- "ADD|" + arr[0] + " " + arr[1]
    }

    if err := scanner.Err(); err != nil {
        log.Error("reading standard input: ", err)
    }

    return routes
}