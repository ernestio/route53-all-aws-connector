/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	ecc "github.com/ernestio/ernest-config-client"
	"github.com/nats-io/nats"
	uuid "github.com/satori/go.uuid"
)

var nc *nats.Conn
var natsErr error

func eventHandler(m *nats.Msg) {
	var e Event

	err := e.Process(m.Subject, m.Data)
	if err != nil {
		println(err.Error())
		return
	}

	if err = e.Validate(); err != nil {
		e.Error(err)
		return
	}

	parts := strings.Split(m.Subject, ".")
	switch parts[1] {
	case "create":
		err = createRoute53(&e)
	case "update":
		err = updateRoute53(&e)
	case "delete":
		err = deleteRoute53(&e)
	}

	if err != nil {
		e.Error(err)
		return
	}

	e.Complete()
}

func getZoneRecords(ev *Event) ([]*route53.ResourceRecordSet, error) {
	svc := getRoute53Client(ev)

	req := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(ev.HostedZoneID),
	}

	resp, err := svc.ListResourceRecordSets(req)
	if err != nil {
		return nil, err
	}

	return resp.ResourceRecordSets, nil
}

func buildResourceRecords(values []string) []*route53.ResourceRecord {
	var records []*route53.ResourceRecord

	for _, v := range values {
		records = append(records, &route53.ResourceRecord{
			Value: aws.String(v),
		})
	}

	return records
}

func isDefaultRule(name string, record *route53.ResourceRecordSet) bool {
	return entryName(*record.Name) == entryName(name) && *record.Type == "SOA" ||
		entryName(*record.Name) == entryName(name) && *record.Type == "NS"
}

func buildRecordsToRemove(ev *Event, existing []*route53.ResourceRecordSet) []*route53.Change {
	// Dont delete the default NS and SOA rules
	// May conflict with non-default rules, needs testing

	var missing []*route53.Change

	for _, recordSet := range existing {

		if ev.Records.HasRecord(*recordSet.Name) != true && isDefaultRule(ev.Name, recordSet) != true {
			missing = append(missing, &route53.Change{
				Action:            aws.String("DELETE"),
				ResourceRecordSet: recordSet,
			})
		}
	}

	return missing
}

func buildChanges(ev *Event, existing []*route53.ResourceRecordSet) []*route53.Change {
	var changes []*route53.Change

	for _, record := range ev.Records {
		changes = append(changes, &route53.Change{
			Action: aws.String("UPSERT"),
			ResourceRecordSet: &route53.ResourceRecordSet{
				Name:            aws.String(record.Entry),
				Type:            aws.String(record.Type),
				TTL:             aws.Int64(record.TTL),
				ResourceRecords: buildResourceRecords(record.Values),
			},
		})
	}

	changes = append(changes, buildRecordsToRemove(ev, existing)...)

	return changes
}

func createRoute53(ev *Event) error {
	svc := getRoute53Client(ev)

	req := &route53.CreateHostedZoneInput{
		CallerReference: aws.String(uuid.NewV4().String()),
		Name:            aws.String(ev.Name),
	}

	if ev.Private == true {
		req.HostedZoneConfig = &route53.HostedZoneConfig{
			PrivateZone: aws.Bool(ev.Private),
		}
		req.VPC = &route53.VPC{
			VPCId:     aws.String(ev.VPCID),
			VPCRegion: aws.String(ev.DatacenterRegion),
		}
	}

	resp, err := svc.CreateHostedZone(req)
	if err != nil {
		return err
	}

	ev.HostedZoneID = *resp.HostedZone.Id

	return updateRoute53(ev)
}

func updateRoute53(ev *Event) error {
	svc := getRoute53Client(ev)

	zr, err := getZoneRecords(ev)
	if err != nil {
		return err
	}

	req := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: buildChanges(ev, zr),
		},
		HostedZoneId: aws.String(ev.HostedZoneID),
	}

	_, err = svc.ChangeResourceRecordSets(req)
	if err != nil {
		return err
	}

	return err
}

func deleteRoute53(ev *Event) error {
	// clear ruleset before delete
	ev.Records = nil
	err := updateRoute53(ev)
	if err != nil {
		return err
	}

	svc := getRoute53Client(ev)

	req := &route53.DeleteHostedZoneInput{
		Id: aws.String(ev.HostedZoneID),
	}

	_, err = svc.DeleteHostedZone(req)

	return err
}

func getRoute53Client(ev *Event) *route53.Route53 {
	creds := credentials.NewStaticCredentials(ev.DatacenterSecret, ev.DatacenterToken, "")
	return route53.New(session.New(), &aws.Config{
		Region:      aws.String(ev.DatacenterRegion),
		Credentials: creds,
	})
}

func main() {
	nc = ecc.NewConfig(os.Getenv("NATS_URI")).Nats()

	fmt.Println("listening for route53.create.aws")
	nc.Subscribe("route53.create.aws", eventHandler)

	fmt.Println("listening for route53.update.aws")
	nc.Subscribe("route53.update.aws", eventHandler)

	fmt.Println("listening for route53.delete.aws")
	nc.Subscribe("route53.delete.aws", eventHandler)

	runtime.Goexit()
}
