angular
	.module('ngPods')
	.factory('podsFactory', function($http){

		function getPods(namespace){
			//return $http.get('http://172.16.164.159:8080/api/v1/pods');
			return $http.get('../pods?namespace='+ namespace);
			//return $http.get('../pods');
		}
		return {
			getPods : getPods,
		};
	});



	