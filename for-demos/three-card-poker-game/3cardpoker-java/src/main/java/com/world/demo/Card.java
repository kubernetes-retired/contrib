package com.world.demo;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;

@JsonIgnoreProperties(ignoreUnknown = true)
public class Card {

    private String card1;
    private String card2;
    private String card3;
    
	public String getCard1() {
		return card1;
	}

	public void setCard1(String card1) {
		this.card1 = card1;
	}

	public String getCard2() {
		return card2;
	}

	public void setCard2(String card2) {
		this.card2 = card2;
	}

	public String getCard3() {
		return card3;
	}

	public void setCard3(String card3) {
		this.card3 = card3;
	}

	public Card(String card1, String card2, String card3) {
		super();
		this.card1 = card1;
		this.card2 = card2;
		this.card3 = card3;
	}
	
	public Card() {
	}

	@Override
	public String toString() {
		return "Card [card1=" + card1 + ", card2=" + card2 + ", card3=" + card3 + "]";
	}

    
}
