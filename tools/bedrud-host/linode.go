package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type linodeInstance struct {
	ID     int      `json:"id"`
	Label  string   `json:"label"`
	Status string   `json:"status"`
	IPv4   []string `json:"ipv4"`
}

type linodeList struct {
	Data []linodeInstance `json:"data"`
}

type linodeType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Price struct {
		Hourly  float64 `json:"hourly"`
		Monthly float64 `json:"monthly"`
	} `json:"price"`
}

func linodeTypePrice(c Config, typeID string) (hourly, monthly float64, err error) {
	if typeID == "" {
		return 0, 0, fmt.Errorf("empty linode type")
	}
	var t linodeType
	err = doJSON("GET", c.LinodeBase+"/linode/types/"+typeID, c.linodeHeaders(), nil, &t)
	if err != nil {
		return 0, 0, err
	}
	return t.Price.Hourly, t.Price.Monthly, nil
}

func fmtUSDRate(hourly, monthly float64, typeID string) string {
	if monthly <= 0 && hourly > 0 {
		monthly = hourly * 24 * 30
	}
	return fmt.Sprintf("$%.4f/hr  (~$%.2f/mo, %s)", hourly, monthly, typeID)
}

func pingLinode(c Config) error {
	var out map[string]any
	return doJSON("GET", c.LinodeBase+"/profile", c.linodeHeaders(), nil, &out)
}

func createLinode(c Config, label, rootPass string, pubKeys []string) (linodeInstance, error) {
	body := map[string]any{
		"region":          c.LinodeRegion,
		"type":            c.LinodeType,
		"image":           c.LinodeImage,
		"label":           label,
		"root_pass":       rootPass,
		"authorized_keys": pubKeys,
		"booted":          true,
		"tags":            []string{"bedrud"},
	}
	var inst linodeInstance
	err := doJSON("POST", c.LinodeBase+"/linode/instances", c.linodeHeaders(), body, &inst)
	return inst, err
}

func getLinode(c Config, id int) (linodeInstance, error) {
	var inst linodeInstance
	err := doJSON("GET", c.LinodeBase+"/linode/instances/"+strconv.Itoa(id), c.linodeHeaders(), nil, &inst)
	return inst, err
}

func listLinodes(c Config) ([]linodeInstance, error) {
	var list linodeList
	err := doJSON("GET", c.LinodeBase+"/linode/instances", c.linodeHeaders(), nil, &list)
	return list.Data, err
}

func deleteLinode(c Config, id int) error {
	err := doJSON("DELETE", c.LinodeBase+"/linode/instances/"+strconv.Itoa(id), c.linodeHeaders(), nil, nil)
	if err != nil && !isHTTPStatus(err, 404) {
		return err
	}
	return nil
}

func isHTTPStatus(err error, code int) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", code))
}

func waitLinodeGone(c Config, id int, tries int) error {
	if tries < 1 {
		tries = 48
	}
	for i := 0; i < tries; i++ {
		inst, err := getLinode(c, id)
		if err != nil {
			if isHTTPStatus(err, 404) {
				fmt.Printf("linode gone\n")
				return nil
			}
			return err
		}
		fmt.Printf("linode wait %d/%d status=%s\n", i+1, tries, inst.Status)
		time.Sleep(linodePoll)
	}
	return fmt.Errorf("linode %d still present after delete", id)
}

var linodePoll = 5 * time.Second

func waitLinodeIP(c Config, id int, tries int) (string, error) {
	var last linodeInstance
	for i := 0; i < tries; i++ {
		inst, err := getLinode(c, id)
		if err != nil {
			return "", err
		}
		last = inst
		if len(inst.IPv4) > 0 && inst.IPv4[0] != "" {
			return inst.IPv4[0], nil
		}
		time.Sleep(linodePoll)
	}
	return "", fmt.Errorf("linode %d has no ipv4 (status %s)", id, last.Status)
}

func findLinodeByLabelOrIP(c Config, label, ipv4 string) (linodeInstance, bool, error) {
	all, err := listLinodes(c)
	if err != nil {
		return linodeInstance{}, false, err
	}
	for _, inst := range all {
		if label != "" && inst.Label == label {
			return inst, true, nil
		}
		for _, ip := range inst.IPv4 {
			if ipv4 != "" && ip == ipv4 {
				return inst, true, nil
			}
		}
	}
	return linodeInstance{}, false, nil
}
