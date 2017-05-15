package main
 
import (
    "os"
    "fmt"
    "net"
    "time"
    "math"
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
var level int = 0

var maxTreeTimeout int = 6000

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

// My intention is to have 0 for gossip routing and 1 for OLSR
var routingMode = 0

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

func StartTreeTimer() {
    treeTimeout := float32(maxTreeTimeout) * float32(math.Pow( 0.75, float64(level) ))
    StartTimerStar(treeTimeout)
}

func StartTimer() {
    StartTimerStar( float32(timeout) )
}

func StartTimerStar(localTimeout float32) {
    stopTimeout(timer)
    timer = startTimeoutF(localTimeout, r1)

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

func StartTimerHello(stamp string) {
    timerHello := startTimeout(timeout*2, r1)

    <- timerHello.C
    js, err := json.Marshal(assembleTimeoutHello(stamp))
    checkError(err, log)
    buffer <- string(js)
    log.Debug("TimerHello Expired " + stamp)
}
// func StopTimerHello() {
//     stopTimeout(timerHello)
//     log.Debug("TimerHello Stopped")
// }


func SendHello(stamp string) {
    SendMessage( assembleHello(myIP, stamp) )
    go StartTimerHello(stamp)
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
    log.Info( myIP.String() + " RESULT="+strconv.Itoa(observations))
    log.Info( myIP.String() + " GLOBAL_TIMEOUT=1")
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
                log.Debug( myIP.String() + " " + j + " MESSAGE_SIZE=" + strconv.Itoa(len(buf)) )
                // log.Info( myIP.String() + " MESSAGE_SIZE=" + strconv.Itoa(len(buf)) )
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
    level = 0

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
        if payload.Type == HelloType &&
            !compareIPs( myIP, payload.Source) {
            
            if contains(ForwardedMessages, payload.Timestamp) {
                time.Sleep(time.Duration((r1.Intn(19000)+1000)/100) * time.Millisecond)
            }
            SendHelloReply(payload)
            log.Debug(myIP.String() + " => _HELLO to " + payload.Source.String())

        } else if payload.Type == HelloTimeoutType { // HELLO TIMEOUT
            if !contains(ForwardedMessages, payload.Timestamp) {
                SendHello(payload.Timestamp)

                log.Debug(myIP.String() + " => HELLO_TIMEOUT ON TIME" + payload.Timestamp)
            } else {
                log.Debug(myIP.String() + " => HELLO_TIMEOUT delayed " + payload.Timestamp)
            }

        } else if payload.Type == HelloReplyType &&
                    compareIPs( myIP, payload.Destination ) {

            stamp := payload.Timestamp

            if _, ok := RouterWaitCount[stamp]; ok {

                // Splitting the SendRoute in before ifs to improve performance
                if RouterWaitCount[stamp] == 0 {
                    SendRoute(payload.Source, RouterWaitRoom[stamp])
                }

                if RouterWaitCount[stamp] == 1 && compareIPs(payload.Source, RouterWaitRoom[stamp].Destination) {
                    SendRoute(payload.Source, RouterWaitRoom[stamp])
                }

                if ( RouterWaitCount[stamp] == 1 && compareIPs(payload.Source, RouterWaitRoom[stamp].Destination) ) || RouterWaitCount[stamp] == 0 {
                    // StopTimerHello()
                    SendRoute(payload.Source, RouterWaitRoom[stamp])
                    ForwardedMessages = appendToList(ForwardedMessages, stamp)
                    // delete(RouterWaitRoom, stamp)
                    RouterWaitCount[stamp] = 1

                    log.Debug(myIP.String() + " => HELLO_REPLY WIN from " + payload.Source.String())
                } else {
                    log.Debug(myIP.String() + " => HELLO_REPLY FAIL from " + payload.Source.String())
                }
            } else {
                log.Debug(myIP.String() + " => HELLO_REPLY NOT IN RouterWaitRoom from " + payload.Source.String())
            }

        } else if payload.Type == RouteByGossipType {
            stamp := payload.Timestamp
            if compareIPs( myIP, payload.Gateway ) && !compareIPs( myIP, payload.Destination ) {
    
                if routingMode == 0 {
                    RouterWaitRoom[stamp] = payload
                    SendHello(stamp)
                } else if routingMode == 1 {
                    routes = parseRoutes(log)
                    SendRoute(net.ParseIP(routes[payload.Destination.String()]), payload)
                }

                log.Debug(myIP.String() + " => ROUTE from " + payload.Source.String() + " to " + payload.Destination.String())
                    
            } else if compareIPs( myIP, payload.Gateway ) && compareIPs( myIP, payload.Destination ) {

                if (routingMode == 0 && !contains(ReceivedMessages, stamp)) || (routingMode == 1) {
                    fsm = true

                    ReceivedMessages = appendToList(ReceivedMessages, stamp)

                    log.Debug(myIP.String() + " SUCCESS ROUTE -> stamp: " + stamp +" from " + payload.Source.String() + " after " + strconv.Itoa(payload.Hops) + " hops")
                    log.Debug(myIP.String() + " => " + j)
                    log.Info(myIP.String() + " => SUCCESS_ROUTE=1")
                } else if routingMode == 0 && contains(ReceivedMessages, stamp) {
                    log.Info(myIP.String() + " => SUCCESS_AGAIN_ROUTE=1")
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
                level = payload.Level

                SendQuery(payload)
                StartTimer()

                log.Debug(myIP.String() + " => State: INITIAL, RCV Query -> SND Query")
                log.Debug( myIP.String() + " => START_QUERY=1")
                log.Debug( myIP.String() + " => LEVEL=" + strconv.Itoa(level))

            }
        break
        case Q1: 
            // RCV QueryACK -> acc(ACK_IP)
            if payload.Type == QueryType && 
                    compareIPs(myIP, payload.Parent) && 
                    !compareIPs(myIP, payload.Source) {

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
                    compareIPs(myIP, payload.Parent) && 
                    !compareIPs(myIP, payload.Source) {

                state = Q2 // loop to stay in Q2
                queryACKlist = append(queryACKlist, payload.Source)

                log.Debug( myIP.String() + " => State: Q2, RCV QueryACK -> acc( " + payload.Source.String() + " ) -> " + strconv.Itoa( len( queryACKlist ) ))

            } else if ( payload.Type == AggregateType || 
                    payload.Type == RouteByGossipType ) && 
                    compareIPs(myIP, payload.Destination) { // RCV Aggregate -> SND Aggregate 

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

                    StartTreeTimer()
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
                    compareIPs(myIP, payload.Destination) {

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
                        StartTreeTimer()

                        log.Debug("if len(queryACKlist) == 0")

                        if rootNode { // WE ARE DONE!!!!
                            LogSuccess() // Suuuuuuucceeeeess!!!
                            CleanupTheHouse()
                        }
                    } else {
                        StartTreeTimer()
                    }

                }

            } else if payload.Type == AggregateType && 
                    compareIPs(parentIP, payload.Source) { // RCV AggregateACK -> done()

                log.Debug( myIP.String() + " => State: A1, RCV Aggregate -> done()")
                CleanupTheHouse()

            } else if payload.Type == TimeoutType { // timeout -> SND AggregateRoute 
                state = A2 // it should do this, but not today
                
                payloadRefurbish := helperAggregatePacket( parentIP, accumulator, observations )

                if routingMode == 0 {
                    RouterWaitRoom[payloadRefurbish.Timestamp] = payloadRefurbish
                    RouterWaitCount[payloadRefurbish.Timestamp] = 0
                    SendHello(payloadRefurbish.Timestamp)   
                } else if routingMode == 1 {
                    routes = parseRoutes(log)
                    SendRoute(net.ParseIP(routes[parentIP.String()]), payloadRefurbish)
                }


                log.Debug( myIP.String() + " => State: A1, timeout() -> SND AggregateRoute")
                log.Debug( myIP.String() + " => " + string(j) )

                if rootNode { // Just to show something
                    LogSuccess() // Suuuuuuucceeeeess!!!
                }
                CleanupTheHouse()
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
            Level: 0,
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
        str := string(buf[0:n])
        // if myIP.String() != "10.12.0.1" {
        //     log.Debug("buffer => "+str)
        // }
        
        buffer <- str
        checkError(err, log)
    }

    close(buffer)
    close(output)

    <-done
}

// the problem is the routing, when I send a HELLO message,
// there could be times, because of reachability or clash between messages
// where no one hears the HELLO so no one replies therefore
// it gets stuck.

// So i need to add a timeout mechanism for the hello message,
// maybe small one, so if no one replies it tries again and again
// until someone replies my hello and I can route.
// Also I should consider a small timeout, not that big. Random timeout of course.
// And MAYBE a "custom" timeout type, instead of reusing the TimeoutType
// I could have a TimeoutHelloType so I can distinguish between the two timeouts.



// ------------------------------------------------------------------------------------

// INFO=> 1026 22:05:02.37 Treesip.go:169 10.12.0.22 SENDING_MESSAGE=1
// DEBU=> 1026 22:05:02.371 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.18","dst":"10.12.0.3","ts":"1012016_1477519485045133019"}
// DEBU=> 1026 22:05:02.382 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.19","dst":"10.12.0.3","ts":"1012016_1477519485045133019"}
// DEBU=> 1026 22:05:02.385 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.20","dst":"10.12.0.3","ts":"1012016_1477519485045133019"}
// DEBU=> 1026 22:05:02.391 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.14","dst":"10.12.0.3","ts":"1012016_1477519485045133019"}
// DEBU=> 1026 22:05:02.393 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.2","dst":"10.12.0.3","ts":"1012016_1477519485045133019"}
// DEBU=> 1026 22:05:02.407 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.16","dst":"10.12.0.3","ts":"1012016_1477519485045133019"}
// DEBU=> 1026 22:05:04.2 Treesip.go:105 TimerHello Expired
// DEBU=> 1026 22:05:04.201 Treesip.go:224 10.12.0.22 => HELLO_TIMEOUT 1012020_1477519485704978357
// DEBU=> 1026 22:05:04.201 Treesip.go:167 10.12.0.22 {"tp":6,"src":"10.12.0.22","ts":"1012020_1477519485704978357"} MESSAGE_SIZE=62
// INFO=> 1026 22:05:04.201 Treesip.go:169 10.12.0.22 SENDING_MESSAGE=1
// DEBU=> 1026 22:05:04.201 Treesip.go:615 buffer => {"tp":6,"src":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.205 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.19","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.208 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.2","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.21 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.16","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.214 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.3","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.216 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.14","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.218 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.23","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.22 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.1","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.222 Treesip.go:615 buffer => {"tp":8,"src":"10.12.0.18","dst":"10.12.0.22","ts":"1012020_1477519485704978357"}
// DEBU=> 1026 22:05:04.361 Treesip.go:615 buffer => {"tp":6,"src":"10.12.0.3","ts":"1012016_1477519485045133019"}
// DEBU=> 1026 22:05:04.361 Treesip.go:218 10.12.0.22 => _HELLO to 10.12.0.3


// So this is the problem, if a AggregateRoute is needed ... is executed, but if it fails, then
// the node will timeout and will send the hello again, but by this time the timestamp changed
// because I did something very stupid somewhere SO ... everything fucks up
// because I heavily rely on this timestamp, so I need to check WHERE THE FUCK I'm changing this stamp
// and make sure that if it's because of a timeout, that the same timestamp remains.