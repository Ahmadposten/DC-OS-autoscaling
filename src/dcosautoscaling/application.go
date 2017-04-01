package dcosautoscaling

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	SCALE_UP     = "increase"
	SCALE_DOWN   = "decrease"
	GREATER_THAN = "gt"
	LESS_THAN    = "lt"
)

type ApplicationInterface interface {
	Scale() error
	GetStatistics() (Stat, error)
}

type ApplicationError struct {
	s string
}

func (e ApplicationError) Error() string {
	return e.s
}

type Application struct {
	Id          string `json:"id"`
	UpdatedAt   int
	Slaves      []Task `json:"tasks"`
	Min         int
	Max         int
	Cooldown    int
	Stats       []Stat
	Instances   int `json:"instances"`
	Desired     int
	Policies    []Policy
	Labels      map[string]string
	VersionInfo struct {
		LastScalingAt      string `json:"lastScalingAt"`
		LastConfigChangeAt string `json:"lastConfigChangeAt"`
	} `json:"versionInfo"`
}

func (a *Application) GetAppDetails() error {
	url := fmt.Sprintf("http://marathon.mesos:8080/v2/apps%s", a.Id)

	response, err := Call("GET", url, "")
	if err != nil {
		return err
	}
	marathonApp := MarathonAppResponse{}

	err = json.Unmarshal([]byte(response), &marathonApp)
	if err != nil {
		log.Printf("An error unmarshalling json getting app details", err)
		return err
	}
	a.Slaves = marathonApp.App.Slaves
	a.VersionInfo = marathonApp.App.VersionInfo

	return nil
}

func (a *Application) Scale() error {
	log.Printf("Trying to scale %s", a.Id)
	lastScaledAt, _ := time.Parse(time.RFC3339, a.VersionInfo.LastScalingAt)

	timeSinceLastScaling := int(time.Now().Unix() - lastScaledAt.Unix())
	if timeSinceLastScaling < a.Cooldown {
		log.Printf("Can not scale application %s since it was last scaled %s seconds ago ", a.Id, timeSinceLastScaling)
		return nil
	}

	url := fmt.Sprintf("http://marathon.mesos:8080/v2/apps%s", a.Id)

	body := fmt.Sprintf("{\"instances\":%d}", a.Desired)
	response, reqerr := Call("PUT", url, body)

	log.Printf("Response from scaling %s is %s", a.Id, response)

	if reqerr != nil {
		return reqerr
	}

	a.Instances = a.Desired
	return nil
}

func (a *Application) Adapt() {
	if a.Desired == a.Instances {
		return
	} else {
		a.Scale()
	}
}

func (a *Application) CheckCPU(rule Policy) bool {
	if len(a.Stats) < (rule.Samples * rule.Interval) {
		// Since we take measurements every second we need to account for the interval
		log.Printf("Not doing a check now since we have only %d "+
			"stats while it required %d sample for an %d interval",
			len(a.Stats), rule.Samples, rule.Interval)
		return false
	}
	stats := a.Stats[len(a.Stats)-(rule.Samples*rule.Interval):]
	var total float64

	for _, v := range stats {
		total += v.CpuUsage
	}

	var trigger bool

	switch rule.Operator {
	case GREATER_THAN:
		trigger = (total / float64(len(stats))) > rule.Threshold
	case LESS_THAN:
		trigger = (total / float64(len(stats))) < rule.Threshold
	}

	if trigger {
		log.Printf("App %s failed a cpu rule %f %s %d ... doing: %s",
			a.Id, (total / float64(len(stats))),
			rule.Operator, rule.Threshold, rule.Action)
	}
	return trigger
}

func (a *Application) CheckMemory(rule Policy) bool {
	log.Printf("Checking %s's memory", a.Id)

	if len(a.Stats) < (rule.Samples * rule.Interval) {
		return false
	}
	stats := a.Stats[len(a.Stats)-(rule.Samples*rule.Interval):]
	var total float64

	for _, v := range stats {
		total += v.MemUsage
	}

	var trigger bool
	switch rule.Operator {
	case GREATER_THAN:
		trigger = (total / float64(len(stats))) > rule.Threshold
	case LESS_THAN:
		trigger = (total / float64(len(stats))) < rule.Threshold
	}

	if trigger {
		log.Printf("App %s failed a memory rule %f %s %d ... doing: %s",
			a.Id, (total / float64(len(stats))),
			rule.Operator, rule.Threshold, rule.Action)
	}
	return trigger
}

