package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	fsnotify "github.com/fsnotify/fsnotify"
	influxdb "github.com/influxdata/influxdb-client-go"
)

var INFLUX_TOKEN string

func DBConnect() (*influxdb.Client, error) {
	// You can generate a Token from the "Tokens Tab" in the UI
	client := http.Client{}
	return influxdb.New("https://us-central1-1.gcp.cloud2.influxdata.com", INFLUX_TOKEN, influxdb.WithHTTPClient(&client))
}

type SensorReport struct {
	CO2         float64 `json:"CO2"`
	Temperature float64 `json:"Temperature"`
}

func main() {
	DBConnect()
	// we use client.NewRowMetric for the example because it's easy, but if you need extra performance
	// it is fine to manually build the []client.Metric{}.
	influx, err := DBConnect()
	if err != nil {
		panic(err)
	}
	watch_sensors(influx)
	// The actual write..., this method can be called concurrently.
}

func watch_sensors(influx *influxdb.Client) {

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("ERROR", err)
	}
	defer watcher.Close()

	//
	done := make(chan bool)

	//
	go func() {
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				fmt.Printf("EVENT! %#v\n", event)
				readings, err := os.Open("sensor-readings.json")
				if err != nil {
					log.Fatal(err)
				}
				defer readings.Close()
				var report SensorReport
				fileBytes, err := ioutil.ReadAll(readings)
				if err != nil {
					log.Fatal(err)
				}
				json.Unmarshal(fileBytes, &report)
				fmt.Printf("%+v\n", report)
				myMetric := []influxdb.Metric{
					influxdb.NewRowMetric(
						map[string]interface{}{"CO2": report.CO2, "temperature": report.Temperature},
						"Sensor Readings",
						map[string]string{"Hostname": "TestBox1"},
						time.Now()),
				}
				_, err = influx.Write(context.Background(), "my-test-bucket", "833c7fbc1d19c9be", myMetric...)
				if err != nil {
					log.Fatal(err) // as above use your own error handling here.
				}
				watch_sensors(influx)
				// watch for errors
			case err := <-watcher.Errors:
				fmt.Println("ERROR", err)
			}
		}
	}()

	// out of the box fsnotify can watch a single file, or a single directory
	if err := watcher.Add("sensor-readings.json"); err != nil {
		fmt.Println("ERROR", err)
	}

	<-done
}
