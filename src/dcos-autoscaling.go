package main

import (
	"dcosautoscaling"
	"log"
	"sync"
	"time"
)

func main() {
	for {

		allApplications, _ := dcosautoscaling.GetAll()
		managedApplications := GetScalable(allApplications)

		var wg sync.WaitGroup
		wg.Add(len(managedApplications))

		log.Printf("Managed applications %s", managedApplications)
		for _, application := range managedApplications {
			go func() {
				application.GetStatistics()
				application.CalibrateDesired()
				if application.Instances != application.Desired {
					application.Adapt()
				}
			}()
		}
		wg.Wait()
		time.Sleep(1 * time.Second)
		// Through a go routing for everyone of them to check and scale
	}
}

// Gets applications which are scalable
func GetScalable(applications []dcosautoscaling.Application) []dcosautoscaling.Application {
	var currentScalables []dcosautoscaling.Application
	for _, application := range applications {
		if application.IsScalable() {
			application.SyncRules()
			currentScalables = append(currentScalables, application)
		}
	}

	// Remove the deleted apps
	return currentScalables
}
