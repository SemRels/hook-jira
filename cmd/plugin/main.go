// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package main

import (
	"log"

	plugin "github.com/SemRels/hook-jira/internal/plugin"
)

func main() {
	client := plugin.NewClient(plugin.Config{})
	log.Printf("hook-jira plugin ready: creates Jira releases and versions (%T)", client)
}
