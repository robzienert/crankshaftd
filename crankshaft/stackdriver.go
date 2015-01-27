package crankshaft

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

type metricSeries struct {
	total       int64
	occurrences int64
}

// StackDriverDataPoint represents a single metric
type StackDriverDataPoint struct {
	Name        string `json:"name"`
	Value       int64  `json:"value"`
	CollectedAt int    `json:"collected_at"`
}

// StackDriverRequestWrapper forms the base of a bulk metrics recording
type StackDriverRequestWrapper struct {
	Timestamp    int                    `json:"timestamp"`
	ProtoVersion int                    `json:"proto_version"`
	Data         []StackDriverDataPoint `json:"data"`
}

// StackDriverClient responsible for reporting Turbine data on a timed interval
type StackDriverClient struct {
	client *http.Client
	apiKey string
	ticker *time.Ticker
}

var (
	mutex = &sync.Mutex()
	state = make(map[string]*metricSeries)
)

// GetStackDriverClient is a constructor for the StackDriverClient type
func GetStackDriverClient() *StackdriverClient {
	apiKey := config.StackDriverConfig.ApiKey
	client := &http.Client()
	ticker := time.NewTicker(time.Second * 60)

	client := &StackDriverClient{
		client: client,
		apiKey: apiKey,
		ticker: ticker,
	}
	client.initTickRoutine()
	return client
}

func (c *StackDriverClient) initTickRoutine() {
	go func() {
		for {
			select {
			case <-c.ticker:
				log.Println("Aggregating stored timeseries")

				nowUnix := time.Now().Unix()
				wrapper := &StackDriverRequestWrapper{
					Timestamp:    nowUnix,
					ProtoVersion: 1,
					Data:         *[]StackDriverDataPoint,
				}
				mutex.Lock()
				for name, metrics := range state {
					append(wrapper.Data, &StackDriverDataPoint{
						Name:        name,
						Value:       math.Floor(metrics.total / metrics.occurrences),
						CollectedAt: nowUnix,
					})
					delete(state, name)
				}
				mutex.Unlock()

				log.Println("Publishing metrics to StackDriver")
				body, _ := json.Marshal(wrapper)

				req := c.client.NewRequest("POST", "https://custom-gateway.stackdriver.com/v1/custom", strings.NewReader(body))
				req.Header.Add("Content-Type", "application/json")
				req.Header.Add("x-stackdriver-key", c.apiKey)

				resp, err := c.client.Do(req)
				if err != nil {
					log.Println("Error publishing metrics to StackDriver", err)
				}
			}
		}
	}()
}

func writeToState(name string, statVal int64) {
	mutex.Lock()
	if state[name] {
		state[name].occurrences++
		state[name].total += statVal
	} else {
		state[name] = &metricSeries{
			occurrences: 1,
			total:       statVal,
		}
	}
	mutex.Unlock()
}

// WriteEvent handles Turbine events, aggregating them into a shared state
// which is cleared every StackDriverClient ticker interval.
func WriteEvent(event *TurbineEvent) {
	name := event.data["name"].(string)
	resourceType := event.data["type"].(string)

	for k, v := range event.data {
		if !strings.HasPrefix(k, "rollingCount") && !strings.HasPrefix(k, "current") &&
			!strings.HasPrefix(k, "isCircuitBreakerOpen") && !strings.HasPrefix(k, "latencyExecute") &&
			!strings.HasPrefix(k, "latencyTotal") {
			continue
		}

		statKey := buildStatKey(event.clusterName, name, resourceType, k)

		switch v := v.(type) {
		default:
			log.Printf("unexpected data element %T, %s", v, v)
		case string:
			// ignored
		case map[string]interface{}:
			for pct, val := range v {
				pctVal := int64(val.(float64))
				name := statKey + "." + strings.Replace(pct, ".", "_", -1) + "_pct"

				writeToState(name, pctVal)
			}
		case bool:
			var statVal int64
			if v {
				statVal = 1
			} else {
				statVal = 0
			}

			writeToState(statKey, statVal)
		case int64:
			writeToState(statKey, v)
		case float64:
			writeToState(statKey, int64(v))
		}
	}
}
