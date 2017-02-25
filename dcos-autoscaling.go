package main

import (
	"dcosautoscaling"
	"fmt"
	"log"
	"sync"
	"time"
)

func main() {
	var managedApplications []dcosautoscaling.Application
	for {

		allApplications, _ := dcosautoscaling.GetAll()
		GetScalable(allApplications, &managedApplications)

		var wg sync.WaitGroup
		log.Print(fmt.Sprintf("Got %d Managed apps ", len(managedApplications)))
		wg.Add(len(managedApplications))

		for k, application := range managedApplications {
			go func(wg *sync.WaitGroup) {
				application.SyncRules()
				application.GetStatistics()
				application.CalibrateDesired()
				if application.Instances != application.Desired {
					application.Adapt()
				}
				managedApplications[k] = application
				wg.Done()
			}(&wg)
		}
		wg.Wait()
		time.Sleep(1 * time.Second)
	}
}

// Gets applications which are scalable
func GetScalable(applications []dcosautoscaling.Application, currentScalables *[]dcosautoscaling.Application) {
	knownLookups := make(map[string]bool)
	for _, v := range *currentScalables {
		knownLookups[v.Id] = true
	}
	allLookups := make(map[string]bool)
	for _, v := range applications {
		if v.Instances > 0 {
			allLookups[v.Id] = true
		}
	}

	var remainingScalables []dcosautoscaling.Application

	// Remove the deleted apps and reset the
	for _, knownApp := range *currentScalables {
		_, found := allLookups[knownApp.Id]

		lastConfigChange, _ := time.Parse(time.RFC3339, knownApp.VersionInfo.LastConfigChangeAt)

		knownApp.GetAppDetails()
		configChanged := int(lastConfigChange.Unix()) > knownApp.UpdatedAt

		if !found || configChanged { // App has just changed
			log.Printf("App %s is deleted or has it's configurations changed", knownApp.Id)
			delete(knownLookups, knownApp.Id)
		} else {
			remainingScalables = append(remainingScalables, knownApp)
		}
	}

	for _, application := range applications {
		_, found := knownLookups[application.Id]
		if !found && application.IsScalable() && application.Instances > 0 {
			application.InitializeScalable()
			remainingScalables = append(remainingScalables, application)
		}
	}

	*currentScalables = remainingScalables
}
