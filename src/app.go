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

	"github.com/kataras/iris"
	"github.com/robfig/cron"
)

const (
	adapterExtension = "http://adapter-extension.default.svc.cluster.local"
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
		fmt.Println(extension)
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

func main() {
	app := iris.Default()
	c := cron.New()
	var triggers Triggers
	err := getTriggerExtensions("OnTime", &triggers)
	if err != nil {
		return
	}
	// Create cron for each matching Extension
	for _, trigger := range triggers {
		trigger := trigger // https://github.com/golang/go/wiki/CommonMistakes#using-closures-with-goroutines
		c.AddJob(trigger.TriggerOn, Extensions(trigger.Extensions))
		fmt.Println("AddJob: ", trigger.TriggerOn, " -> Extensions:", len(trigger.Extensions))
	}
	c.Start()

	app.Get("/public/hc", func(ctx iris.Context) {
		ctx.JSON(iris.Map{
			"message": "OK",
		})
	})
	// listen and serve on http://0.0.0.0:8080.
	app.Run(iris.Addr(":8080"))
}
