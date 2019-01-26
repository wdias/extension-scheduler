package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/kataras/iris"
	"github.com/robfig/cron"
)

const (
	adapterExtension = "http://adapter-extension.default.svc.cluster.local"
	schedulerList    = "trigger_scheduler"
	timeGap          = 3
)

// Timeseries Timeseries structure with timeseriesId and metadataIds
type Timeseries struct {
	TimeseriesID   string `json:"timeseriesId"`
	ModuleID       string `json:"moduleId"`
	ValueType      string `json:"valueType"`
	ParameterID    string `json:"parameterId"`
	LocationID     string `json:"locationId"`
	TimeseriesType string `json:"timeseriesType"`
	TimeStepID     string `json:"timeStepId"`
}

// Extensions blabla
type Extensions []struct {
	ExtensionID string `json:"extensionId"`
	Extension   string `json:"extension"`
	Function    string `json:"function"`
	Data        struct {
		InputVariables  []string `json:"inputVariables"`
		OutputVariables []string `json:"outputVariables"`
		Variables       []struct {
			Timeseries Timeseries `json:"timeseries"`
			VariableID string     `json:"variableId"`
		} `json:"variables"`
	} `json:"data"`
	Options json.RawMessage `json:"options"`
}

// Triggers Extensions per each Unique OnTime trigger
type Triggers []struct {
	TriggerOn  string     `json:"trigger_on"`
	Extensions Extensions `json:"extensions"`
}

// Run Post a trigger request when cron job executed
func (extensions Extensions) Run() {
	for _, extension := range extensions {
		extensionSVC := fmt.Sprint("http://extension-", strings.ToLower(extension.Extension), ".default.svc.cluster.local")
		extensionURL := fmt.Sprint(extensionSVC, "/extension/", strings.ToLower(extension.Extension), "/trigger/", extension.ExtensionID)
		jsonValue, _ := json.Marshal(extension)
		resp, err := netClient.Post(extensionURL, "application/json", bytes.NewBuffer(jsonValue))
		if err != nil {
			fmt.Println("Error: Send to extension:", extensionURL, err.Error())
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			fmt.Println("Send request failed:", extensionURL, err.Error())
		}
		fmt.Println("Trigger ", extension.ExtensionID, extensionURL)
	}
}

func getTriggerExtensions(triggerType string, triggers *Triggers) error {
	fmt.Println("GET Extensions:", fmt.Sprint(" triggerType:", triggerType))
	path := fmt.Sprint(adapterExtension, "/extension/trigger_type/", triggerType)
	response, err := netClient.Get(path)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("Unable to find OnTime Triggers")
	}
	err = json.Unmarshal(body, &triggers)
	return nil
}

var tr = &http.Transport{
	MaxIdleConns:       10,
	IdleConnTimeout:    30 * time.Second,
	DisableCompression: true,
	Dial: (&net.Dialer{
		Timeout: 5 * time.Second,
	}).Dial,
	TLSHandshakeTimeout: 5 * time.Second,
}
var netClient = &http.Client{
	Transport: tr,
	Timeout:   time.Second * 10,
}
var redisClient = redis.NewClient(&redis.Options{
	Addr:     "adapter-redis-master.default.svc.cluster.local:6379",
	Password: "wdias123", // no password set
	DB:       1,          // use default DB
})

func main() {
	app := iris.Default()
	app.Get("/public/hc", func(ctx iris.Context) {
		ctx.JSON(iris.Map{
			"message": "OK",
		})
	})
	// listen and serve on http://0.0.0.0:8080.
	go app.Run(iris.Addr(":8080"))

	cronPrev := cron.New()
	c := cron.New()
	cSeconds := 0
	ondemandCount := 100 // Sync at the startup & run the cron.Start() first time

	for {
		time.Sleep(timeGap * time.Second)
		// Read and create Jobs for newly created Extensions
		scheduleTriggers, err := redisClient.LPop(schedulerList).Result()
		if err == redis.Nil {
			fmt.Println("Nothing new scheduled.")
		} else if err != nil {
			fmt.Println("Retrieve new schedule failed:", err.Error())
		} else {
			var newTriggers Triggers
			err = json.Unmarshal([]byte(scheduleTriggers), &newTriggers)
			for _, trigger := range newTriggers {
				trigger := trigger // https://github.com/golang/go/wiki/CommonMistakes#using-closures-with-goroutines
				c.AddJob(trigger.TriggerOn, Extensions(trigger.Extensions))
				fmt.Println("AddNewJob: ", trigger.TriggerOn, " -> Extensions:", len(trigger.Extensions))
			}
			ondemandCount++
		}
		// After certain period of time or exceed amount of creating separate Jobs, discard all and place a new cron jobs
		cSeconds++
		if !(ondemandCount > 10 || cSeconds*timeGap > 60) {
			continue
		}
		fmt.Println("Sync OnTime Schedule...")
		var triggers Triggers
		err = getTriggerExtensions("OnTime", &triggers)
		if err != nil {
			continue
		}
		cronPrev = c
		c = cron.New()
		// Create cron for each matching Extension
		for _, trigger := range triggers {
			trigger := trigger // https://github.com/golang/go/wiki/CommonMistakes#using-closures-with-goroutines
			c.AddJob(trigger.TriggerOn, Extensions(trigger.Extensions))
			fmt.Println("AddJob: ", trigger.TriggerOn, " -> Extensions:", len(trigger.Extensions))
		}
		c.Start()
		cronPrev.Stop()
		ondemandCount = 0
		cSeconds = 0
	}
}
