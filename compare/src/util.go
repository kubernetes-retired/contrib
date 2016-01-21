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

package src

import (
	"fmt"
	"text/tabwriter"

	"github.com/daviddengcn/go-colortext"
	"math"
)

/* Slightly modified version of daviddengcn/go-colortext, so it'll work well with tabwriter */
func resetColor(writer *tabwriter.Writer) {
	fmt.Fprint(writer, "\x1b[0m")
}

func changeColor(color ct.Color, writer *tabwriter.Writer) {
	if color == ct.None {
		return
	}
	s := ""
	if color != ct.None {
		s = fmt.Sprintf("%s%d", s, 30+(int)(color-ct.Black))
	}

	s = "\x1b[0;" + s + "m"
	fmt.Fprint(writer, s)
}

func changeColorFloat64AndWrite(data, baseline, allowedVariance float64, enableOutputColoring bool, writer *tabwriter.Writer) {
	if enableOutputColoring {
		if data > baseline*allowedVariance {
			changeColor(ct.Red, writer)
		} else if math.IsNaN(data) {
			changeColor(ct.Yellow, writer)
		} else {
			// to keep tabwriter happy...
			changeColor(ct.White, writer)
		}
	}
	fmt.Fprintf(writer, "%.2f", data)
	if enableOutputColoring {
		resetColor(writer)
	}
}

func leq(left, right float64) bool {
	return left <= right || (math.IsNaN(left) && math.IsNaN(right))
}

// Prints build number injecting dummy colors to make cell align again
func printBuildNumber(buildNumber int, writer *tabwriter.Writer, enableOutputColoring bool) {
	if enableOutputColoring {
		changeColor(ct.White, writer)
	}
	fmt.Fprintf(writer, "%v", buildNumber)
	if enableOutputColoring {
		resetColor(writer)
	}
}
