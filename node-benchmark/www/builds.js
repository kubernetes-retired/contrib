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

// Plots in dashboard
var plots = new Set(['latency', 'kubelet_cpu', 'kubelet_memory', 'runtime_cpu', 'runtime_memory']);
var seriesPlots = new Set(['latency', 'kubelet_cpu', 'kubelet_memory', 'runtime_cpu', 'runtime_memory'])

// Metrics to plot in each plot
var plotRules = {
    'latency': ['Perc50', 'Perc90', 'Perc99'],
    'kubelet_cpu': ['Perc50', 'Perc90', 'Perc99'],
    'kubelet_memory': ['memory', 'rss', 'workingset'],
    'runtime_cpu': ['Perc50', 'Perc90', 'Perc99'],
    'runtime_memory': ['memory', 'rss', 'workingset'],    
}

// Rules to parse test options
var testOptions = {
    'density': {
        options: ['opertation', 'mode', 'pods', 'interval (ms)', 'background pods', 'stress'],
        remark: '',
    },
    'resource': {
        options: ['pods'],
        remark: '',
    },   
}

var app = angular.module('PerfDashApp', ['ngMaterial', 'chart.js', 'ui.router']);
var clearSeriesCharts = false;

app.config(function($stateProvider, $urlRouterProvider) {

        $urlRouterProvider.otherwise('/tab/dash');
        $stateProvider
        .state('builds', {
            url: "/builds",
            templateUrl: "partials/builds.html"
        })
        .state('comparison', {
            url: "/comparison",
            templateUrl: "partials/comparison.html"
        })
        .state('series', {
            url: "/series",
            templateUrl: "partials/series.html"
        });
    });

app.controller('AppCtrl', ['$scope', '$http', '$interval', '$location', 
    function($scope, $http, $interval, $location) {
    $scope.controller = new PerfDashApp($http, $scope);
    $scope.controller.refresh();
    // Refresh every 5 min.  The data only refreshes every 10 minutes on the server
    $interval($scope.controller.refresh.bind($scope.controller), 300000);

    $scope.selectedIndex = 0;
    $scope.$watch('selectedIndex', function(current, old) {
            switch (current) {
                case 0:
                    $location.url("/builds");
                    break;
                case 1:
                    $location.url("/comparison");
                    break;
                case 2:
                    $location.url("/series");
                    break;
            }
            if(old == 2) { // clear charts for time series plot
                console.log("clear")
                clearSeriesCharts = true;
            }
        });
}]);

var PerfDashApp = function(http, scope) {
    this.http = http;
    this.scope = scope;
    this.onClick = this.onClickInternal_.bind(this);

    // machine/image/test to plot is not defined at beginning
    //this.node = 'undefined';
    //this.image = 'undefined';
    //this.test = 'undefined';
    //this.testType = 'undefined';
    this.minBuild = 0;

    // Data/option for charts
    this.seriesMap = {};
    this.seriesDataMap = {};
    this.optionsMap = {};
    this.buildsMap = {};

    // tests contain full test names
    this.tests = [];
    // testOptionMap contains options for each test type
    this.testOptionTreeRoot = {};
    this.testOptions = {};
    this.testTypes = [];
    this.testSelectedOptions = {};

    this.testNodeTreeRoot = {};
    this.testHostList = [];

    // comparisonList contains all tests for comparison
    this.comparisonList = [];
    this.comparisonListSelected = [];

    // aggregation in test comparison
    this.aggregateTypes = ['latest', 'average'];
    this.aggregateType = 'latest';

    // for comparison data
    this.comparisonSeriesMap = {};
    this.comparisonSeriesDataMap = {};
    this.comparisonOptionsMap = {};
    this.comparisonLabelsMap = plotRules;

    // for time series
    this.seriesCharts = {};
};

// TODO(coufon): not handled for benchmark yet
PerfDashApp.prototype.onClickInternal_ = function(data) {
    console.log(data);
    // Get location
    // TODO(random-liu): Make the URL configurable if we want to support more
    // buckets in the future.
    window.location = "http://kubekins.dls.corp.google.com/job/" + this.job + "/" + data[0].label + "/";
};

