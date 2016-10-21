package main
 
import (
    "os"
    "fmt"
    "net"
    "time"
    "strconv"
    "math/rand"
    "encoding/json"

    "github.com/op/go-logging"
    "github.com/chepeftw/treesip/utils"
    "github.com/chepeftw/treesip/packet"
    "github.com/chepeftw/treesip/timing"
    "github.com/chepeftw/treesip/manet"
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
var RouterWaitRoom map[string]packet.Packet = make(map[string]packet.Packet)
var ForwardedMessages []string = []string{}

// +++++++++ Channels
var buffer = make(chan string)
var output = make(chan string)
var router = make(chan string)
var done = make(chan bool)


func StartTimer() {
    timer = timing.Timeout(timeout, timer, r1)

    go func() {
        <- timer.C
        js, err := json.Marshal(packet.AssembleTimeout())
        utils.CheckError(err, log)
        buffer <- string(js)
        log.Debug("Timer expired")
    }()
}
func StopTimer() {
    timing.StopTimeout(timer)
}


func SendQuery(payload packet.Packet) {
    queryPayload := packet.AssembleQuery(payload, parentIP, myIP)
    SendMessage(queryPayload)
}
func SendAggregate(destination net.IP, outcome float32, observations int) {
    aggregatePayload := packet.AssembleAggregate(destination, outcome, observations, parentIP, myIP, timeout)
    SendMessage(aggregatePayload)
}
func SendMessage(payload packet.Packet) {
    js, err := json.Marshal(payload)
    utils.CheckError(err, log)

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
    utils.CheckError(err, log)
    LocalAddr, err := net.ResolveUDPAddr(Protocol, myIP.String()+":0")
    utils.CheckError(err, log)
    Conn, err := net.DialUDP(Protocol, LocalAddr, ServerAddr)
    utils.CheckError(err, log)
    defer Conn.Close()

    for {
        j, more := <-output
        if more {
            if Conn != nil {
                buf := []byte(j)
                _,err = Conn.Write(buf)
                log.Info( myIP.String() + " MESSAGE_SIZE=" + strconv.Itoa(len(buf)) )
                log.Info( myIP.String() + " SENDING_MESSAGE=1" )
                utils.CheckError(err, log)
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
    for {
        j, more := <-buffer
        if more {
            // First we take the json, unmarshal it to an object
            payload := packet.Packet{}
            json.Unmarshal([]byte(j), &payload)

            // Now we start! FSM TIME!
            switch state {
            case INITIAL:
                // RCV start() -> SND Query
                if payload.Type == packet.StartType {
                    startTime = time.Now().UnixNano() // Start time of the monitoring process
                    state = Q1 // Moving to Q1 state
                    parentIP = nil
                    timeout = payload.Timeout

                    SendQuery(payload)
                    StartTimer()

                    log.Info( myIP.String() + " => State: INITIAL, start() -> SND Query")

                } else if payload.Type == packet.QueryType { // RCV Query -> SND Query
                    state = Q1 // Moving to Q1 state
                    parentIP = payload.Source
                    timeout = payload.Timeout

                    SendQuery(payload)
                    StartTimer()

                    log.Info(myIP.String() + " => State: INITIAL, RCV Query -> SND Query")
                }
            break
            case Q1: 
                // RCV QueryACK -> acc(ACK_IP)
                if payload.Type == packet.QueryType && payload.Parent.Equal(myIP) && !payload.Source.Equal(myIP) {
                    state = Q2
                    queryACKlist = append(queryACKlist, payload.Source)

                    StopTimer()

                    log.Debug( myIP.String() + " => State: Q1, RCV QueryACK -> acc( " + payload.Source.String() + " )-> " + strconv.Itoa( len( queryACKlist ) ) )

                } else if payload.Type == packet.TimeoutType { // timeout()/edgeNode() -> SND Aggregate
                    state = A1

                    // Just one outcome and 1 observation because it should be the end of a branch
                    accumulator = manet.FunctionValue(accumulator)
                    observations = 1
                    SendAggregate(parentIP, accumulator, observations)
                    StartTimer()

                    log.Debug( myIP.String() + " => State: Q1, timeout() -> SND Aggregate")
                }
            break
            case Q2:
                // RCV QueryACK -> acc(ACK_IP)
                if payload.Type == packet.QueryType && payload.Parent.Equal(myIP) && !payload.Source.Equal(myIP) {
                    state = Q2 // loop to stay in Q2
                    queryACKlist = append(queryACKlist, payload.Source)

                    log.Debug( myIP.String() + " => State: Q2, RCV QueryACK -> acc( " + payload.Source.String() + " ) -> " + strconv.Itoa( len( queryACKlist ) ))

                } else if payload.Type == packet.AggregateType && payload.Aggregate.Destination.Equal(myIP) { // RCV Aggregate -> SND Aggregate 
                    // not always but yes
                    // I check that the parent it is itself, that means that he already stored this guy
                    // in the queryACKList
                    state = A1
                    queryACKlist = utils.RemoveFromList(payload.Source, queryACKlist)

                    StopTimer()
                    accumulator, observations  = manet.AggregateValue( payload.Aggregate.Outcome, payload.Aggregate.Observations, accumulator, observations)

                    if len(queryACKlist) == 0 {
                        accumulator = manet.FunctionValue(accumulator)
                        observations = observations + 1
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
                if payload.Type == packet.AggregateType && payload.Aggregate.Destination.Equal(myIP) {
                    state = A1
                    queryACKlist = utils.RemoveFromList(payload.Source, queryACKlist)

                    StopTimer()
                    accumulator, observations  = manet.AggregateValue( payload.Aggregate.Outcome, payload.Aggregate.Observations, accumulator, observations)

                    log.Debug( myIP.String() + " => State: A1, RCV Aggregate & loop() -> SND Aggregate " + payload.Source.String() + " -> " + strconv.Itoa(len(queryACKlist)))

                    if len(queryACKlist) == 0 && !rootNode {
                        accumulator = manet.FunctionValue(accumulator)
                        observations = observations + 1

                        SendAggregate(parentIP, accumulator, observations)
                        // StartTimer()

                        log.Debug("if len(queryACKlist) == 0 && !rootNode")

                    } else if len(queryACKlist) == 0 && rootNode { // WE ARE DONE!!!!
                        accumulator = manet.FunctionValue(accumulator)
                        observations = observations + 1

                        SendAggregate(myIP, accumulator, observations) // Just for ACK

                        log.Debug("else if len(queryACKlist) == 0 && rootNode")
                        LogSuccess() // Suuuuuuucceeeeess!!!
                        CleanupTheHouse()
                    } else {
                        StartTimer()
                    }

                } else if payload.Type == packet.AggregateType && payload.Source.Equal(parentIP) { // RCV AggregateACK -> done()
                    log.Debug( myIP.String() + " => State: A1, RCV Aggregate -> done()")
                    CleanupTheHouse()

                } else if payload.Type == packet.TimeoutType { // timeout -> SND AggregateRoute // not today
                    // state = A2 // it should do this, but not today
                    log.Debug( myIP.String() + " => State: A1, timeout() -> SND AggregateRoute")

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

        query := packet.Query{
                Function: "avg",
                RelaySet: []*net.IP{},
            }

        calculatedTimeout := 800
        if externalTimeout > 0 {
            calculatedTimeout = externalTimeout
        }

        payload := packet.Packet{
            Type: packet.StartType,
            Source: myIP,
            Timeout: calculatedTimeout,
            Query: &query,
        }

        log.Info("The leader has been chosen!!! All hail the new KING!!! " + neo)
        time.Sleep(time.Second * 3)

        js, err := json.Marshal(payload)
        utils.CheckError(err, log)
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
    myIP = utils.SelfieIP();
    log.Info("Good to go, my ip is " + myIP.String())

    // Lets prepare a address at any address at port 10001
    ServerAddr,err := net.ResolveUDPAddr(Protocol, Port)
    utils.CheckError(err, log)
 
    // Now listen at selected port
    ServerConn, err := net.ListenUDP(Protocol, ServerAddr)
    utils.CheckError(err, log)
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
        utils.CheckError(err, log)
    }

    close(buffer)
    close(output)

    <-done
}