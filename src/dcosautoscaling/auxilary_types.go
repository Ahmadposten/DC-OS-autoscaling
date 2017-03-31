package dcosautoscaling

type MarathonAppsResponse struct {
	Apps []Application `json:"apps"`
}

type MarathonAppResponse struct {
	App Application `json:"app"`
}

type MesosStat struct {
	ExecutorId string `json:"executor_id"`
	Statistics struct {
		CPUlimit          float64 `json:"cpus_limit"`
		CPUSystemTimeSecs float64 `json:"cpus_system_time_secs"`
		CPUUserTimeSecs   float64 `json:"cpus_user_time_secs"`
		MemLimit          float64 `json:"mem_limit_bytes"`
		MemRSS            float64 `json:"mem_rss_bytes"`
		Timestamp         float64 `json:"timestamp"`
	} `json:"statistics"`
}

type Stat struct {
	CpuUsage float64
	MemUsage float64
}
type Policy struct {
	Type      string
	Threshold float64
	Interval  int
	Samples   int
	Operator  string
	Action    string
	Step      int
}

type Task struct {
	Id    string `json:"id"`
	Agent string `json:"slaveId"`
	Host  string `json:"host"`
}

type HTTPResponseError struct {
	s string
}

func (e HTTPResponseError) Error() string {
	return e.s
}