// Fetch data from the server and update the data to display
PerfDashApp.prototype.refresh = function() {
    // get test information
    this.http.get("info")
            .success(function(data) {
                this.testInfo = data;
            }.bind(this))
    .error(function(data) {
        console.log("error fetching result");
        console.log(data);
    });
    // get test data
    this.http.get("api")
            .success(function(data) {
                this.tests = Object.keys(data);
                this.allData = data;
                this.parseTest()
                this.parseNodeInfo();
                this.testChanged();
            }.bind(this))
    .error(function(data) {
        console.log("error fetching result");
        console.log(data);
    });
};

// Parse test information
PerfDashApp.prototype.parseTest = function() {
    angular.forEach(this.tests, function(test) {
        parts = test.split("_");

        treeNode = this.testOptionTreeRoot
        angular.forEach(parts, function(part) {
            if(!(part in treeNode)) {
                treeNode[part] = {}; // new node
            }
            treeNode = treeNode[part]; // next level
        }, this);
    }, this);
    this.testTypes = Object.keys(this.testOptionTreeRoot);
};

// Change test options selection when test type is changed
PerfDashApp.prototype.testTypeChanged = function() {
    if(!this.testType) {
        return;
    }
    this.testSelectedOptions = {}
    this.testOptions = {}

    options = testOptions[this.testType].options;
    treeNode = this.testOptionTreeRoot[this.testType];
    keys = Object.keys(treeNode);
    option = options[0];
    this.testOptions[option] = keys;
    this.testSelectedOptions[option] = keys[0]; // init value   
    this.testOptionChanged(option);
}

// Select test options
PerfDashApp.prototype.testOptionChanged = function(name) {
    // set initial values
    treeNode = this.testOptionTreeRoot[this.testType];
    options = testOptions[this.testType].options;
    toChange = false;
    for(var i in options) {
        option = options[i];
        if(toChange) {
            keys = Object.keys(treeNode);
            this.testOptions[option] = keys;
            if(keys.length == 0) {
                break;
            }
            selected = keys[0]; // init value
            this.testSelectedOptions[option] = selected;
        } else {
            selected = this.testSelectedOptions[option];
        }
        treeNode = treeNode[selected];
        if(option == name) {
            toChange = true;
        }
    }

    this.test = this.testType;
    for(var i in options) {
        option = options[i];
        selected = this.testSelectedOptions[option];
        if(!selected) {
            break;
        }
        //console.log(selected)
        this.test += '_' + selected
    }
    this.testChanged();
    //console.log(this.test)
}

// Parse 'machine' and 'image' labels from 'node'
PerfDashApp.prototype.parseNodeInfo = function() {
    angular.forEach(this.allData, function(test, testName) {
        if(!(testName in this.testNodeTreeRoot)) {
            this.testNodeTreeRoot[testName] = {};
        }

        angular.forEach(test.data, function(nodeData, nodeName) {
            pair = nodeName.split("-e2e-node-")
            machine = pair[0];
            image = pair[1];
            newNodeName = image + '/' + machine;
            test.data[newNodeName] = nodeData;

            // make selection of machine/image/host here
            treeNode = this.testNodeTreeRoot[testName];
            if(!(image in treeNode)) {
                treeNode[image] = {};
            }
            treeNode = treeNode[image];
            if(!(machine in treeNode)) {
                treeNode[machine] = {};
            }            

            delete test.data[nodeName];
        }, this);
    }, this);
};


// Apply new data to charts, using the selected test, reflect the changes to test options
PerfDashApp.prototype.testChangedWrapper = function() {
    this.testChanged();

    parts = this.test.split('_');

    this.testType = parts[0];
    options = testOptions[this.testType].options;
    treeNode = this.testOptionTreeRoot[this.testType];

    selecteds = parts.slice(1, parts.length);
    for(var i in selecteds) {
        selected = selecteds[i];
        option = options[i];
        this.testSelectedOptions[option] = selected;
        this.testOptions[option] = Object.keys(treeNode);
        treeNode = treeNode[selected]
    }
};

// Apply new data to charts, using the selected test
PerfDashApp.prototype.testChanged = function() {
    if(!this.test | !(this.test in this.allData)) {
        return;
    }
    this.imageList = Object.keys(this.testNodeTreeRoot[this.test]);

    //this.job = this.allData[this.test].job;

    //console.log(JSON.stringify(this.allData))
    
    this.nodeChanged()
};

