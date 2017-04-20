angular
	.module('ngPods')
	.filter('podFilter', function () {
		return function(input,runString) {
			var filtered = [];
			var pods = input;
			var run = runString;
			var items = {};
			items = angular.fromJson(pods.items);
			
			angular.forEach(items, function(pod){
				if(pod.metadata.hasOwnProperty('labels')){
					if (pod.metadata.labels.run == run) {
					//console.log("Status = " + JSON.stringify(pod.status.phase));	
					filtered.push(pod);
					//console.log("Filtered: "+ pod);
					}
				}
			});

			return filtered;
		};
	});