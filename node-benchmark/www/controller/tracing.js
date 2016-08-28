PerfDashApp.prototype.loadProbes = function() {
    if(this.build == null) {
        this.build = this.minBuild;
    }
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
    this.probes = Object.keys(dataItem.op_series);
}

PerfDashApp.prototype.plotBuildsTracing = function() {
    if(this.probeStart == null || this.probeEnd == null) {
        return;
    }
    latencyPercentiles = {
        'Perc50': [],
        'Perc90': [],
        'Perc99': [],
    }
    this.tracingBuilds = [];
    for (build = this.minBuild; build <= this.maxBuild; build++) { 
        startTimeData = this.extractTracingData(this.probeStart, build);
        endTimeData = this.extractTracingData(this.probeEnd, build);

        latency = arraySubstract(endTimeData, startTimeData).sort(function(a, b){return a-b});
        console.log(latency)

        latencyPercentiles['Perc50'].push(getPercentile(latency, 0.5));
        latencyPercentiles['Perc90'].push(getPercentile(latency, 0.9));
        latencyPercentiles['Perc99'].push(getPercentile(latency, 0.99));

        console.log(build)
        console.log(getPercentile(latency, 0.5))
        console.log(getPercentile(latency, 0.9))
        console.log(getPercentile(latency, 0.99))

        this.tracingBuilds.push(build);
    }
    console.log(JSON.stringify(latencyPercentiles));
    this.tracingData = [];
    this.tracingSeries = [];
    for(var metric in latencyPercentiles) {
        this.tracingData.push(latencyPercentiles[metric]);
        this.tracingSeries.push(metric);
    }
    this.tracingOptions = {
        scales: {
            xAxes: [{
                scaleLabel: {
                    display: true,
                    labelString: 'builds',
                }
            }],
            yAxes: [{
                scaleLabel: {
                    display: true,
                    labelString: 's',
                }
            }]
        }, 
        elements: {
            line: {
                fill: false,
            },
        },
        legend: {
            display: true,
        },
    };
}

var arraySubstract = function(arr1, arr2) {
    var diff = [];
    for(var i in arr1) {
        diff.push(parseInt(arr1[i] - arr2[i])/1000000000);
    }
    return diff;
}

var getPercentile = function(arr, perc) {
    return arr[Math.floor(arr.length*perc)];
}

PerfDashApp.prototype.extractTracingData = function(probe, build) {
    series = this.allData[this.test].data[this.node][build].series;
    dataItem = series[0];
    if(probe in dataItem.op_series) {
        return dataItem.op_series[probe];
    }

    // try following dataitems
    for(var i in series) {
        if(i == '0'){
            continue;
        }
        newDataItem = series[i];
        if(newDataItem.version == dataItem.version &&
            newDataItem.labels.test == dataItem.labels.test &&
            newDataItem.labels.node == dataItem.labels.node){
            if(probe in newDataItem.op_series) {
                return newDataItem.op_series[probe];
            }
        }
    }
}