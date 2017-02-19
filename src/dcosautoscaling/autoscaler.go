package dcosautoscaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type MesosStat struct {
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

type ApplicationInterface interface {
	Scale() error
	GetStatistics() (Stat, error)
}

type Application struct {
	Id        string `json:"id"`
	Slaves    []Task `json:"tasks"`
	Min       int
	Max       int
	Cooldown  int
	Stats     []Stat
	Instances int `json:"instances"`
	Desired   int
	Policies  []Policy
	Labels    map[string]string
}

type Policy struct {
	Type      string
	Threshold float64
	Interval  int
	Samples   int
}

type Task struct {
	Id    string `json:"id"`
	Agent string `json:"slaveId"`
	Host  string `json:"host"`
}

type ApplicationError struct {
	s string
}

func (e ApplicationError) Error() string {
	return e.s
}

func (a Application) Scale() error {
	log.Print(fmt.Sprintf("Scaling %s", a.Id))
	if a.Max < a.Desired {
		log.Print("Can not scale up, reached the maximum for this app")
		err := ApplicationError{s: "Can not scale up: Reached maximum scale"}
		return err
	}

	if a.Desired < a.Min {
		log.Print("Can not scale down, reached minumum")
		err := ApplicationError{s: "Can not scale down: Reached minimum scale"}
		return err
	}

	url := fmt.Sprintf("http://marathon.mesos:8080/v2/apps%s", a.Id)

	body := fmt.Sprintf("{\"instances\":%s}", a.Desired)
	response, reqerr := Call("PUT", url, body)

	log.Print(fmt.Sprintf("Response from scaling %s is %s", a.Id, response))
	if reqerr != nil {
		return reqerr
	}

	a.Instances++
	return nil
}

func (a Application) Adapt() {
	if a.Desired == a.Instances {
		return
	} else {
		a.Scale()
	}
}

func (a Application) checkCPU(rule Policy) bool {
	if len(a.Stats) < rule.Samples {
		return false
	}
	stats := a.Stats[len(a.Stats)-rule.Samples:]
	var total float64

	for _, v := range stats {
		total += v.CpuUsage
	}
	return (total / float64(len(stats))) > rule.Threshold
}

func (a Application) checkMemory(rule Policy) bool {
	log.Print(fmt.Sprintf("Checking memory %s", a.Id))
	if len(a.Stats) < rule.Samples {
		return false
	}
	stats := a.Stats[len(a.Stats)-rule.Samples:]
	var total float64

	for _, v := range stats {
		total += v.MemUsage
	}
	return (total / float64(len(stats))) > rule.Threshold
}

func (a Application) CalibrateDesired() {
	log.Print(fmt.Sprintf("Calibrating %s", a.Id))
	for _, policy := range a.Policies {
		switch policy.Type {
		case "cpu":
			exceeding := a.checkCPU(policy)
			if exceeding {
				a.Desired++
				break
			}
		case "memory":
			exceeding := a.checkMemory(policy)
			if exceeding {
				a.Desired++
				break
			}
		}
	}
}

func GetDeltas(slave Task) (float64, float64) {
	url := fmt.Sprintf("http://%s:5050/monitor/statistics.json", slave.Host)
	response, err := Call("GET", url, "")
	if err != nil {
		return .0, .0
	}

	stat1 := MesosStat{}
	stat2 := MesosStat{}

	json.Unmarshal([]byte(response), stat1)
	time.Sleep(1 * time.Second) // Wait for 1 second to get a second sample to get current usage
	response2, err := Call("GET", url, "")
	if err != nil {
		return .0, .0
	}
	json.Unmarshal([]byte(response2), stat2)
	stat1CpuTotal := stat1.Statistics.CPUUserTimeSecs + stat1.Statistics.CPUSystemTimeSecs
	stat2CpuTotal := stat2.Statistics.CPUUserTimeSecs + stat2.Statistics.CPUSystemTimeSecs

	cpuDelta := stat2CpuTotal - stat1CpuTotal
	timestampDelta := stat2.Statistics.Timestamp - stat1.Statistics.Timestamp
	cpuUsage := (cpuDelta / timestampDelta) * 100

	memoryUsage := (stat2.Statistics.MemRSS / stat2.Statistics.MemLimit) * 100

	return cpuUsage, memoryUsage
}

func Average(arr []float64) float64 {
	var total float64

	for _, v := range arr {
		total += v
	}
	return total / float64(len(arr))
}

func (a Application) GetStatistics() (Stat, error) {
	var cpuUsage float64
	var memoryUsage float64

	var cpuValues []float64
	var memValues []float64

	log.Print(fmt.Sprintf("Getting stats of %s", a.Id))
	for _, slave := range a.Slaves {
		cpuUsage, memoryUsage = GetDeltas(slave)
		cpuValues = append(cpuValues, cpuUsage)
		memValues = append(memValues, memoryUsage)
	}
	cpuAvarage := Average(cpuValues)
	memAverage := Average(memValues)

	stat := Stat{
		CpuUsage: cpuAvarage,
		MemUsage: memAverage,
	}
	a.Stats = append(a.Stats, stat)

	return stat, nil
}

func GetAll() ([]Application, error) {
	response, err := Call("GET", "http://marathon.mesos:8080/api/v2/apps", "")
	var applications []Application

	if err != nil {
		return applications, err
	}

	jsonErr := json.Unmarshal([]byte(response), &applications)

	return applications, jsonErr
}

func Call(method string, url string, body string) (string, error) {

	req, httperr := http.NewRequest(method, url, bytes.NewBuffer([]byte(body)))

	if httperr != nil {
		log.Print("Calling error http %s", httperr)
		return "", httperr
	}

	client := http.Client{
		Timeout: time.Duration(3 * time.Second),
	}

	resp, reqerr := client.Do(req)
	if reqerr != nil {
		log.Print("Calling error %s", reqerr)
		return "", reqerr
	}
	defer resp.Body.Close()

	responseString, err := ioutil.ReadAll(resp.Body)
	return string(responseString), err
}

func (a Application) IsScalable() bool {
	pattern := "AUTOSCALING_[0-9]_RULE_[A-Z]*"
	for _, v := range a.Labels {
		found, _ := regexp.Match(pattern, []byte(v))
		if found {
			return true
		}
	}
	return false
}

func (a Application) SyncRules() {
	pattern := "AUTOSCALING_[0-9]_RULE_[A-Z]*"

	takenRules := make(map[string]bool)
	for k, _ := range a.Labels {
		if found, _ := regexp.Match(pattern, []byte(k)); found {
			ruleNumber := string(k[12])
			if _, k := takenRules[ruleNumber]; !k {
				t := fmt.Sprintf("AUTOSCALING_%d_RULE_TYPE", ruleNumber)
				threshold := fmt.Sprintf("AUTOSCALING_%d_RULE_THRESHOLD", ruleNumber)
				samples := fmt.Sprintf("AUTOSCALING_%d_RULE_SAMPLES", ruleNumber)
				interval := fmt.Sprintf("AUTOSCALING_%d_RULE_INTERVAL", ruleNumber)

				newrule := Policy{}

				typeVal, typeFound := a.Labels[t]
				thresholdVal, thresholdFound := a.Labels[threshold]
				samplesVal, samplesFound := a.Labels[samples]
				intervalVal, intervalFound := a.Labels[interval]

				if typeFound && thresholdFound && samplesFound && intervalFound {
					parsedThreshold, _ := strconv.ParseFloat(thresholdVal, 64)
					parsedInterval, _ := strconv.ParseInt(intervalVal, 10, 8)
					parsedSamples, _ := strconv.ParseInt(samplesVal, 10, 8)

					newrule.Type,
						newrule.Threshold,
						newrule.Samples,
						newrule.Interval = typeVal, parsedThreshold, int(parsedSamples), int(parsedInterval)
					a.Policies = append(a.Policies, newrule)
				}
				takenRules[ruleNumber] = true
				log.Print(fmt.Sprintf("Syncing table for %s %s", a.Id, newrule))
			}
		}
	}
}
