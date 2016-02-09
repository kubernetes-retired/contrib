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

var app = angular.module('PerfDashApp', ['ngMaterial', 'chart.js']);

var PerfDashApp = function(http, scope) {
    this.http = http;
    this.scope = scope;
    this.selectedResource = "pods";
    this.selectedVerb = "GET";
    this.testNames = [];
    this.series = [ "Perc99", "Perc90", "Perc50" ];
    this.allData = null;
};

PerfDashApp.prototype.onClick = function(data) {
    console.log(data);
    window.location = "http://kubekins.dls.corp.google.com/job/kubernetes-e2e-gce-scalability/" + data[0].label + "/"
};

// Fetch data from the server and update the data to display
PerfDashApp.prototype.refresh = function() {
    this.http.get("api")
    .success(function(data) {
        this.testNames = Object.keys(data);
        this.testName = this.testNames[0];
        this.allData = data;
        this.testNameChanged();
        this.labels = this.getLabels();
        this.resources = this.getResources();
        this.verbs = this.getVerbs();
    }.bind(this))
    .error(function(data) {
        console.log("error fetching api");
        console.log(data);
    });
};

// Update the data to graph, using the selected resource and verb
PerfDashApp.prototype.resourceChanged = function() {
    this.seriesData = [
        this.getStream(this.getData(this.selectedResource, this.selectedVerb), "Perc99"),
        this.getStream(this.getData(this.selectedResource, this.selectedVerb), "Perc90"),
        this.getStream(this.getData(this.selectedResource, this.selectedVerb), "Perc50")
    ];
};

// Update the data to graph, using the selected testName
PerfDashApp.prototype.testNameChanged = function() {
    this.data = this.allData[this.testName];
    this.resourceChanged();
};

// Get the set of all resources (e.g. 'pods') in the data set
PerfDashApp.prototype.getResources = function() {
    var set = {};
    angular.forEach(this.data, function(value, key) {
        angular.forEach(value, function(v, k) {
            set[k] = true;
        })
    });
    var result = [];
    angular.forEach(set, function(value, key) {
        result.push(key);
    })
    return result;
};

// Get the set of all verbs (e.g. 'GET') in the data set
PerfDashApp.prototype.getVerbs = function() {
    var set = {};
    angular.forEach(this.data, function(value, key) {
        angular.forEach(value, function(v, k) {
            angular.forEach(v, function(val) {
                set[val.verb] = true;
            });
        });
    });
    var result = [];
    angular.forEach(set, function(value, key) {
        result.push(key);
    })
    return result;
};


// Get all of the possible labels for the data set (e.g. build numbers)
PerfDashApp.prototype.getLabels = function() {
    var result = [];
    angular.forEach(this.data, function(value, key) {
        result.push(key);
    })
    return result;
};

// Extract a time series of latency data for a specific object type
// 'object' the type of object to extract data for (e.g. 'pods')
// 'verb' the verb to extract data for (e.g. 'GET')
PerfDashApp.prototype.getData = function(object, verb) {
    var result = [];
    angular.forEach(this.data, function(value, key) {
        var dataSet = value[object];
        angular.forEach(dataSet, function(latency) {
            if (latency.verb == verb) {
                result.push(latency);
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
        result.push(value.latency[stream] / 1000000);
    });
    return result;
};

app.controller('AppCtrl', ['$scope', '$http', '$interval', function($scope, $http, $interval) {
    $scope.controller = new PerfDashApp($http, $scope);
    $scope.controller.refresh();

    // Refresh every 60 secs.  The data only refreshes every 10 minutes on the server
    $interval($scope.controller.refresh.bind($scope.controller), 60000) 
}]);
