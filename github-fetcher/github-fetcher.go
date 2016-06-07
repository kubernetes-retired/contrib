/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/spf13/cobra"
	utilflag "k8s.io/kubernetes/pkg/util/flag"
)

type fetcherConfig struct {
	Client
	MySQLConfig

	once      bool
	frequency int
}

func addRootFlags(cmd *cobra.Command, config *fetcherConfig) {
	cmd.PersistentFlags().IntVar(&config.frequency, "frequency", 2, "Number of iterations per hour")
	cmd.PersistentFlags().BoolVar(&config.once, "once", false, "Run once and then leave")
}

func runProgram(config *fetcherConfig) error {
	db, err := config.CreateDatabase()
	if err != nil {
		return err
	}

	for {
		begin := time.Now()
		tx := db.Begin()
		if err = UpdateIssues(tx, config); err != nil {
			tx.Rollback()
			return err
		}
		if err = UpdateIssueEvents(tx, config); err != nil {
			tx.Rollback()
			return err
		}
		tx.Commit()

		if config.once {
			break
		}

		runTime := time.Now().Sub(begin)
		time.Sleep(time.Hour/time.Duration(config.frequency) - runTime)
	}

	return nil
}

func main() {
	config := &fetcherConfig{}
	root := &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "Fetches github database: Pull-requests, issues, and events",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runProgram(config)
		},
	}
	root.SetGlobalNormalizationFunc(utilflag.WordSepNormalizeFunc)
	addRootFlags(root, config)
	config.Client.AddFlags(root)
	config.MySQLConfig.AddFlags(root)
	if err := root.Execute(); err != nil {
		glog.Fatalf("%v\n", err)
	}
}
