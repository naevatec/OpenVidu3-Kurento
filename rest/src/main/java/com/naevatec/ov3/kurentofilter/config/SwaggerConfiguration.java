package com.naevatec.ov3.kurentofilter.config;


import java.util.*;

import org.springframework.context.annotation.*;
import org.springframework.web.bind.annotation.RequestMethod;
import springfox.documentation.builders.*;
import springfox.documentation.service.*;
import springfox.documentation.spi.DocumentationType;
import springfox.documentation.spring.web.plugins.Docket;
import springfox.documentation.swagger2.annotations.EnableSwagger2;

@Configuration
@EnableSwagger2
public class SwaggerConfiguration {

	@Bean
	public Docket api() {
		return new Docket(DocumentationType.SWAGGER_2).select().apis(RequestHandlerSelectors
				.basePackage("com.naevatec.ov3.kurentofilter")).paths(
				PathSelectors.any()).build().apiInfo(apiEndPointInfo()).globalResponseMessage(
				RequestMethod.GET, getCustomizedResponseMessages()).globalResponseMessage(
				RequestMethod.POST, getCustomizedResponseMessages()).globalResponseMessage(
				RequestMethod.PUT, getCustomizedResponseMessages()).globalResponseMessage(
				RequestMethod.DELETE, getCustomizedResponseMessages());
	}

	public ApiInfo apiEndPointInfo() {
		return new ApiInfoBuilder().title("Ov3 Filters Rest API").description(
				"Ov3 filters").contact(
				new Contact("NaevaTec", "https://www.naevatec.com/", "info@naevatec.com")).version(
				"1.0.0").build();
	}

	private List<ResponseMessage> getCustomizedResponseMessages() {
		List<ResponseMessage> responseMessages = new ArrayList<>();
		responseMessages.add(new ResponseMessageBuilder().code(500).message("Internal Server Error")
				.build());
		responseMessages.add(new ResponseMessageBuilder().code(404)
				.message("Reason for Bad Request").build());
		return responseMessages;
	}

}