// Apply new data to charts, using the selected node (machine/image)
PerfDashApp.prototype.nodeChanged = function() {
    if(!this.image) {
        return;
    } else if(!this.machine){
        this.machineList = Object.keys(this.testNodeTreeRoot[this.test][this.image]);
        return;
    }
    this.node = this.image + '/' + this.machine;
    this.data = this.allData[this.test].data[this.node];
    
    this.labels = this.getLabels();
    this.builds = this.getBuilds();
    this.labelChanged();
};

// Update the data to charts, using selected labels
PerfDashApp.prototype.labelChanged = function() {
    // get data for each plot
    angular.forEach(plots, function(plot){
        this.seriesDataMap[plot] = [];
        this.seriesMap[plot] = [];
        this.buildsMap[plot] = [];
        switch(plot) {
            case 'latency':
                selectedLabels = {
                    'datatype': 'latency',
                };
                break;
            case 'kubelet_cpu':
                selectedLabels = {
                    'datatype': 'resource',
                    'container': 'kubelet',
                    'resource': 'cpu',
                };
                break;
            case 'kubelet_memory':
                selectedLabels = {
                    'datatype': 'resource',
                    'container': 'kubelet',
                    'resource': 'memory',
                };
                break;
            case 'runtime_cpu':
                selectedLabels = {
                    'datatype': 'resource',
                    'container': 'runtime',
                    'resource': 'cpu',
                };
                break;
            case 'runtime_memory':
                selectedLabels = {
                    'datatype': 'resource',
                    'container': 'runtime',
                    'resource': 'memory',
                };
                break;
            default:
                console.log('unkown plot type ' + plot)
                return;              
        }
        //selectedLabels['node'] = this.node;
        result = this.getData(selectedLabels, this.buildsMap[plot]);
        //console.log(JSON.stringify(this.buildsMap[plot]))
        if (Object.keys(result).length <= 0) {
            return;
        }
        // All the unit should be the same
        this.optionsMap[plot] = {
            scales: {
                xAxes: [{
                    scaleLabel: {
                        display: true,
                        labelString: 'Build',
                    }
                }],
                yAxes: [{
                    scaleLabel: {
                        display: true,
                        labelString: result[0].unit,
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
        angular.forEach(plotRules[plot], function(metric) {
            this.seriesDataMap[plot].push(this.getStream(result, metric));
            this.seriesMap[plot].push(metric);
        }, this);
    }, this)
};

// Get all of the builds for the data set (e.g. build numbers)
PerfDashApp.prototype.getBuilds = function() {
    return Object.keys(this.data)
};

// Get the set of all labels (e.g. 'resources', 'verbs') in the data set
PerfDashApp.prototype.getLabels = function() {
    var set = {};
    angular.forEach(this.data, function(items, build) {
        angular.forEach(items.perf, function(item) {
            angular.forEach(item.labels, function(label, name) {
                if (set[name] == undefined) {
                    set[name] = {}
                }
                set[name][label] = true
            });
        });
    });

    this.selectedLabels = {}
    var labels = {};
    angular.forEach(set, function(items, name) {
        labels[name] = [];
        angular.forEach(items, function(ignore, item) {
            if (this.selectedLabels[name] == undefined) {
              this.selectedLabels[name] = item;
            }
            labels[name].push(item)
        }, this);
    }, this);
    return labels;
};

// Extract a time series of data for specific labels
PerfDashApp.prototype.getData = function(labels, builds) {
    var result = [];
    angular.forEach(this.data, function(items, build) {
        angular.forEach(items.perf, function(item) {
            var match = true;
            angular.forEach(labels, function(label, name) {
                if (item.labels[name] != label) {
                    match = false;
                }
            });
            if (match && builds[builds.length-1] != build) {
                result.push(item);
                builds.push(build)
            }
        });
    });
    return result;
};

// Given a slice of data, turn it into a time series of numbers
// 'data' is an array of APICallLatency objects
// 'stream' is a selector for latency data, (e.g. 'Perc50')
PerfDashApp.prototype.getStream = function(data, stream) {
    var result = [];
    angular.forEach(data, function(value) {
        result.push(value.data[stream]);
    });
    return result;
};