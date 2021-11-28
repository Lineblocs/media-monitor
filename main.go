package main

import (
    "net/http"
	"os"
    "github.com/clockworksoul/smudge"
    "fmt"
    "net"
    "sync"
"time"
"context"
"errors"
   	"encoding/json"
    lineblocs "bitbucket.org/infinitet3ch/lineblocs-go-helpers"
	"strings"
	"io/ioutil"
	"strconv"
	"github.com/nadirhamid/amigo"
     metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/tools/clientcmd"
    metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
    	restclient "k8s.io/client-go/rest"
    v1 "k8s.io/api/core/v1"

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
var currentNode  *lineblocs.MediaServer
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
		err := SafeUpdateLiveStat(server, "live_status",  server.Status)
		if err != nil {
				fmt.Println("Could not update live server status..\r\n");
			}

            }
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

func createK8sConfig() (kubernetes.Interface, *restclient.Config, error) {
	var kubeconfig string
	kubeconfig= "/root/.kube/config"
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, nil, err
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return clientset, config,nil
}
func GetNodeByName(clientset kubernetes.Interface, name string, namespace string) (*v1.Node, error) {
 	nodes, err := ListNodes(clientset)
    if err != nil {
        return nil, err
    }
	for _, item := range nodes {
		if item.Name == name {
			return &item, nil
		}
	}
	return nil, errors.New("No node found.")
}

func ListNodes(clientset kubernetes.Interface) ([]v1.Node, error) {
    nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})

    if err != nil {
        return nil, err
    }

    return nodes.Items, nil
}
func addNodeToCluster() (error) {
    fmt.Println("adding new node to cluster..")
    return nil
}
func removeNode() (error) {
    return nil
}
func getCPUSample2() (float64, error) {
    clientset,config,err :=createK8sConfig()
    if err != nil {
        return 0,err
    }
    mc, err := metrics.NewForConfig(config)
    if err != nil {
        return 0,err
    }
    ctx := context.Background()
    //ns :="voip"
    nodeMetrics, err := mc.MetricsV1beta1().NodeMetricses().List(ctx,metav1.ListOptions{})
    if err != nil {
        fmt.Println("Error:", err)
        return 0,err
    }
    for _, nodeMetric := range nodeMetrics.Items {
        cpuQuantity:= nodeMetric.Usage.Cpu().MilliValue()
        memQuantity, ok := nodeMetric.Usage.Memory().AsInt64()
        if !ok {
        fmt.Println("Cannot get stats..")
            return 0,errors.New("error stats")
        }
        node, err := GetNodeByName(clientset, nodeMetric.Name, "voip")
        if err != nil {
        fmt.Println("Error:", err)
            return 0,err
        }
        for _, item := range node.Status.Addresses {
            if item.Type == "ExternalIP" {
                ip:=item.Address
                resavailable:=node.Status.Capacity
               	if available, found := resavailable[v1.ResourceCPU]; found {
                    cpuUsed := float64(cpuQuantity) / float64(available.MilliValue()) * 100
                    if ip == os.Getenv("IP_ADDRESS") { //same node
                        msg := fmt.Sprintf("Node Name: %s \n %f, CPU usage: %d \n Memory usage: %d", nodeMetric.Name, cpuUsed, cpuQuantity, memQuantity)
                        fmt.Println(msg)
                        return cpuUsed,nil
                      } else { // another node
                      }
               }
            }
        }
    }
    /*
    for _, podmetric := range podmetrics.items {
        _=podmetric.metadata.name
        podContainers := podMetric.Containers
        for _, container := range podContainers {
            cpuQuantity, ok := container.Usage.Cpu().AsInt64()
            memQuantity, ok := container.Usage.Memory().AsInt64()
            if !ok {
                return 0,0
            }
            msg := fmt.Sprintf("Container Name: %s \n CPU usage: %d \n Memory usage: %d", container.Name, cpuQuantity, memQuantity)
            fmt.Println(msg)
        }

    }
    */
    return 0,errors.New("could not get stat..")
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

func SafeUpdateLiveStat(server *lineblocs.MediaServer, field string, value string) (error) {
    if server == nil {
        fmt.Println("server not found.. ")
        return nil
    }
    err:=lineblocs.UpdateLiveStat(server, field,value)
     if err != nil {
        fmt.Println("Error: ", err)
        return err
    }
    return nil
}
func updateLiveStats() {
	fmt.Println("Updating live statistics..\r\n");

	err := setupCurrentNode()
    if err != nil {
        fmt.Println("Could not setup current node..\r\n");
        return
    }

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
			err = SafeUpdateLiveStat(currentNode, "live_call_count",  results[0])
			if err != nil {
				fmt.Println("Could not update live call count..\r\n");
			}
		}
	}

    total0,err := getCPUSample2()
    if err != nil {
        fmt.Println("Error: ", err)
        return
    }
    fmt.Sprintf("%f", total0)
	fmt.Printf("CPU usage is at %f\r\n", total0);
    cpuUsageStr := strconv.FormatFloat(total0, 'f', 6, 64)
    fmt.Printf("setting cpu usage to %s\r\n",cpuUsageStr)
    err = SafeUpdateLiveStat(currentNode, "live_cpu_pct_used",  cpuUsageStr)
    if err != nil {
        fmt.Println("Could not update cpu usage..\r\n");
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

func setupCurrentNode() (error) {
    servers,err := CreateMediaServers()
	if err != nil {
		return err
	}

	for _, server := range servers {
        //make it alive initially
        server.Status = "ALIVE"
        fmt.Printf("comparing %s and %s\r\n", server.IpAddress,os.Getenv("IP_ADDRESS"));
       	ipSlice := strings.Split(server.IpAddress, ":")
        if (ipSlice[0] == os.Getenv("IP_ADDRESS")) {
            fmt.Printf("Setting current node!\r\n");
            currentNode = server
        }

        fmt.Printf("adding server: ID %d, IP %s with initial status %s\r\n", server.Id, server.IpAddress, server.Status)
	}
    return nil
}


func main() {
    var err error
	fmt.Println("starting smudge node..");
    channel = make(chan []*lineblocs.MediaServer)
    servers,err = CreateMediaServers()
	if err != nil {
		panic(err)
	}
    err=setupCurrentNode()
    if err != nil {
	    fmt.Println("current node not found in DB...")
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
