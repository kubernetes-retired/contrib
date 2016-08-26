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

// draw a throughput-latency plot
PerfDashApp.prototype.loadOverview = function() {
    // selection
    pods = 105;
    intervalList = [0]; // [0, 100, 200, 300];
    image = 'containervm-v20160321';
    machine = 'n1-standard-1';
    metric = 'Perc100';

    testTmpl = 'density_create_batch_' + pods + '_0_';
    node = image + '/' + machine;

    for(var id in intervalList) {
        test = testTmpl + intervalList[id];
        data = this.allData[test].data[node];
        builds = Object.keys(data);
        // latest
        build = builds[builds.length - 1];
        perfBuild = data[build].perf;

        console.log(JSON.stringify(perfBuild));
    }
}