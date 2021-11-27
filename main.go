package main

import (
    "net/http"
	"os"
    "github.com/clockworksoul/smudge"
    "fmt"
    "net"
    "sync"
"time"
   	"encoding/json"
    lineblocs "bitbucket.org/infinitet3ch/lineblocs-go-helpers"
	"strings"
	"io/ioutil"
	"strconv"
	"github.com/nadirhamid/amigo"

)

type ServerData struct{
  mu sync.RWMutex
  servers []*lineblocs.MediaServer
}
type MyStatusListener struct {
    smudge.StatusListener
}
var servers []*lineblocs.MediaServer
var router *lineblocs.SIPRouter
var data *ServerData
var channel chan []*lineblocs.MediaServer
var myServer  *lineblocs.MediaServer
var ami  *amigo.Amigo


// Creating hanlder functions
func DeviceStateChangeHandler(m map[string]string) {
	fmt.Printf("DeviceStateChange event received: %v\n", m)
}

func DefaultHandler(m map[string]string) {
	fmt.Printf("Event received: %v\n", m)
}


func CreateMediaServers()([]*lineblocs.MediaServer, error) {
    //region := os.Getenv("REGION")
    //servers,err := lineblocs.CreateMediaServers(region)
    servers,err := lineblocs.CreateMediaServers()
	if err != nil {
        return nil, err
    }
    return servers,nil
}
func CreateSIPRouter()(*lineblocs.SIPRouter, error) {
    region := os.Getenv("REGION")
    router,err := lineblocs.GetSIPRouter(region)
	if err != nil {
        return nil, err
    }
    return router,nil
}
func (m MyStatusListener) OnChange(node *smudge.Node, status smudge.NodeStatus) {
    fmt.Printf("Node %s is now status %s\n", node.Address(), status)
        for _, server := range servers {
            if node.Address() == server.Node.Address() { //change the node
                server.Status = status.String()
		fmt.Printf("setting server status to %s\r\n", server.Status);
		err := lineblocs.UpdateLiveStat(server, "live_status",  server.Status)
		if err != nil {
				fmt.Println("Could not update live server status..\r\n");
			}

            }
        }
    data.mu.Lock()

    defer data.mu.Unlock()
    data.servers =servers
    for _, server := range servers {
    	fmt.Printf("server %s status = %s \r\n", server.IpAddress, server.Status)
	}

}

type MyBroadcastListener struct {
    smudge.BroadcastListener
}

func (m MyBroadcastListener) OnBroadcast(b *smudge.Broadcast) {
    fmt.Printf("Received broadcast from %s: %s\n",
        b.Origin().Address(),
        string(b.Bytes()))
}

func getCPUSample() (idle, total uint64) {
    contents, err := ioutil.ReadFile("/proc/stat")
    if err != nil {
        return
    }
    lines := strings.Split(string(contents), "\n")
    for _, line := range(lines) {
        fields := strings.Fields(line)
        if fields[0] == "cpu" {
            numFields := len(fields)
            for i := 1; i < numFields; i++ {
                val, err := strconv.ParseUint(fields[i], 10, 64)
                if err != nil {
                    fmt.Println("Error: ", i, fields[i], err)
                }
                total += val // tally up all the numbers to get total ticks
                if i == 4 {  // idle is the 5th field in the cpu line
                    idle = val
                }
            }
            return
        }
    }
    return
}

func updateLiveStats() {
	fmt.Println("Updating live statistics..\r\n");
	result, err := ami.Action(map[string]string{"Action": "Command", "Command": "core show channels"})
	// If not error, processing result. Response on Action will follow in defined events.
	// You need to catch them in event channel, DefaultHandler or specified HandlerFunction
	fmt.Println(result, err)
	for k, v := range result { 
			fmt.Printf("key[%s] value[%s]\n", k, v)
		if k == "Output2" {
			results := strings.Split(v, " ")
			fmt.Printf("%s active calls\r\n", results[0]);
			fmt.Printf("setting active calls  to %s\r\n",results[0]);
			err = lineblocs.UpdateLiveStat(myServer, "live_call_count",  results[0])
			if err != nil {
				fmt.Println("Could not update live call count..\r\n");
			}
		}
	}

    idle0, total0 := getCPUSample()
    time.Sleep(3 * time.Second)
    idle1, total1 := getCPUSample()

    idleTicks := float64(idle1 - idle0)
    totalTicks := float64(total1 - total0)
    cpuUsage := 100 * (totalTicks - idleTicks) / totalTicks
	fmt.Printf("CPU usage is at %f\r\n", cpuUsage);
			cpuUsageStr := strconv.FormatFloat(cpuUsage, 'f', 6, 64)
			fmt.Printf("setting active calls  to %s\r\n",cpuUsageStr)
			err = lineblocs.UpdateLiveStat(myServer, "live_cpu_pct_used",  cpuUsageStr)
			if err != nil {
				fmt.Println("Could not update live call count..\r\n");
			}


}

