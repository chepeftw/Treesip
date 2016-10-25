package main
 
import (
    "os"
    "fmt"
    "net"
    "time"
    "strings"
    "strconv"
    "math/rand"
    "encoding/json"

    "github.com/op/go-logging"
)


// +++++++++++++++++++++++++++
// +++++++++ Go-Logging Conf
// +++++++++++++++++++++++++++
var log = logging.MustGetLogger("treesip")

var format = logging.MustStringFormatter(
    "%{level:.4s}=> %{time:0102 15:04:05.999} %{shortfile} %{message}",
)


// +++++++++ Constants
const (
    Port              = ":10001"
    Protocol          = "udp"
    BroadcastAddr     = "255.255.255.255"
    LocalhostAddr     = "127.0.0.1"
)

const (
    INITIAL = iota
    Q1
    Q2
    A1
    A2
    A3
)

// +++++++++ Global vars
var state = INITIAL
var myIP net.IP = net.ParseIP(LocalhostAddr)
var parentIP net.IP = net.ParseIP(LocalhostAddr)
var timeout int = 0

var queryACKlist []net.IP = []net.IP{}
var timer *time.Timer

var accumulator float32 = 0
var observations int = 0
var rootNode bool = false
var startTime int64 = 0

var globalNumberNodes int = 0
var externalTimeout int = 0
var globalCounter int = 0
var electionNode string = ""
var runMode string = ""

var s1 = rand.NewSource(time.Now().UnixNano())
var r1 = rand.New(s1)

// +++++++++ Routing Protocol
var routes map[string]string = make(map[string]string)
var RouterWaitRoom map[string]Packet = make(map[string]Packet)
var RouterWaitCount map[string]int = make(map[string]int)
var ForwardedMessages []string = []string{}
var ReceivedMessages []string = []string{}

// +++++++++ Channels
var buffer = make(chan string)
var output = make(chan string)
var router = make(chan string)
var done = make(chan bool)


func StartTimer() {
    timer = startTimeout(timeout, timer, r1)

    go func() {
        <- timer.C
        js, err := json.Marshal(assembleTimeout())
        checkError(err, log)
        buffer <- string(js)
        log.Debug("Timer expired")
    }()
}
func StopTimer() {
    stopTimeout(timer)
}


func SendHello(stamp string) {
    SendMessage( assembleHello(myIP, stamp) )
}
func SendHelloReply(payload Packet) {
    SendMessage( assembleHelloReply(payload, myIP) )
}
func SendRoute(gateway net.IP, payloadIn Packet) {
    SendMessage( assembleRoute(gateway, payloadIn) )
}
func SendQuery(payload Packet) {
    SendMessage( assembleQuery(payload, parentIP, myIP) )
}
func SendAggregate(destination net.IP, outcome float32, observations int) {
    SendMessage( helperAggregatePacket( destination, outcome, observations ) )
}
func helperAggregatePacket(destination net.IP, outcome float32, observations int) Packet {
    stamp := strings.Replace(myIP.String(), ".", "", -1) + "_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
    return assembleAggregate(destination, outcome, observations, parentIP, myIP, timeout, stamp)
}
func SendMessage(payload Packet) {
    js, err := json.Marshal(payload)
    checkError(err, log)
    output <- string(js)
}

func LogSuccess() {
    log.Info(
        myIP.String() + 
        " => State: A1, **DONE**, Result: " + 
        strconv.FormatFloat( float64(accumulator), 'f', 6, 32) + 
        " and Observations:" + 
        strconv.Itoa(observations))

    log.Info( myIP.String() + " CONVERGENCE_TIME=" + strconv.FormatInt( (time.Now().UnixNano() - startTime) / int64(time.Millisecond), 10 ))
}

// Function that handles the output channel
func attendOutputChannel() {
    ServerAddr,err := net.ResolveUDPAddr(Protocol, BroadcastAddr+Port)
    checkError(err, log)
    LocalAddr, err := net.ResolveUDPAddr(Protocol, myIP.String()+":0")
    checkError(err, log)
    Conn, err := net.DialUDP(Protocol, LocalAddr, ServerAddr)
    checkError(err, log)
    defer Conn.Close()

    for {
        j, more := <-output
        if more {
            if Conn != nil {
                buf := []byte(j)
                _,err = Conn.Write(buf)
                // log.Debug( myIP.String() + " " + j + " MESSAGE_SIZE=" + strconv.Itoa(len(buf)) )
                log.Info( myIP.String() + " MESSAGE_SIZE=" + strconv.Itoa(len(buf)) )
                log.Info( myIP.String() + " SENDING_MESSAGE=1" )
                checkError(err, log)
            }
        } else {
            fmt.Println("closing channel")
            done <- true
            return
        }
    }
}