func (a *Application) CalibrateDesired() {
	log.Printf("Calibrating desired for app: %s", a.Id)

	for _, policy := range a.Policies {
		trigger := a.CheckPolicy(policy)
		switch policy.Type {
		case "cpu":
			trigger = a.CheckCPU(policy)
		case "memory":
			trigger = a.CheckMemory(policy)
		}
		if trigger {
			switch policy.Action {
			case SCALE_UP:
				if a.Desired <= a.Instances && a.Desired < a.Max {
					a.Desired += policy.Step
				}
				break
			case SCALE_DOWN:
				if a.Desired >= a.Instances && a.Desired > a.Min {
					a.Desired -= policy.Step
				}
				break
			}
		}
	}
}

func (a *Application) CheckPolicy(rule Policy) bool {
	if rule.Type == "cpu" {
		return a.CheckCPU(rule)
	} else {
		return a.CheckMemory(rule)
	}
}

func GetDeltas(slave Task) (float64, float64, error) {
	url := fmt.Sprintf("http://%s:5051/monitor/statistics.json", slave.Host)
	response, err := Call("GET", url, "")
	if err != nil {
		return .0, .0, err
	}

	allStats1 := []MesosStat{}
	allStats2 := []MesosStat{}

	json.Unmarshal([]byte(response), &allStats1)

	stat1, err1 := FindTaskStat(slave, allStats1)
	if err1 != nil {
		return 0.0, 0.0, err1
	}

	time.Sleep(1 * time.Second) // Wait for 1 second to get a second sample to get current usage
	response2, err := Call("GET", url, "")
	if err != nil {
		return .0, .0, err
	}
	json.Unmarshal([]byte(response2), &allStats2)

	stat2, err2 := FindTaskStat(slave, allStats2)
	if err2 != nil {
		return 0.0, 0.0, err2
	}
	stat1CpuTotal := stat1.Statistics.CPUUserTimeSecs + stat1.Statistics.CPUSystemTimeSecs
	stat2CpuTotal := stat2.Statistics.CPUUserTimeSecs + stat2.Statistics.CPUSystemTimeSecs

	cpuDelta := stat2CpuTotal - stat1CpuTotal

	timestampDelta := stat2.Statistics.Timestamp - stat1.Statistics.Timestamp
	cpuUsage := (cpuDelta / timestampDelta) * 100

	memoryUsage := (stat2.Statistics.MemRSS / stat2.Statistics.MemLimit) * 100

	return cpuUsage, memoryUsage, nil
}

func (a *Application) GetStatistics() (Stat, error) {
	var cpuValues []float64
	var memValues []float64

	log.Printf("Getting stats of %s", a.Id)
	var wg sync.WaitGroup

	wg.Add(len(a.Slaves))
	for _, slave := range a.Slaves {
		go func(wg *sync.WaitGroup) {
			cpuUsage, memoryUsage, err := GetDeltas(slave)
			log.Print("Deltas are ", cpuUsage, memoryUsage)
			if err != nil {
				wg.Done()
			} else {
				cpuValues = append(cpuValues, cpuUsage)
				memValues = append(memValues, memoryUsage)
				wg.Done()
			}
		}(&wg)
	}
	wg.Wait()
	cpuAvarage := Average(cpuValues)
	memAverage := Average(memValues)

	stat := Stat{
		CpuUsage: cpuAvarage,
		MemUsage: memAverage,
	}
	log.Printf("A new stat %f %f", cpuAvarage, memAverage)

	a.Stats = append(a.Stats, stat)
	return stat, nil
}

