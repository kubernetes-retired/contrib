/*
Copyright 2015 The Kubernetes Authors.

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

package nginx

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/mitchellh/mapstructure"
	"k8s.io/kubernetes/pkg/api"

	"k8s.io/contrib/ingress/controllers/nginx/nginx/config"
)

const (
	customHTTPErrors     = "custom-http-errors"
	skipAccessLogUrls    = "skip-access-log-urls"
	whitelistSourceRange = "whitelist-source-range"
)

// getDNSServers returns the list of nameservers located in the file /etc/resolv.conf
func getDNSServers() ([]string, error) {
	var nameservers []string
	file, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nameservers, err
	}

	// Lines of the form "nameserver 1.2.3.4" accumulate.
	lines := strings.Split(string(file), "\n")
	for l := range lines {
		trimmed := strings.TrimSpace(lines[l])
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "nameserver" {
			nameservers = append(nameservers, fields[1:]...)
		}
	}

	glog.V(3).Infof("nameservers to use: %v", nameservers)
	return nameservers, nil
}

// getConfigKeyToStructKeyMap returns a map with the ConfigMapKey as key and the StructName as value.
func getConfigKeyToStructKeyMap() map[string]string {
	keyMap := map[string]string{}
	n := &config.Configuration{}
	val := reflect.Indirect(reflect.ValueOf(n))
	for i := 0; i < val.Type().NumField(); i++ {
		fieldSt := val.Type().Field(i)
		configMapKey := strings.Split(fieldSt.Tag.Get("structs"), ",")[0]
		structKey := fieldSt.Name
		keyMap[configMapKey] = structKey
	}
	return keyMap
}

// splitConfigList splits a config param by "," and returns the slice.
func splitConfigList(conf *api.ConfigMap, key string) []string {
	s := make([]string, 0)
	if val, ok := conf.Data[key]; ok {
		delete(conf.Data, key)
		s = strings.Split(val, ",")
	}
	return s
}

// ReadConfig obtains the configuration defined by the user merged with the defaults.
func (ngx *Manager) ReadConfig(conf *api.ConfigMap) config.Configuration {
	if len(conf.Data) == 0 {
		return config.NewDefault()
	}

	cfgCM := config.Configuration{}
	cfgDefault := config.NewDefault()

	metadata := &mapstructure.Metadata{}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "structs",
		Result:           &cfgCM,
		WeaklyTypedInput: true,
		Metadata:         metadata,
	})

	cErrors := make([]int, 0)
	if val, ok := conf.Data[customHTTPErrors]; ok {
		delete(conf.Data, customHTTPErrors)
		for _, i := range strings.Split(val, ",") {
			j, err := strconv.Atoi(i)
			if err != nil {
				glog.Warningf("%v is not a valid http code: %v", i, err)
			} else {
				cErrors = append(cErrors, j)
			}
		}
	}

	cSkipUrls := splitConfigList(conf, skipAccessLogUrls)
	wl := splitConfigList(conf, whitelistSourceRange)

	err = decoder.Decode(conf.Data)
	if err != nil {
		glog.Infof("%v", err)
	}

	keyMap := getConfigKeyToStructKeyMap()

	valCM := reflect.Indirect(reflect.ValueOf(cfgCM))

	for _, key := range metadata.Keys {
		fieldName, ok := keyMap[key]
		if !ok {
			continue
		}

		valDefault := reflect.ValueOf(&cfgDefault).Elem().FieldByName(fieldName)

		fieldCM := valCM.FieldByName(fieldName)

		if valDefault.IsValid() {
			valDefault.Set(fieldCM)
		}
	}

	cfgDefault.CustomHTTPErrors = ngx.filterErrors(cErrors)
	cfgDefault.SkipAccessLogURLs = cSkipUrls
	cfgDefault.WhitelistSourceRange = wl
	// no custom resolver means use the system resolver
	if cfgDefault.Resolver == "" {
		cfgDefault.Resolver = ngx.defResolver
	}
	return cfgDefault
}

func (ngx *Manager) filterErrors(errCodes []int) []int {
	fa := make([]int, 0)
	for _, errCode := range errCodes {
		if errCode > 299 && errCode < 600 {
			fa = append(fa, errCode)
		} else {
			glog.Warningf("error code %v is not valid for custom error pages", errCode)
		}
	}

	return fa
}

func (ngx *Manager) needsReload(data []byte) (bool, error) {
	filename := ngx.ConfigFile
	in, err := os.Open(filename)
	if err != nil {
		return false, err
	}

	src, err := ioutil.ReadAll(in)
	in.Close()
	if err != nil {
		return false, err
	}

	if !bytes.Equal(src, data) {
		err = ioutil.WriteFile(filename, data, 0644)
		if err != nil {
			return false, err
		}

		dData, err := diff(src, data)
		if err != nil {
			glog.Errorf("error computing diff: %s", err)
			return true, nil
		}

		if glog.V(2) {
			glog.Infof("NGINX configuration diff a/%s b/%s\n", filename, filename)
			glog.Infof("%v", string(dData))
		}

		return len(dData) > 0, nil
	}

	return false, nil
}

func diff(b1, b2 []byte) (data []byte, err error) {
	f1, err := ioutil.TempFile("", "")
	if err != nil {
		return
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	f2, err := ioutil.TempFile("", "")
	if err != nil {
		return
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	f1.Write(b1)
	f2.Write(b2)

	data, err = exec.Command("diff", "-u", f1.Name(), f2.Name()).CombinedOutput()
	if len(data) > 0 {
		err = nil
	}
	return
}
