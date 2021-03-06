/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
)

var (
	// ErrDatacenterIDInvalid : error for invalid datacenter id
	ErrDatacenterIDInvalid = errors.New("Datacenter VPC ID invalid")
	// ErrDatacenterRegionInvalid : error for datacenter revgion invalid
	ErrDatacenterRegionInvalid = errors.New("Datacenter Region invalid")
	// ErrDatacenterCredentialsInvalid : error for datacenter credentials invalid
	ErrDatacenterCredentialsInvalid = errors.New("Datacenter credentials invalid")
	// ErrZoneNameInvalid : error for zone name invalid
	ErrZoneNameInvalid = errors.New("Route53 zone name invalid")
)

// Records stores a collection of records
type Records []Record

// Record stores the entries for a zone
type Record struct {
	Entry  string   `json:"entry"`
	Type   string   `json:"type"`
	Values []string `json:"values"`
	TTL    int64    `json:"ttl"`
}

// Event stores the route53 data
type Event struct {
	UUID             string  `json:"_uuid"`
	BatchID          string  `json:"_batch_id"`
	ProviderType     string  `json:"_type"`
	HostedZoneID     string  `json:"hosted_zone_id"`
	Name             string  `json:"name"`
	Private          bool    `json:"private"`
	Records          Records `json:"records"`
	VPCID            string  `json:"vpc_id"`
	DatacenterName   string  `json:"datacenter_name,omitempty"`
	DatacenterRegion string  `json:"datacenter_region"`
	DatacenterToken  string  `json:"datacenter_token"`
	DatacenterSecret string  `json:"datacenter_secret"`
	ErrorMessage     string  `json:"error_message,omitempty"`
	action           string
}

func entryName(entry string) string {
	if string(entry[len(entry)-1]) == "." {
		return entry[:len(entry)-1]
	}
	return entry
}

// HasRecord returns true if a matched entry is found
func (r Records) HasRecord(entry string) bool {
	// check with removed . character as well
	for _, record := range r {
		if entryName(record.Entry) == entryName(entry) {
			return true
		}
	}
	return false
}

// Validate checks if all criteria are met
func (ev *Event) Validate() error {
	if ev.VPCID == "" {
		return ErrDatacenterIDInvalid
	}

	if ev.DatacenterRegion == "" {
		return ErrDatacenterRegionInvalid
	}

	if ev.DatacenterSecret == "" || ev.DatacenterToken == "" {
		return ErrDatacenterCredentialsInvalid
	}

	if ev.Name == "" {
		return ErrZoneNameInvalid
	}

	return nil
}

// Process the raw event
func (ev *Event) Process(subject string, data []byte) error {
	ev.action = strings.Split(subject, ".")[1]

	err := json.Unmarshal(data, &ev)
	if err != nil {
		nc.Publish("route53."+ev.action+".aws.error", data)
	}
	return err
}

// Error the request
func (ev *Event) Error(err error) {
	log.Printf("Error: %s", err.Error())
	ev.ErrorMessage = err.Error()

	data, err := json.Marshal(ev)
	if err != nil {
		log.Panic(err)
	}
	nc.Publish("route53."+ev.action+".aws.error", data)
}

// Complete the request
func (ev *Event) Complete() {
	data, err := json.Marshal(ev)
	if err != nil {
		ev.Error(err)
	}
	nc.Publish("route53."+ev.action+".aws.done", data)
}