func CleanupTheHouse() {
    state = INITIAL
    parentIP = net.ParseIP(LocalhostAddr)
    timeout = 0

    queryACKlist = []net.IP{}
    StopTimer()

    accumulator = 0
    observations = 0
    rootNode = false
    startTime = 0

    go selectLeaderOfTheManet()
}


// Function that handles the buffer channel
func attendBufferChannel() {
fsm := true
for {
    j, more := <-buffer
    if more {
        // First we take the json, unmarshal it to an object
        payload := Packet{}
        json.Unmarshal([]byte(j), &payload)

        // Exclusive to the gossip routing protocol
        fsm = false
        if payload.Type == HelloType {
            if myIP.String() != payload.Source.String() {
                if contains(ForwardedMessages, payload.Timestamp) {
                    time.Sleep(time.Duration((r1.Intn(19000)+1000)/100) * time.Millisecond)
                }
                SendHelloReply(payload)
                log.Debug(myIP.String() + " => Sending HELLO_REPLY to " + payload.Source.String())
            }

        } else if payload.Type == HelloReplyType {
            if myIP.String() == payload.Destination.String() {
                stamp := payload.Timestamp

                if _, ok := RouterWaitCount[stamp]; ok {
                    if ( RouterWaitCount[stamp] == 1 && payload.Source.String() == RouterWaitRoom[stamp].Destination.String() ) || RouterWaitCount[stamp] == 0 {
                        SendRoute(payload.Source, RouterWaitRoom[stamp])
                        ForwardedMessages = append(ForwardedMessages, stamp)
                        if len(ForwardedMessages) > 100 {
                            ForwardedMessages = ForwardedMessages[len(ForwardedMessages)-100:]
                        }
                        // delete(RouterWaitRoom, stamp)
                        RouterWaitCount[stamp] = 1

                        log.Debug(myIP.String() + " => HELLO_REPLY from " + payload.Source.String())
                    }
                }
            }

        } else if payload.Type == RouteByGossipType {
            stamp := payload.Timestamp
            if myIP.String() == payload.Gateway.String() {
                if myIP.String() != payload.Destination.String() {
                    RouterWaitRoom[stamp] = payload

                    SendHello(stamp)

                    log.Debug(myIP.String() + " => Routing from " + payload.Source.String())
                       
                } else {
                    if !contains(ReceivedMessages, stamp) {
                        fsm = true

                        ReceivedMessages = append(ReceivedMessages, stamp)
                        if len(ReceivedMessages) > 100 {
                            ReceivedMessages = ReceivedMessages[len(ReceivedMessages)-100:]
                        }

                        log.Debug(myIP.String() + " SUCCESS ROUTE -> stamp: " + stamp +" from " + payload.Source.String() + " after " + strconv.Itoa(payload.Hops) + " hops")
                        log.Debug(myIP.String() + " => " + j)
                        log.Info(myIP.String() + " => SUCCESS_ROUTE=1")
                    } else {
                        log.Info(myIP.String() + " => SUCCESS_AGAIN_ROUTE=1")
                    }
                }
            }
        } else {
            fsm = true
        }



        // Now we start! FSM TIME!
        if fsm {
        switch state {
        case INITIAL:
            // RCV start() -> SND Query
            if payload.Type == StartType {
                startTime = time.Now().UnixNano() // Start time of the monitoring process
            }

            if payload.Type == StartType || payload.Type == QueryType {
                state = Q1 // Moving to Q1 state
                parentIP = payload.Source
                timeout = payload.Timeout

                SendQuery(payload)
                StartTimer()

                log.Debug(myIP.String() + " => State: INITIAL, RCV Query -> SND Query")
                log.Info( myIP.String() + " => START_QUERY=1")

            }
        break
        case Q1: 
            // RCV QueryACK -> acc(ACK_IP)
            if payload.Type == QueryType && 
                    payload.Parent.Equal(myIP) && !payload.Source.Equal(myIP) {

                state = Q2
                queryACKlist = append(queryACKlist, payload.Source)

                StopTimer()

                log.Debug( myIP.String() + " => State: Q1, RCV QueryACK -> acc( " + payload.Source.String() + " )-> " + strconv.Itoa( len( queryACKlist ) ) )

            } else if payload.Type == TimeoutType { // timeout()/edgeNode() -> SND Aggregate
                state = A1

                // Just one outcome and 1 observation because it should be the end of a branch
                accumulator = functionValue(accumulator)
                observations = 1
                log.Debug( myIP.String() + " => OBSERVATIONS=1" )
                SendAggregate(parentIP, accumulator, observations)
                StartTimer()

                log.Debug( myIP.String() + " => State: Q1, timeout() -> SND Aggregate")
            }
        break
        case Q2:
            // RCV QueryACK -> acc(ACK_IP)
            if payload.Type == QueryType && 
                    payload.Parent.Equal(myIP) && 
                    !payload.Source.Equal(myIP) {

                state = Q2 // loop to stay in Q2
                queryACKlist = append(queryACKlist, payload.Source)

                log.Debug( myIP.String() + " => State: Q2, RCV QueryACK -> acc( " + payload.Source.String() + " ) -> " + strconv.Itoa( len( queryACKlist ) ))

            } else if ( payload.Type == AggregateType || 
                    payload.Type == RouteByGossipType ) && 
                    payload.Destination.Equal(myIP) { // RCV Aggregate -> SND Aggregate 

                // not always but yes
                // I check that the parent it is itself, that means that he already stored this guy
                // in the queryACKList
                state = A1
                queryACKlist = removeFromList(payload.Source, queryACKlist)

                StopTimer()
                accumulator, observations  = aggregateValue( payload.Aggregate.Outcome, payload.Aggregate.Observations, accumulator, observations)

                if len(queryACKlist) == 0 {
                    accumulator = functionValue(accumulator)
                    observations = observations + 1
                    log.Debug( myIP.String() + " => OBSERVATIONS=1" )
                    SendAggregate(parentIP, accumulator, observations)

                    StartTimer()
                }

                log.Debug( myIP.String() + " => State: Q2, RCV Aggregate -> SND Aggregate remove " + payload.Source.String() + " -> " + strconv.Itoa( len( queryACKlist ) ))
            }
        break
        case A1:
            // RCV Aggregate -> SND Aggregate // not always but yes
            // I check that the parent it is itself, that means that he already stored this guy
            // in the queryACKList
            if ( payload.Type == AggregateType || 
                    payload.Type == RouteByGossipType ) && 
                    payload.Destination.Equal(myIP) {

                if containsIP(queryACKlist, payload.Source) {

                    if payload.Type == RouteByGossipType {
                            log.Debug( myIP.String() + " Incoming routing => " + j)
                    }
                    
                    state = A1
                    queryACKlist = removeFromList(payload.Source, queryACKlist)

                    StopTimer()
                    accumulator, observations  = aggregateValue( payload.Aggregate.Outcome, payload.Aggregate.Observations, accumulator, observations)

                    log.Debug( myIP.String() + " => State: A1, RCV Aggregate & loop() -> SND Aggregate " + payload.Source.String() + " -> " + strconv.Itoa(len(queryACKlist)))

                    if len(queryACKlist) == 0 {
                        accumulator = functionValue(accumulator)
                        observations = observations + 1
                        log.Debug( myIP.String() + " => OBSERVATIONS=1" )

                        SendAggregate(parentIP, accumulator, observations)
                        // StartTimer()

                        log.Debug("if len(queryACKlist) == 0")

                        if rootNode { // WE ARE DONE!!!!
                            LogSuccess() // Suuuuuuucceeeeess!!!
                            CleanupTheHouse()
                        }
                    } else {
                        // StartTimer()
                    }

                }

            } else if payload.Type == AggregateType && 
                    payload.Source.Equal(parentIP) { // RCV AggregateACK -> done()

                log.Debug( myIP.String() + " => State: A1, RCV Aggregate -> done()")
                CleanupTheHouse()

            } else if payload.Type == TimeoutType { // timeout -> SND AggregateRoute // not today
                state = A2 // it should do this, but not today
                
                payloadRefurbish := helperAggregatePacket( parentIP, accumulator, observations )
                RouterWaitRoom[payloadRefurbish.Timestamp] = payloadRefurbish
                RouterWaitCount[payloadRefurbish.Timestamp] = 0
                SendHello(payloadRefurbish.Timestamp)

                log.Debug( myIP.String() + " => State: A1, timeout() -> SND AggregateRoute")
                log.Debug( myIP.String() + " => " + string(j) )

                // if rootNode { // Just to show something
                //     LogSuccess() // Suuuuuuucceeeeess!!!
                // }
                // CleanupTheHouse()
            }
        break
        case A2:
            // Not happening bro!
        break
        default:
            // Welcome to Stranger Things ... THIS REALLY SHOULD NOT HAPPEN
        break
        }
        }

    } else {
        log.Debug("closing channel")
        done <- true
        return
    }
}
}

