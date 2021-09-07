package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/michael-go/go-jsn/jsn"
	"github.com/tidwall/sjson"
)

// IssuesFilter is the top level filter type of filtering Snyk response
type IssuesFilter struct {
	Filters Filter `json:"filters"`
}

// PriorityScoreRange defines the range of priority for which to retrieve issues
type PriorityScoreRange struct {
	Minimum int `json:"min"`
	Maximum int `json:"max"`
}

// Filter allows to filter on severity, type, ignore or patched vuln
type Filter struct {
	Severities      []string           `json:"severities"`
	ExploitMaturity []string           `json:"exploitMaturity,omitempty"`
	PriorityScore   PriorityScoreRange `json:"priorityScore"`
	Types           []string           `json:"types"`
	Ignored         bool               `json:"ignored"`
	Patched         bool               `json:"patched"`
}

func getVulnsWithoutTicket(endpointAPI string, orgID string, projectID string, token string, severity string, maturityFilter []string, priorityScoreThreshold int, issueType string, tickets map[string]string) map[string]interface{} {

	body := IssuesFilter{
		Filter{
			Severities: []string{"high"},
			Types:      []string{"vuln", "license"},
			Ignored:    false,
			Patched:    false,
		},
	}
	if issueType != "all" && issueType != "" {
		body.Filters.Types = []string{issueType}
	}
	switch severity {
	case "critical":
		body.Filters.Severities = []string{"critical"}
	case "high":
		body.Filters.Severities = []string{"critical", "high"}
	case "medium":
		body.Filters.Severities = []string{"critical", "high", "medium"}
	case "low":
		body.Filters.Severities = []string{"critical", "high", "medium", "low"}
	default:
		log.Fatalln("Unexpected severity threshold")
	}
	if len(maturityFilter) > 0 {
		body.Filters.ExploitMaturity = maturityFilter
	}
	body.Filters.PriorityScore.Minimum = 0
	body.Filters.PriorityScore.Maximum = 1000
	if priorityScoreThreshold > 0 {
		body.Filters.PriorityScore.Minimum = priorityScoreThreshold
	}
	marshalledBody, err := json.Marshal(body)

	if err != nil {
		log.Fatalln(err)
	}

	responseAggregatedData := makeSnykAPIRequest("POST", endpointAPI+"/v1/org/"+orgID+"/project/"+projectID+"/aggregated-issues", token, marshalledBody)
	j, err := jsn.NewJson(responseAggregatedData)
	var vulnsPerPath map[string]interface{}
	for _, e := range j.K("issues").Array().Elements() {
		if _, found := tickets[e.K("id").String().Value]; !found {
			bytes, err := json.Marshal(e)
			if err != nil {
				log.Fatalln(err)
			}
			json.Unmarshal(bytes, &vulnsPerPath)
			var issueId = e.K("id").String().Value
			ProjectIssuePathData := makeSnykAPIRequest("GET", endpointAPI+"/v1/org/"+orgID+"/project/"+projectID+"/issue/"+issueId+"/paths", token, nil)
			k, er := jsn.NewJson(ProjectIssuePathData)
			if er != nil {
				log.Fatalln(er)
			}
			vulnsPerPath["from"] = k.K("paths").Stringify()
		}
	}
	for _, e := range j.K("issues").K("licenses").Array().Elements() {
		if _, found := tickets[e.K("id").String().Value]; !found {
			bytes, err := json.Marshal(e)
			if err != nil {
				log.Fatalln(err)
			}
			json.Unmarshal(bytes, &vulnsPerPath)
			var issueId = e.K("id").String().Value
			ProjectIssuePathData := makeSnykAPIRequest("GET", endpointAPI+"/v1/org/"+orgID+"/project/"+projectID+"/issue/"+issueId+"/paths", token, nil)
			k, er := jsn.NewJson(ProjectIssuePathData)
			if er != nil {
				log.Fatalln(er)
			}
			vulnsPerPath["from"] = k.K("paths").Stringify()
		}
	}

	return vulnsPerPath
}

func consolidateAllPathsIntoSingleVuln(vulnsPerPath []interface{}) map[string]interface{} {
	vulnsWithAllPaths := make(map[string]interface{})

	for _, vuln := range vulnsPerPath {
		//vuln := vulnPerPath
		vulnJSON, _ := jsn.NewJson(vuln)

		if _, found := vulnsWithAllPaths[vulnJSON.K("id").String().Value]; !found {
			// Changing "from": ["a","b","c"] to "from": [["a","b","c"]]
			fmt.Println("id found is: ", vulnJSON.K("id").String().Value)
			var vulnJSONPaths [][]string
			var vulnJSONPath []string
			for _, value := range vulnJSON.K("from").Array().Elements() {
				vulnJSONPath = append(vulnJSONPath, value.Stringify())
			}
			fmt.Println("path found is: ", vulnJSONPath)
			vulnJSONPaths = append(vulnJSONPaths, vulnJSONPath)
			// Modify json with the "from" array change
			vuln, _ = sjson.Set(vulnJSON.Stringify(), "from", vulnJSONPaths)

		} else {
			var vulnJSONPath []string
			for _, value := range vulnJSON.K("from").Array().Elements() {
				vulnJSONPath = append(vulnJSONPath, value.Stringify())
			}
			vulnToAddPathTo, _ := jsn.NewJson(vulnsWithAllPaths[vulnJSON.K("id").String().Value])
			// from.-1 appends to the end of the array
			vuln, _ = sjson.Set(vulnToAddPathTo.Stringify(), "from.-1", vulnJSONPath)
		}
		// Update the vuln with changes
		vulnsWithAllPaths[vulnJSON.K("id").String().Value] = vuln
	}
	return vulnsWithAllPaths
}

func createMaturityFilter(filtersArray []string) []string {

	var MaturityFilter []string

	for _, filter := range filtersArray {
		switch filter {
		case "no-data":
			MaturityFilter = append(MaturityFilter, filter)
		case "no-known-exploit":
			MaturityFilter = append(MaturityFilter, filter)
		case "proof-of-concept":
			MaturityFilter = append(MaturityFilter, filter)
		case "mature":
			MaturityFilter = append(MaturityFilter, filter)
		case "":
		default:
			log.Fatalf("INPUT ERROR: %s is not a valid maturity level. Levels are Must be one of [no-data,no-known-exploit,proof-of-concept,mature]", filter)
		}
	}
	return MaturityFilter
}
