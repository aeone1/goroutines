package workerPool

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var host string
var ports string
var numWorkers int

func init() {
	flag.StringVar(&host, "host", "127.0.0.1", "Host to scan.")
	flag.StringVar(&ports, "ports", "5400-5500", "Port(s) (e.g. 80, 22-100).")
	flag.IntVar(&numWorkers, "workers", runtime.NumCPU(), "Number of workers. Defaults to system number of CPUs.")
}

func Main() {
	flag.Parse()

	var openPorts []int

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		printResults(openPorts)
		os.Exit(0)
	}()

	portsToScan, err := parsePortsToScan(ports)
	if err != nil {
		fmt.Printf("Failded to parse ports to scan: %s\n", err)
		os.Exit(1)
	}

	portsChan := make(chan int, numWorkers) // A buffered channel doesn't need a reciever at the other end
	resultsChan := make(chan int)

	for i := 0; i < cap(portsChan); i++ { // numWorkers also acceptable here
		go worker(host, portsChan, resultsChan)
	}

	go func(){
		for _, p := range portsToScan {
			portsChan <- p
		}
	}()

	for i := 0; i < len(portsToScan); i++ {
		if p := <-resultsChan; p != 0 { // non-zero port means it's open
			openPorts = append(openPorts, p)
		}
	}

	close(portsChan)
	close(resultsChan)
	printResults(openPorts)
}

func parsePortsToScan(portsFlag string) ([]int, error) {
	rawPorts := strings.Split(portsFlag, "-")

	switch len(rawPorts) {
		case 1:
			port, err := strconv.Atoi(rawPorts[0])
			return []int{port}, err
		case 2:
			fp, err := strconv.Atoi(rawPorts[0])
			if err != nil {
				fmt.Println("Invalid 'from' port")
				return nil, err
			}

			tp, err := strconv.Atoi(rawPorts[1])
			if err != nil {
				fmt.Println("Invalid 'to' port")
				return nil, err
			}

			if fp > tp {
				x := tp
				tp = fp
				fp = x
			}

			ports := make([]int, 0, 2)
			for i := fp; i <= tp; i++ {
				ports = append(ports, i)
			}
			return ports, nil
		default:
			return nil, fmt.Errorf("Port(s) (e.g. 80, 22-100)")
	}
}

func worker(host string, portsChan <-chan int, resultsChan chan<- int) {
	for p := range portsChan {
		address := fmt.Sprintf("%s:%d", host, p)
		conn, err := net.Dial("tcp", address)
		if err != nil {
			fmt.Printf("%d CLOSED (%s)\n", p, err)
			resultsChan <- 0
			continue
		}
		conn.Close()
		resultsChan <- p
	}
}

func printResults(ports []int) {
	sort.Ints(ports)
	fmt.Println("\nResults\n------------------")
	for _, p := range ports {
		fmt.Printf("%d - OPEN\n", p)
	}
}
