package com.world.demo;

import java.util.ArrayList;
import java.util.List;

import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.client.RestTemplate;

@RestController
public class ServiceController {

    @RequestMapping("/health")
    public String hello() {
        return "Composite Service app running";
    }
    
    @RequestMapping("/card-from-composite")
    public Card pods() {
    	//REDIS_COMPOSITE_APP_SERVICE_HOST
    	String redisCompositeService = System.getenv("REDIS_COMPOSITE_SERVICE");
    	if(redisCompositeService== null)
    	{
    		System.out.println("Please provide REDIS_COMPOSITE_SERVICE env variable");
    		System.exit(1);
    	}
    	String redisComposite = System.getenv(redisCompositeService);   	
    	if(redisComposite== null)
    	{
    		System.out.println("env variable for redisComposite is not available");
    		System.exit(1);
    	}
    	
    	//"http://172.16.164.159:8080/api/v1/proxy/namespaces/default/services/redis-composite-app/card";
    	String url = "http://" + redisComposite + ":8082/card";
    	System.out.println("Calling composite service-->"+ url);
    	RestTemplate restTemplate = new RestTemplate();
    	Card card = restTemplate.getForObject(url,Card.class);
    	return card;
    	
    }
    
    @RequestMapping("/card")
    public Card card() {
    	
    	Card card = new Card();
    	List<String> draws = new ArrayList<>();
    	while(draws.size() <3)
    	{
    		String draw = getACard();
    		if(!draws.contains(draw)){
    			draws.add(draw);
    		}
    	}
    	card.setCard1((draws.get(0)));
    	card.setCard2((draws.get(1)));
    	card.setCard3((draws.get(2)));
        return card;
    }
    
    private String getACard(){
    	
    	RestTemplate restTemplate = new RestTemplate();
    	
    	String redisAAPPService = System.getenv("REDIS_A_APP_SERVICE");
    	if(redisAAPPService== null)
    	{
    		System.out.println("Please provide REDIS_A_APP_SERVICE env variable");
    		System.exit(1);
    	}
    		
    	String redisA = System.getenv(redisAAPPService);
    	
    	String redisBAPPService = System.getenv("REDIS_B_APP_SERVICE");
    	if(redisAAPPService== null)
    	{
    		System.out.println("Please provide REDIS_B_APP_SERVICE env variable");
    		System.exit(1);
    	}
    	String redisB = System.getenv(redisBAPPService);
    	
    	if(redisB== null || redisA==null)
    	{
    		System.out.println("Please provide redisA or redisB env variable not available");
    		System.exit(1);
    	}
    	
    	
    	String url1 = "http://" + redisA + ":8080/randomNumber";
    	String url2 = "http://" + redisB + ":8081/randomSuit";
    	
    	//String url1 = "http://172.16.164.159:8080/api/v1/proxy/namespaces/default/services/redis-a-app/randomNumber";
    	//String url2 = "http://172.16.164.159:8080/api/v1/proxy/namespaces/default/services/redis-b-app/randomSuit";
    				
    	Random ranNumber = restTemplate.getForObject(url1,Random.class);
    	Random ranSuit = restTemplate.getForObject(url2,Random.class);
    	String retVal = ranNumber.getId() + "_of_" + ranSuit.getId();
    	System.out.println(retVal);
    	return retVal;
    }
}
