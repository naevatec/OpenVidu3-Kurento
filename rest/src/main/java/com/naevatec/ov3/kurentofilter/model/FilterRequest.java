package com.naevatec.ov3.kurentofilter.model;

import javax.validation.constraints.NotBlank;

import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiModelProperty;

@ApiModel(description = "Request for adding a filter to OV3 participant")
public class FilterRequest {

	@ApiModelProperty(notes = "Filter options", required = true)
	@NotBlank(message = "filterOptions must not be empty")
	private String filterOptions;

	@ApiModelProperty(notes = "Filter type", required = true)
	@NotBlank(message = "filterType must not be empty")
	private String filterType;

    public String getFilterOptions() {
        return filterOptions;
    }

    public void setFilterOptions(String filterCommand) {
        this.filterOptions = filterCommand;
    }

    public String getFilterType() {
        return filterType;
    }

    public void setFilterType(String filterType) {
        this.filterType = filterType;
    }

    @Override
    public String toString() {
        return "FilterRequest [filterCommand=" + filterOptions + ", filterType=" + filterType + "]";
    }
    
}
