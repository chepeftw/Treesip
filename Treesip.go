package main
 
import (
    "os"
    "fmt"
    "net"
    "time"
    "math"
    "flag"
    "strings"
    "strconv"
    "encoding/json"

    "github.com/op/go-logging"
    "github.com/chepeftw/treesiplibs"
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
    DefPort           = ":10001"
    RouterPort        = ":10000"
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
)

// +++++++++ Global vars
var state = INITIAL
var myIP net.IP = net.ParseIP(LocalhostAddr)
var myLH net.IP = net.ParseIP(LocalhostAddr)
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
//var routingMode = 0

// +++++++++ Routing Protocol
//var routes map[string]string = make(map[string]string)

// +++++++++ Multi node support
var Port = ":0"
var PortInt = 0

// +++++++++ Channels
var buffer = make(chan string)
var output = make(chan string)
var done = make(chan bool)

func StartTreeTimer() {
    treeTimeout := float32(maxTreeTimeout) * float32(math.Pow( 0.75, float64(level) ))
    StartTimerStar(treeTimeout)
}

func StartTimer() {
    StartTimerStar( float32(timeout) )
}

func StartTimerStar(localTimeout float32) {
    treesiplibs.StopTimeout(timer)
    timer = treesiplibs.StartTimeoutF(localTimeout)

    go func() {
        <- timer.C
        js, err := json.Marshal(treesiplibs.AssembleTimeout())
        treesiplibs.CheckError(err, log)
        buffer <- string(js)
        log.Debug("Timer expired")
    }()
}
func StopTimer() { treesiplibs.StopTimeout(timer) }


