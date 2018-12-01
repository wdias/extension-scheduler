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
)

const (
	adapterMetadata  = "http://adapter-metadata.default.svc.cluster.local"
	adapterExtension = "http://adapter-extension.default.svc.cluster.local"
)

// Timeseries blabla
type timeseries struct {
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
			TimeseriesID string `json:"timeseriesId"`
			VariableID   string `json:"variableId"`
		} `json:"variables"`
	} `json:"data"`
	Options json.RawMessage `json:"options"`
}

func getTimeseries(timeseriesID string, metadata *timeseries) error {
	fmt.Println("URL:", fmt.Sprint(adapterMetadata, "/timeseries/", timeseriesID))
	response, err := netClient.Get(fmt.Sprint(adapterMetadata, "/timeseries/", timeseriesID))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("Unable to find Timeseries: %q", timeseriesID)
	}
	err = json.Unmarshal(body, &metadata)
	return nil
}

func getExtensions(triggerType string, timeseriesID string, extensions *Extensions) error {
	fmt.Println("GET Extensions:", fmt.Sprint(" triggerType:", triggerType, " timeseriesID:", timeseriesID))
	path := fmt.Sprint(adapterExtension, "/extension/trigger_type/", triggerType)
	if timeseriesID != "" {
		path = fmt.Sprint(path, "?timeseriesId=", timeseriesID)
	}
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
		return fmt.Errorf("Unable to find Timeseries: %q", timeseriesID)
	}
	err = json.Unmarshal(body, &extensions)
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
	app.Get("/onchange/{timeseriesID:string}", func(ctx iris.Context) {
		timeseriesID := ctx.Params().Get("timeseriesID")
		fmt.Println("timeseriesID:", timeseriesID)
		var extensions Extensions
		err := getExtensions("OnChange", timeseriesID, &extensions)
		if err != nil {
			ctx.JSON(iris.Map{"response": err.Error()})
			return
		}
		for _, extension := range extensions {
			extensionURL := fmt.Sprint("http://extension-", strings.ToLower(extension.Extension), ".default.svc.cluster.local")
			jsonValue, _ := json.Marshal(extension)
			resp, err := netClient.Post(fmt.Sprint(extensionURL, "/extension/", strings.ToLower(extension.Extension), "/trigger/", extension.ExtensionID), "application/json", bytes.NewBuffer(jsonValue))
			if err != nil {
				fmt.Println("Error: Send to extension:", extensionURL, err)
			}
			defer resp.Body.Close()
			fmt.Println("Trigger ", extension.ExtensionID, resp.Body)
		}
		ctx.JSON(extensions)
	})

	app.Get("/public/hc", func(ctx iris.Context) {
		ctx.JSON(iris.Map{
			"message": "OK",
		})
	})
	// listen and serve on http://0.0.0.0:8080.
	app.Run(iris.Addr(":8080"))
}