// This function selects the Root node, the source, the NEO!
// The idea is that if the run mode is "single", it is intended 
// to run only one time (mind blowing), so it will run, it will converge and that's it.
// If the run mode is different ("continuous"), it means that the same configuration 
// should apply but it will run multiple times to get multiple measures with the same "infrastructure"
// To make multiple runs faster and efficient for data collection.
func selectLeaderOfTheManet() {
    // The root, the source, the neo, the parent of the MANET, you name it
    neo := electionNode

    // Please note that the first part of this if is IF IS NOT runmode single.
    // In other words if the runmode is single, it does not do anything special or significative,
    // But assumes that a "electionNode" was configured manually from the outside.
    // First we transform the "global counter", which is the number of the node manually
    // elected as the root node, into an actual IP.
    if runMode != "single" {
        if globalNumberNodes != 0 {
            if globalCounter == globalNumberNodes {
                return
            }

            // This allows me a maximum number of 62500 nodes. (250*250)
            s3 := int(globalCounter/250)
            s4 := int(globalCounter%250)+1

            neo = "10.12." + strconv.Itoa(s3) + "." + strconv.Itoa(s4)
            globalCounter = globalCounter + 1
        }
    } else {
        if globalCounter > 0 {
            return
        }

        globalCounter = globalCounter + 1
    }


    // If I AM NEO ... send the first message.
    if myIP.String() == neo {
        rootNode = true

        query := Query{
                Function: "avg",
                RelaySet: []*net.IP{},
            }

        calculatedTimeout := 800
        if externalTimeout > 0 {
            calculatedTimeout = externalTimeout
        }

        payload := Packet{
            Type: StartType,
            Source: myIP,
            Timeout: calculatedTimeout,
            Query: &query,
        }

        log.Info("The leader has been chosen!!! All hail the new KING!!! " + neo)
        time.Sleep(time.Second * 3)

        js, err := json.Marshal(payload)
        checkError(err, log)
        log.Debug("Initial JSON " + string(js))
        buffer <- string(js)
    }
}
// ------------
 
