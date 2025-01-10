package com.naevatec.ov3.kurentofilter.model;

import javax.validation.constraints.NotBlank;

import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiModelProperty;

@ApiModel(description = "Request for adding a filter to OV3 participant")
public class FilterRequest {

	@ApiModelProperty(notes = "Filter command", required = true)
	@NotBlank(message = "filterCommand must not be empty")
	private String filterCommand;

	@ApiModelProperty(notes = "Filter type", required = true)
	@NotBlank(message = "filterType must not be empty")
	private String filterType;

    public String getFilterCommand() {
        return filterCommand;
    }

    public void setFilterCommand(String filterCommand) {
        this.filterCommand = filterCommand;
    }

    public String getFilterType() {
        return filterType;
    }

    public void setFilterType(String filterType) {
        this.filterType = filterType;
    }

    @Override
    public String toString() {
        return "FilterRequest [filterCommand=" + filterCommand + ", filterType=" + filterType + "]";
    }
    
}
