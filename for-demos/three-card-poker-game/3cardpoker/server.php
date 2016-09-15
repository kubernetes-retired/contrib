<?php

error_reporting(E_ALL);
ini_set('display_errors', 0);

$url = 'http://' . getenv('REDIS_COMPOSITE_APP_SERVICE_HOST') . ':8082/card';
//print($url);
$json = file_get_contents($url, 0, $context);

print($json);
$data = json_decode($json, true);
//print_r($data);

$card1 = $data['card1'];
$card2 = $data['card2'];
$card3 = $data['card3'];
?>
