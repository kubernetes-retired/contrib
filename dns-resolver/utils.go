/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"text/template"
)

const (
	hexDigit = "0123456789abcdef"
)

func mergeTemplate(tmpl, output string, data map[string]interface{}) error {
	w, err := os.Create(output)
	if err != nil {
		return err
	}
	defer w.Close()

	t, err := template.ParseFiles(tmpl)
	if err != nil {
		return err
	}

	return t.Execute(w, data)
}

func buildPtr(ip string) string {
	ptr, err := reverseaddr(ip)
	if err != nil {
		return ""
	}

	return ptr
}

func buildPtrDomain(ip string) string {
	ptr := buildPtr(ip)
	idx := strings.Index(ptr, ".")
	return ptr[idx+1:]
}

// reverseaddr returns the in-addr.arpa. or ip6.arpa. hostname of the IP
// address addr suitable for rDNS (PTR) record lookup or an error if it fails
// to parse the IP address.
func reverseaddr(addr string) (arpa string, err error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", &net.DNSError{Err: "unrecognized address", Name: addr}
	}
	if ip.To4() != nil {
		return strconv.FormatUint(uint64(ip[15]), 10) + "." + strconv.FormatUint(uint64(ip[14]), 10) + "." + strconv.FormatUint(uint64(ip[13]), 10) + "." + strconv.FormatUint(uint64(ip[12]), 10) + ".in-addr.arpa.", nil
	}
	// Must be IPv6
	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	// Add it, in reverse, to the buffer
	for i := len(ip) - 1; i >= 0; i-- {
		v := ip[i]
		buf = append(buf, hexDigit[v&0xF])
		buf = append(buf, '.')
		buf = append(buf, hexDigit[v>>4])
		buf = append(buf, '.')
	}
	// Append "ip6.arpa." and return (buf already has the final .)
	buf = append(buf, "ip6.arpa."...)
	return string(buf), nil
}

func checksum(files []string) string {
	hasher := sha256.New()

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return ""
		}
		io.Copy(hasher, f)
		f.Close()
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))
}
