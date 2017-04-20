angular
	.module('ngPods')
	.controller('podsController', function($scope, podsFactory, $timeout, $location, $filter) {
		
		
		$scope.pods ={};
		$scope.namespace = "";

		$scope.reload = function () {

			var ns = $location.path();
			ns = ns.substring(1, ns.length)
			$scope.namespace = ns;
		    podsFactory.getPods(ns).then(

				function successCallback(data) {
				    $scope.pods = data.data;
				    $scope.pods.items =  data.data.items;		
					$scope.presentationPods = $filter('podFilter')(data.data,'three-card-poker');
					$scope.compositePods = $filter('podFilter')(data.data,'redis-composite-app');
					$scope.atomicAPods =$filter('podFilter')(data.data,'redis-a-app');
					$scope.atomicBPods =$filter('podFilter')(data.data,'redis-b-app');
					$scope.dbPods=$filter('podFilter')(data.data,'redis');			
				},

			  	function errorCallback(error) {
			      	console.log(error);
				}
			);

		    $timeout(function(){
		      $scope.reload();
		    },1000)
		  };

		  $scope.reload();

		$scope.newListing = {};
	});