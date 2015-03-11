package crankshaft

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"
)

var config Config

// MonitorClusters is the entry point into monitoring a turbine stream.
func MonitorClusters(conf Config) {
	config = conf

	log.Println(config)

	// Get individual clusters, make channels
	clusterList := config.Clusters
	//channels := make([]chan TurbineEvent, len(clusterList))
	eventChannel := make(EventChannel)

	// Start goroutines for each cluster
	for i := range clusterList {
		go turbine(eventChannel, strings.TrimSpace(config.Path), strings.TrimSpace(clusterList[i]))
	}

	// Get client (Influx or StatsD)
	client := provideStatWriter()

	// Consume events
	for event := range eventChannel {
		client.WriteEvent(event)
	}
}

func turbine(c EventChannel, turbinePath string, clusterName string) {
	defer close(c)

	for {
		err := attachToTurbine(turbinePath, clusterName, c)
		if err != nil {
			log.Println("Turbine session ended with error", err, "restarting...")
			time.Sleep(3 * time.Second) // wait
		}
	}
}

func attachToTurbine(turbinePath string, clusterName string, c EventChannel) error {
	log.Println("Opening Turbine connection for", clusterName)

	// TODO: urlencode
	req, err := http.NewRequest("GET", turbinePath+"/turbine.stream?cluster="+clusterName, nil)

	if err != nil {
		log.Println("Error creating HTTP request", err)
		return err
	}

	conn, err := connectToTurbineServer()
	if err != nil {
		log.Println("Error opening TCP socket", err)
		return err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))
	clientConn := httputil.NewClientConn(conn, nil)
	resp, err := clientConn.Do(req)

	if err != nil {
		log.Println("Error sending HTTP request", err)
		return err
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			log.Println("Error reading bytes from stream", err)
			return err
		}

		conn.SetDeadline(time.Now().Add(10 * time.Second))
		line = bytes.TrimSpace(line)
		s := string(line[:])

		if strings.HasPrefix(s, "data: ") {
			s = strings.TrimPrefix(s, "data: ")

			data, err := unmarshalJSON(s)

			if err != nil {
				log.Println("Error decoding JSON", err)
				return err
			}

			c <- &TurbineEvent{clusterName, data}
		}
	}
}

func connectToTurbineServer() (net.Conn, error) {
	turbineURL := config.Host + ":" + strconv.Itoa(config.Port)
	log.Println("Opening Turbine connection host:", turbineURL)
	conn, err := net.Dial("tcp", turbineURL)

	if config.TLSEnabled {
		conn = tls.Client(conn, &tls.Config{})
	}

	return conn, err
}

func unmarshalJSON(payload string) (map[string]interface{}, error) {
	var f map[string]interface{}
	err := json.Unmarshal([]byte(payload), &f)

	return f, err
}

func provideStatWriter() StatWriter {
	var statClient StatWriter

	switch config.BackendType {
	default:
		log.Fatal("Error:", config.BackendType, "is not a valid backend type")
	case "statsd":
		statClient = GetStatsClient()
	case "stackdriver":
		statClient = GetStackDriverClient()
	}

	return statClient
}
