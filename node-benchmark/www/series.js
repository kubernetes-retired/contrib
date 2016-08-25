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

PerfDashApp.prototype.buildChanged = function() {
    // search for the selected node
    series = this.allData[this.test].data[this.node][this.build].series;
    dataItem = series[0];
    // merge following dataitems
    for(var i in series) {
        if(i == '0'){
            continue;
        }
        newDataItem = series[i];
        //console.log(JSON.stringify(newDataItem))
        if(newDataItem.version == dataItem.version &&
           newDataItem.labels.test == dataItem.labels.test &&
           newDataItem.labels.node == dataItem.labels.node){
               if(newDataItem.op_series != null) {
                   for(var k in newDataItem.op_series) {
                       dataItem.op_series[k] = newDataItem.op_series[k];
                   }
               }
                if(newDataItem.resource_series != null) {
                   for(var k in newDataItem.resource_series) {
                       dataItem.resource_series[k] = newDataItem.resource_series[k];
                   }
               }
           }
    }
    //console.log(JSON.stringify(dataItem))
    this.timeseries = dataItem.resource_series;
    this.latencySeriesMap = dataItem.op_series;

    this.plotTimeSeries();
    this.seriesBuildLabels = dataItem.labels;
}

// Plot the time series data for the selected build
PerfDashApp.prototype.plotTimeSeries = function() {
    // align timeline
    var start = Math.min(this.timeseries['kubelet']['ts'][0],
        this.timeseries['runtime']['ts'][0])
    // get data for each plot
    angular.forEach(seriesPlots, function(plot){
        var ctx, dataSets;

        switch(plot) {
            case 'latency':
                dataSets = [{ 
                        label: 'create_test',
                        data: getHistSeries(this.latencySeriesMap['create'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        })),
                        backgroundColor: 'rgba(51,153,255,0.3)',
                    },
                    { 
                        label: 'running_test',
                        data: getHistSeries(this.latencySeriesMap['running'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        })),
                        backgroundColor: 'rgba(0,204,102,0.3)',
                    },
                    { 
                        label: 'firstSeen_kublet',
                        data: getHistSeries(this.latencySeriesMap['PodCreatefirstSeen'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        })),
                        backgroundColor: 'rgba(30,164,40,0.3)',
                    },
                    { 
                        label: 'running_kublet',
                        data: getHistSeries(this.latencySeriesMap['PodCreateRunning'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        })),
                        backgroundColor: 'rgba(0,20,102,0.3)',
                    },
                    /*
                    { 
                        label: 'container',
                        data: getHistSeries(this.latencySeriesMap['container'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        })),
                        backgroundColor: 'rgba(1000,20,0,0.3)',
                    },
                    */
                ];
                unit = "#Pod"
                ctx = document.getElementById("series_latency").getContext("2d");
                break;
            case 'kubelet_cpu':
                dataSets = [{ 
                    label: 'resource',
                    data: combineSeries(
                        this.timeseries['kubelet']['ts'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        }), 
                        this.timeseries['kubelet']['cpu']
                    ),
                }];
                unit = this.timeseries['kubelet']['unit']['cpu']
                ctx = document.getElementById("series_kubelet_cpu").getContext("2d");
                break;
            case 'kubelet_memory':
                dataSets = [{ 
                    label: 'resource',
                    data: combineSeries(
                        this.timeseries['kubelet']['ts'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        }), 
                        this.timeseries['kubelet']['memory']
                    ),
                }];
                unit = this.timeseries['kubelet']['unit']['memory']
                ctx = document.getElementById("series_kubelet_memory").getContext("2d");
                break;
            case 'runtime_cpu':
                dataSets = [{ 
                    label: 'resource',
                    data: combineSeries(
                        this.timeseries['runtime']['ts'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        }), 
                        this.timeseries['runtime']['cpu']
                    ),
                }];
                unit = this.timeseries['runtime']['unit']['cpu'];
                ctx = document.getElementById("series_runtime_cpu").getContext("2d");
                break;
            case 'runtime_memory':
                dataSets = [{ 
                    label: 'resource',
                    data: combineSeries(
                        this.timeseries['runtime']['ts'].map(function(value){
                            return ((value - start)/1e9).toFixed(1);
                        }), 
                        this.timeseries['runtime']['memory']
                    ),
                }];
                unit = this.timeseries['runtime']['unit']['memory'];
                ctx = document.getElementById("series_runtime_memory").getContext("2d");
                break;
            default:
                console.log('unkown plot type ' + plot);
                return;              
        }

        if(clearSeriesCharts) {
            this.seriesCharts = {};
            clearSeriesCharts = false;
        }

        if(plot in this.seriesCharts) {
            console.log("update")
            this.seriesCharts[plot].data.datasets = dataSets;
            this.seriesCharts[plot].update();
        } else {
            console.log("new")
            this.seriesCharts[plot] = new Chart(ctx, {
                type: 'line',
                data: {
                    datasets: dataSets,
                },
                options: {
                    scales: {
                        xAxes: [{
                            type: 'linear',
                            position: 'bottom',
                            scaleLabel: {
                                display: true,
                                labelString: 'time(s)',
                            }
                        }],
                        yAxes: [{
                            scaleLabel: {
                                display: true,
                                labelString: unit,
                            }
                        }],
                    },
                    legend: {
                        display: (plot == 'latency')?true:false,
                    }
                }
            });
        } 
    }, this)
};

var combineSeries = function(s0, s1) {
    if(s0.length != s1.length) {
        console.log("Series length mismatch.");
        return;
    }
    var ret = [];
    for(var i in s0) {
        ret.push({
            x: s0[i],
            y: s1[i],
        })
    }
    return ret
}

var getHistSeries = function(s0) {
    var ret = [];
    var sum = 0;
    for(var i in s0) {
        ret.push({
            x: s0[i],
            y: ++sum,
        });
    }
    return ret    
}