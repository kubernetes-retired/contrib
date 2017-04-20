"use strict";
var app = angular.module('BulkLGTMModule', ['ngMaterial', 'angularMoment']);

app.config(['$compileProvider', function ($compileProvider) {
  $compileProvider.debugInfoEnabled(false);
}]);

app.controller('BulkLGTMCntl', ['DataService', '$interval', '$location', BulkLGTMCntl]);

function BulkLGTMCntl(dataService, $interval, $location) {
  var self = this;
  self.prs = {};
  self.prsCount = 0;
  self.health = {};
  self.metadata = {};
  self.testResults = {};
  self.tabLoaded = {};
  self.functionLoading = {};
  self.queryNum = 0;
  self.selected = 1;
  self.OverallHealth = "";
  self.StatOrder = "-Count";
  self.location = $location;
  self.lgtm = lgtm;

  dataService.getData("bulkprs/prs").then(function success(response) {
    self.prs = response.data;
  }, function error(response) {
    console.log("Error: Getting pr data: " + response);
  });

  // Request Avatars that are only as large necessary (CSS limits them to 40px)
  function fixPRAvatars(prs) {
    angular.forEach(prs, function(pr) {
      if (/^https:\/\/avatars.githubusercontent.com\/u\/\d+\?v=3$/.test(pr.AvatarURL)) {
        pr.AvatarURL += '&size=40';
      }
    });
  }

  function refreshPRs() {
    dataService.getData('prs').then(function successCallback(response) {
      var prs = response.data.PRStatus;
      fixPRAvatars(prs);
      self.prs = prs;
      self.prsCount = Object.keys(prs).length;
      self.prSearchTerms = getPRSearchTerms();
    }, function errorCallback(response) {
      console.log("Error: Getting SubmitQueue Status");
    });
  }

  function refreshMetadata() {
    dataService.getData('metadata').then(function successCallback(response) {
      var metadata = response.data;
      self.metadata = metadata;
    }, function errorCallback(response) {
      console.log("Error: Getting MetaData for SubmitQueue");
    });
  }

  function lgtm(number) {
    dataService.getData("bulkprs/lgtm?number=" + number).then(function success(response) {
      for (var i = 0; i < self.prs.length; i++) {
        if (self.prs[i].number == number) {
          self.prs.splice(i, 1);
          return;
        }
      }
    }, function error(response) {
      console.log("Error LGTM-ing PR: " + response);
    });
  }
}

app.filter('loginOrPR', function() {
  return function(prs, searchVal) {
    searchVal = searchVal || "";
    prs = prs || [];
    searchVal = angular.lowercase(searchVal);
    var out = [];

    angular.forEach(prs, function(pr) {
      var shouldPush = false;
      var llogin = pr.Login.toLowerCase();
      if (llogin.indexOf(searchVal) === 0) {
        shouldPush = true;
      }
      if (pr.Number.toString().indexOf(searchVal) === 0) {
        shouldPush = true;
      }
      if (shouldPush) {
        out.push(pr);
      }
    });
    return out;
  };
});

app.service('DataService', ['$http', dataService]);

function dataService($http) {
  return ({
    getData: getData,
  });

  function getData(file) {
    return $http.get(file);
  }
}