//func SendRoute(gateway net.IP, payloadIn treesiplibs.Packet) {
//    SendMessage( treesiplibs.AssembleRoute(gateway, payloadIn) )
//}
func SendQuery(payload treesiplibs.Packet) {
    SendMessage( treesiplibs.AssembleQuery(payload, parentIP, myIP) )
}
func SendAggregate(destination net.IP, outcome float32, observations int) {
    SendMessage( helperAggregatePacket( destination, outcome, observations ) )
}
func helperAggregatePacket(destination net.IP, outcome float32, observations int) treesiplibs.Packet {
    stamp := strings.Replace(myIP.String(), ".", "", -1) + "_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
    return treesiplibs.AssembleAggregate(destination, outcome, observations, parentIP, myIP, timeout, stamp, PortInt)
}
func SendMessage(payload treesiplibs.Packet) {
    js, err := json.Marshal(payload)
	treesiplibs.CheckError(err, log)
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
    treesiplibs.CheckError(err, log)
    LocalAddr, err := net.ResolveUDPAddr(Protocol, myIP.String()+":0")
    treesiplibs.CheckError(err, log)
    Conn, err := net.DialUDP(Protocol, LocalAddr, ServerAddr)
    treesiplibs.CheckError(err, log)
    defer Conn.Close()

    for {
        j, more := <-output
        if more {
            if Conn != nil {
                buf := []byte(j)
                _,err = Conn.Write(buf)
                log.Debug( myIP.String() + " " + j + " MESSAGE_SIZE=" + strconv.Itoa(len(buf)) )
                log.Info( myIP.String() + " SENDING_MESSAGE=1" )
                treesiplibs.CheckError(err, log)
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
for {
    j, more := <-buffer
    if more {
        // First we take the json, unmarshal it to an object
        payload := treesiplibs.Packet{}
        json.Unmarshal([]byte(j), &payload)


        // Now we start! FSM TIME!
        switch state {
        case INITIAL:
            // RCV start() -> SND Query
            if payload.Type == treesiplibs.StartType {
                startTime = time.Now().UnixNano() // Start time of the monitoring process
            }

            if payload.Type == treesiplibs.StartType || payload.Type == treesiplibs.QueryType {
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
            if payload.Type == treesiplibs.QueryType && 
                    eqIp(myIP, payload.Parent) && 
                    !eqIp(myIP, payload.Source) {

                state = Q2
                queryACKlist = append(queryACKlist, payload.Source)

                StopTimer()

                log.Debug( myIP.String() + " => State: Q1, RCV QueryACK -> acc( " + payload.Source.String() + " )-> " + strconv.Itoa( len( queryACKlist ) ) )

            } else if payload.Type == treesiplibs.TimeoutType { // timeout()/edgeNode() -> SND Aggregate
                state = A1

                // Just one outcome and 1 observation because it should be the end of a branch
                accumulator = treesiplibs.FunctionValue(accumulator)
                observations = 1
                log.Debug( myIP.String() + " => OBSERVATIONS=1" )
                SendAggregate(parentIP, accumulator, observations)
                StartTimer()

                log.Debug( myIP.String() + " => State: Q1, timeout() -> SND Aggregate")
            }
        break
        case Q2:
            // RCV QueryACK -> acc(ACK_IP)
            if payload.Type == treesiplibs.QueryType && 
                    eqIp(myIP, payload.Parent) && 
                    !eqIp(myIP, payload.Source) {

                state = Q2 // loop to stay in Q2
                queryACKlist = append(queryACKlist, payload.Source)

                log.Debug( myIP.String() + " => State: Q2, RCV QueryACK -> acc( " + payload.Source.String() + " ) -> " + strconv.Itoa( len( queryACKlist ) ))

            } else if ( payload.Type == treesiplibs.AggregateType || 
                    payload.Type == treesiplibs.RouteByGossipType ) && 
                    eqIp(myIP, payload.Destination) { // RCV Aggregate -> SND Aggregate 

                // not always but yes
                // I check that the parent it is itself, that means that he already stored this guy
                // in the queryACKList
                state = A1
                queryACKlist = treesiplibs.RemoveFromList(payload.Source, queryACKlist)

                StopTimer()
                accumulator, observations  = treesiplibs.AggregateValue( payload.Aggregate.Outcome, payload.Aggregate.Observations, accumulator, observations)

                if len(queryACKlist) == 0 {
                    accumulator = treesiplibs.FunctionValue(accumulator)
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
            if ( payload.Type == treesiplibs.AggregateType || 
                    payload.Type == treesiplibs.RouteByGossipType ) && 
                    eqIp(myIP, payload.Destination) {

                if treesiplibs.ContainsIP(queryACKlist, payload.Source) {

                    if payload.Type == treesiplibs.RouteByGossipType {
                            log.Debug( myIP.String() + " Incoming routing => " + j)
                    }
                    
                    state = A1
                    queryACKlist = treesiplibs.RemoveFromList(payload.Source, queryACKlist)

                    StopTimer()
                    accumulator, observations  = treesiplibs.AggregateValue( payload.Aggregate.Outcome, payload.Aggregate.Observations, accumulator, observations)

                    log.Debug( myIP.String() + " => State: A1, RCV Aggregate & loop() -> SND Aggregate " + payload.Source.String() + " -> " + strconv.Itoa(len(queryACKlist)))

                    if len(queryACKlist) == 0 {
                        accumulator = treesiplibs.FunctionValue(accumulator)
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

            } else if payload.Type == treesiplibs.AggregateType && 
                    eqIp(parentIP, payload.Source) { // RCV AggregateACK -> done()

                log.Debug( myIP.String() + " => State: A1, RCV Aggregate -> done()")
                CleanupTheHouse()

            } else if payload.Type == treesiplibs.TimeoutType { // timeout -> SND AggregateRoute 
                state = A2 // it should do this, but not today
                
                payloadRefurbish := helperAggregatePacket( parentIP, accumulator, observations )

                //if routingMode == 0 {
	        toRouter(payloadRefurbish)
                //} else if routingMode == 1 {
                //    routes = treesiplibs.ParseRoutes(log)
                //    SendRoute(net.ParseIP(routes[parentIP.String()]), payloadRefurbish)
                //}

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

    } else {
        log.Debug("closing channel")
        done <- true
        return
    }

    }
}

func eqIp( a net.IP, b net.IP ) bool {
	return treesiplibs.CompareIPs(a, b)
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

        query := treesiplibs.Query{
                Function: "avg",
                RelaySet: []*net.IP{},
            }

        calculatedTimeout := 800
        if externalTimeout > 0 {
            calculatedTimeout = externalTimeout
        }

        payload := treesiplibs.Packet{
            Type: treesiplibs.StartType,
            Source: myIP,
            Port: PortInt,
            Timeout: calculatedTimeout,
            Level: 0,
            Query: &query,
        }

        log.Info("The leader has been chosen!!! All hail the new KING!!! " + neo)
        time.Sleep(time.Second * 3)

        js, err := json.Marshal(payload)
        treesiplibs.CheckError(err, log)
        log.Debug("Initial JSON " + string(js))
        buffer <- string(js)
    }
}


func toRouter(payload treesiplibs.Packet) {
    ServerAddr,err := net.ResolveUDPAddr(Protocol, myLH.String()+RouterPort)
    treesiplibs.CheckError(err, log)
    LocalAddr, err := net.ResolveUDPAddr(Protocol, myLH.String()+":0")
    treesiplibs.CheckError(err, log)
    Conn, err := net.DialUDP(Protocol, LocalAddr, ServerAddr)
    treesiplibs.CheckError(err, log)
    defer Conn.Close()

    if Conn != nil {
        js, err := json.Marshal(payload)
        treesiplibs.CheckError(err, log)

        buf := []byte(js)
        _,err = Conn.Write(buf)
        log.Info( myIP.String() + " INTERNAL_MESSAGE=1" )
        treesiplibs.CheckError(err, log)
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

    // Flags
    var portFlag string
    flag.StringVar(&portFlag, "port", DefPort, "The IP of the root node, e.g. :10001")
    Port = portFlag
    PortInt, _ = strconv.Atoi( Port[1:] )

    targetSyncFlag := flag.Float64("sync", targetSync, "The sync time to start working")
    targetSync = *targetSyncFlag

    var rootNodeIP string
    flag.StringVar(&rootNodeIP, "root", "10.0.0.0", "The IP of the root node")
    electionNode = rootNodeIP
    log.Info("NEO : rootNodeIP is " + rootNodeIP)
    log.Info("NEO : electionNode is " + electionNode)


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
    if targetSync > now {
        sleepTime = int(targetSync - now)
        log.Info("SYNC: Sync time is " + strconv.FormatFloat( targetSync, 'f', 6, 64) )
    } else {
        sleepTime = globalNumberNodes
    }
    log.Info("SYNC: sleepTime is " + strconv.Itoa(sleepTime))
    time.Sleep(time.Second * time.Duration(sleepTime))
    // ------------

    // But first let me take a selfie, in a Go lang program is getting my own IP
    myIP = treesiplibs.SelfieIP()
    log.Info("Good to go, my ip is " + myIP.String())

    // Lets prepare a address at any address at port 10001
    ServerAddr,err := net.ResolveUDPAddr(Protocol, Port)
    treesiplibs.CheckError(err, log)
 
    // Now listen at selected port
    ServerConn, err := net.ListenUDP(Protocol, ServerAddr)
    treesiplibs.CheckError(err, log)
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
        
        buffer <- str
        treesiplibs.CheckError(err, log)
    }

    close(buffer)
    close(output)

    <-done
}

