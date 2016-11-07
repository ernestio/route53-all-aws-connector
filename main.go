/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	ecc "github.com/ernestio/ernest-config-client"
	"github.com/nats-io/nats"
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

func createRoute53(ev *Event) (err error) {
	return err
}

func updateRoute53(ev *Event) (err error) {
	return err
}

func deleteRoute53(ev *Event) (err error) {
	return err
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