func GetAll() ([]Application, error) {
	response, err := Call("GET", "http://marathon.mesos:8080/v2/apps", "")
	var applications []Application

	if err != nil {
		log.Printf("Error getting all applications ", err)
		return applications, err
	}

	marathonApps := MarathonAppsResponse{}
	jsonErr := json.Unmarshal([]byte(response), &marathonApps)

	applications = marathonApps.Apps
	return applications, jsonErr
}

func (a *Application) InitializeScalable() {

	a.UpdatedAt = int(time.Now().Unix())

	minStringVal, hasMin := a.Labels["AUTOSCALING_MIN_INSTANCES"]

	cooldown, hasCoolDown := a.Labels["AUTOSCALING_COOLDOWN_PERIOD"]
	maxStringVal, hasMax := a.Labels["AUTOSCALING_MAX_INSTANCES"]

	if hasCoolDown {
		intCoolDown, _ := strconv.ParseInt(cooldown, 10, 64)
		a.Cooldown = int(intCoolDown)
	} else {
		a.Cooldown = 300 // Default cooldown is 5 mins
	}

	a.Desired = a.Instances
	if hasMin {
		parsedMin, _ := strconv.ParseInt(minStringVal, 10, 64)
		a.Min = int(parsedMin)
	} else {
		a.Min = a.Instances
	}

	if hasMax {
		parsedMax, _ := strconv.ParseInt(maxStringVal, 10, 64)
		a.Max = int(parsedMax)
	} else {
		a.Max = a.Instances
	}
	a.GetAppDetails()
}

func (a Application) IsScalable() bool {
	for k, _ := range a.Labels {
		if k == "AUTOSCALABLE" {
			return true
		}
	}
	return false
}

func (a *Application) SyncRules() {
	pattern := "AUTOSCALING_[0-9]_RULE_[A-Z]*"

	takenRules := make(map[string]bool)
	var policies []Policy

	log.Printf("Syncing rules table for %s: ", a.Id)

	for k, _ := range a.Labels {
		if found, _ := regexp.Match(pattern, []byte(k)); found {
			ruleNumber := string(k[12])
			if _, k := takenRules[ruleNumber]; !k {
				t := fmt.Sprintf("AUTOSCALING_%s_RULE_TYPE", ruleNumber)
				threshold := fmt.Sprintf("AUTOSCALING_%s_RULE_THRESHOLD", ruleNumber)
				samples := fmt.Sprintf("AUTOSCALING_%s_RULE_SAMPLES", ruleNumber)
				interval := fmt.Sprintf("AUTOSCALING_%s_RULE_INTERVAL", ruleNumber)
				action := fmt.Sprintf("AUTOSCALING_%s_RULE_ACTION", ruleNumber)
				operator := fmt.Sprintf("AUTOSCALING_%s_RULE_OPERATOR", ruleNumber)
				step := fmt.Sprintf("AUTOSCALING_%s_RULE_STEP", ruleNumber)

				newrule := Policy{}

				typeVal, typeFound := a.Labels[t]
				thresholdVal, thresholdFound := a.Labels[threshold]
				samplesVal, samplesFound := a.Labels[samples]
				intervalVal, intervalFound := a.Labels[interval]
				actionVal, actionFound := a.Labels[action]
				operatorVal, operatorFound := a.Labels[operator]
				stepVal, stepFound := a.Labels[step]

				if typeFound && thresholdFound &&
					samplesFound && intervalFound &&
					actionFound && operatorFound {
					parsedThreshold, _ := strconv.ParseFloat(thresholdVal, 64)
					parsedInterval, _ := strconv.ParseInt(intervalVal, 10, 64)
					parsedSamples, _ := strconv.ParseInt(samplesVal, 10, 64)

					var parsedStep int64
					if stepFound {
						parsedStep, _ = strconv.ParseInt(stepVal, 10, 64)
					} else {
						parsedStep = 1
					}

					newrule.Type,
						newrule.Threshold, newrule.Samples,
						newrule.Interval, newrule.Action,
						newrule.Operator, newrule.Step = typeVal, parsedThreshold,
						int(parsedSamples), int(parsedInterval),
						actionVal, operatorVal, int(parsedStep)

					policies = append(policies, newrule)
				}
				takenRules[ruleNumber] = true
			}
		}
	}
	a.Policies = policies
}
