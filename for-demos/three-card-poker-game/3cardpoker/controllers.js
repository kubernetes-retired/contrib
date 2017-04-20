var pokerApp = angular.module('poker', ['ui.bootstrap']);

/**
 * Constructor
 */
function PokerController() {}

pokerApp.controller('PokerCtrl', function ($scope, $http, $location) {
        $scope.controller = new PokerController();
        $scope.controller.scope_ = $scope;
        $scope.controller.location_ = $location;
        $scope.controller.http_ = $http;

        

        var data = "./PNGcards/10_of_clubs.png,./PNGcards/9_of_clubs.png,./PNGcards/8_of_clubs.png"
        var cards = data.split(",");

            $scope.card1 = cards[0];
            $scope.card2 = cards[1];
            $scope.card3 = cards[2];


        $scope.controller.http_.get("server.php")
            .success(function(data) {
                console.log(data);
                //var cards = data.split(",");
                $scope.card1 = './PNGcards/'.concat(data.card1).concat('.png');
                $scope.card2 = './PNGcards/'.concat(data.card2).concat('.png');
                $scope.card3 = './PNGcards/'.concat(data.card3).concat('.png');
                
            });


            
        
});