func startSmudge(servers []*lineblocs.MediaServer) {
    heartbeatMillis := 500
    listenPort := 9999

    // Set configuration options
    smudge.SetListenPort(listenPort)
    smudge.SetHeartbeatMillis(heartbeatMillis)
    smudge.SetListenIP(net.ParseIP("0.0.0.0"))

    // Add the status listener
    smudge.AddStatusListener(MyStatusListener{})

    // Add the broadcast listener
    smudge.AddBroadcastListener(MyBroadcastListener{})
    fmt.Println("Adding router IP " + router.IpAddress);
    // add the router
    node, err := smudge.CreateNodeByAddress(router.IpAddress + ":9999")
    if err != nil {
	fmt.Println("Could not add router..");
        //panic(err)
        //return
    } else {
    	smudge.AddNode(node)
    }
    for _, server := range servers {
	smudge.AddNode(server.Node)
    }
    //channel <- servers
    // Start the server!

    smudge.Begin()

}
func Status(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("received Status request..\r\n");
    data.mu.RLock()
    defer data.mu.RUnlock()
    srvs := data.servers
    fmt.Printf("got statuses..\r\n");
    for _, server := range srvs {
        fmt.Printf("server list: %s\r\n", server.IpAddress)
    }

	json.NewEncoder(w).Encode(srvs)
	w.WriteHeader(http.StatusOK) 
}


func main() {
    var err error
	fmt.Println("starting smudge node..");
    channel = make(chan []*lineblocs.MediaServer)
    servers,err = CreateMediaServers()
	if err != nil {
		panic(err)
	}

	for _, server := range servers {
        //make it alive initially
        server.Status = "ALIVE"
        fmt.Printf("comparing %s and %s\r\n", server.IpAddress,os.Getenv("IP_ADDRESS"));
        if (server.IpAddress == os.Getenv("IP_ADDRESS")) {
            fmt.Printf("Setting my own server!\r\n");
            myServer = server
        }

        fmt.Printf("adding server: ID %d, IP %s with initial status %s\r\n", server.Id, server.IpAddress, server.Status)
	}
    data = &ServerData{
        mu: sync.RWMutex{}, servers: servers }


    router,err = CreateSIPRouter()
	if err != nil {
		panic(err)
    }

	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	fmt.Println("Init Amigo")

    user := os.Getenv("AMI_USER")
    pass := os.Getenv("AMI_PASS")
    host := os.Getenv("AMI_HOST")
    port := os.Getenv("AMI_PORT")
	settings := &amigo.Settings{Username: user, Password: pass, Host: host, Port: port}
	ami = amigo.New(settings)
	ami.Connect()
	ami.On("connect", func(message string) {

		fmt.Println("Connected to AMI!!!!!!!!!!!", message)
        go func() {
            for {
            select {
                case <- ticker.C:
                    // do stuff
                updateLiveStats();
                case <- quit:
                    ticker.Stop()
                    return
                }
            }
        }()
    });

	ami.On("error", func(message string) {
		fmt.Println("Connection error:", message)
	})

	// Registering handler function for event "DeviceStateChange"
	ami.RegisterHandler("DeviceStateChange", DeviceStateChangeHandler)

	// Registering default handler function for all events.
	ami.RegisterDefaultHandler(DefaultHandler)

	// Optionally create channel to receiving all events
	// and set created channel to receive all events
	c := make(chan map[string]string, 100)
	ami.SetEventChannel(c)

	// Check if connected with Asterisk, will send Action "QueueSummary"
	if ami.Connected() {
	}
	
	ch := make(chan bool)
	<-ch
}
