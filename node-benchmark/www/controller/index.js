var app = angular.module('PerfDashApp', ['ngMaterial', 'chart.js', 'ui.router']);
var clearSeriesCharts = false;

app.config(function($stateProvider, $urlRouterProvider) {

        //$urlRouterProvider.otherwise('/tab/dash');
        $stateProvider
        .state('builds', {
            url: "/builds",
            templateUrl: "view/builds.html"
        })
        .state('comparison', {
            url: "/comparison",
            templateUrl: "view/comparison.html"
        })
        .state('series', {
            url: "/series",
            templateUrl: "view/series.html"
        })
        .state('config', {
            url: "/config",
            templateUrl: "view/config.html"
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
                case 3:
                    $location.url("/config");
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

    // for condig
    this.minBuild = 0;
    this.maxBuild = 0;
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
                //this.loadOverview();
            }.bind(this))
    .error(function(data) {
        console.log("error fetching result");
        console.log(data);
    });
};