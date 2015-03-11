package crankshaft

// Config of crankshaftd
type Config struct {
	Host        string
	Path        string
	Port        int
	TLSEnabled  bool `toml:"tls_enabled"`
	Clusters    []string
	BackendType string
	Statsd      StatsDConfig
	StackDriver StackDriverConfig
}

// StatsDConfig info
type StatsDConfig struct {
	Host   string
	Port   int
	Prefix string
}

// StackDriverConfig info
type StackDriverConfig struct {
	ApiKey string
}

type EventChannel chan *TurbineEvent

type StatWriter interface {
	WriteEvent(event *TurbineEvent)
}

type TurbineEvent struct {
	clusterName string
	data        map[string]interface{}
}
