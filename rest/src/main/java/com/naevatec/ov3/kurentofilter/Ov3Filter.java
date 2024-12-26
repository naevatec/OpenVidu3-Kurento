package com.naevatec.ov3.kurentofilter;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.scheduling.annotation.*;

@SpringBootApplication
@EnableScheduling
@EnableAsync
public class Ov3Filter {

	public static void main(String[] args) {
		SpringApplication.run(Ov3Filter.class, args);
	}
	
}