func main() {

    if nnodes := os.Getenv("NNODES"); nnodes != "" {
        globalNumberNodes, _ = strconv.Atoi( nnodes )
    }
    if ntimeout := os.Getenv("NTIMEOUT"); ntimeout != "" {
        externalTimeout, _ = strconv.Atoi( ntimeout )
    }
    if rootn := os.Getenv("ROOTN"); rootn != "" {
        electionNode = rootn
    }
    if fsmmode := os.Getenv("FSMMODE"); fsmmode != "" {
        runMode = fsmmode
    }
    targetSync := float64(0)
    if tsync := os.Getenv("TARGETSYNC"); tsync != "" {
        targetSync, _ = strconv.ParseFloat(tsync, 64)
    }


    // Logger configuration
    var logPath = "/var/log/golang/"
    if _, err := os.Stat(logPath); os.IsNotExist(err) {
        os.MkdirAll(logPath, 0777)
    }

    var logFile = logPath + "treesip.log"
    f, err := os.OpenFile(logFile, os.O_APPEND | os.O_CREATE | os.O_RDWR, 0666)
    if err != nil {
        fmt.Printf("error opening file: %v", err)
    }
    defer f.Close()

    backend := logging.NewLogBackend(f, "", 0)
    backendFormatter := logging.NewBackendFormatter(backend, format)
    backendLeveled := logging.AddModuleLevel(backendFormatter)
    backendLeveled.SetLevel(logging.DEBUG, "")
    logging.SetBackend(backendLeveled)
    log.Info("")
    log.Info("")
    log.Info("")
    log.Info("------------------------------------------------------------------------")
    log.Info("")
    log.Info("")
    log.Info("")
    log.Info("Starting Treesip process, waiting some time to get my own IP...")
    // ------------

    // It gives some time for the network to get configured before it gets its own IP.
    // This value should be passed as a environment variable indicating the time when
    // the simulation starts, this should be calculated by an external source so all
    // Go programs containers start at the same UnixTime.
    now := float64(time.Now().Unix())
    sleepTime := 0
    if( targetSync > now ) {
        sleepTime = int(targetSync - now)
        log.Info("SYNC: Sync time is " + strconv.FormatFloat( targetSync, 'f', 6, 64) )
    } else {
        sleepTime = globalNumberNodes
    }
    log.Info("SYNC: sleepTime is " + strconv.Itoa(sleepTime))
    time.Sleep(time.Second * time.Duration(sleepTime))
    // ------------

    // But first let me take a selfie, in a Go lang program is getting my own IP
    myIP = selfieIP();
    log.Info("Good to go, my ip is " + myIP.String())

    // Lets prepare a address at any address at port 10001
    ServerAddr,err := net.ResolveUDPAddr(Protocol, Port)
    checkError(err, log)
 
    // Now listen at selected port
    ServerConn, err := net.ListenUDP(Protocol, ServerAddr)
    checkError(err, log)
    defer ServerConn.Close()

    // Run the FSM! The one in charge of everything
    go attendBufferChannel()
    // Run the Output! The channel for communicating with the outside world!
    go attendOutputChannel()
    // Run the election of the leader!
    go selectLeaderOfTheManet()
 
    buf := make([]byte, 1024)
 
    for {
        n,_,err := ServerConn.ReadFromUDP(buf)
        buffer <- string(buf[0:n])
        checkError(err, log)
    }

    close(buffer)
    close(output)

    <-done
}