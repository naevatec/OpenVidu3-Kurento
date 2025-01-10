package com.naevatec.ov3.kurentofilter.model;

import javax.validation.constraints.NotBlank;

import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiModelProperty;

@ApiModel(description = "Request for adding a filter to OV3 participant")
public class MethodParamsRequest {

	@ApiModelProperty(notes = "Filter method", required = true)
	@NotBlank(message = "filter method must not be empty")
	private String filterMethod;

	@ApiModelProperty(notes = "Method params in JSON structure", required = true)
	@NotBlank(message = "methodParams must not be empty")
	private String methodParams;

    public String getFilterMethod() {
        return filterMethod;
    }

    public void setFilterMethod(String filterMethod) {
        this.filterMethod = filterMethod;
    }

    public String getMethodParams() {
        return methodParams;
    }

    public void setMethodParams(String filterType) {
        this.methodParams = filterType;
    }

    @Override
    public String toString() {
        return "FilterRequest [filterMethod=" + filterMethod + ", methodParams=" + methodParams + "]";
    }
    
}
