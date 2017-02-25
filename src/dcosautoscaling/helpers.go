package dcosautoscaling

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func FindTaskStat(task Task, stats []MesosStat) (MesosStat, error) {
	for _, v := range stats {
		if v.ExecutorId == task.Id {
			return v, nil
		}
	}
	return MesosStat{}, ApplicationError{fmt.Sprint("Could not find the task ", task)}
}

func Average(arr []float64) float64 {
	var total float64

	for _, v := range arr {
		total += v
	}
	if len(arr) == 0 {
		return 0
	}

	return total / float64(len(arr))
}

func Call(method string, url string, body string) (string, error) {

	req, httperr := http.NewRequest(method, url, bytes.NewBuffer([]byte(body)))

	if httperr != nil {
		log.Printf("http creating http request to %s: ", url, httperr)
		return "", httperr
	}

	client := http.Client{
		Timeout: time.Duration(3 * time.Second),
	}

	resp, reqerr := client.Do(req)

	if reqerr != nil {
		log.Printf("Calling executing http request to %s: ", url, reqerr)
		return "", reqerr
	}
	defer resp.Body.Close()

	responseString, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return "", nil
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		error := HTTPResponseError{fmt.Sprintf("Response code is %d", resp.StatusCode)}
		log.Printf("A no success response calling %s: %d ", url, resp.StatusCode)
		return string(responseString), error
	}

	return string(responseString), nil
}
