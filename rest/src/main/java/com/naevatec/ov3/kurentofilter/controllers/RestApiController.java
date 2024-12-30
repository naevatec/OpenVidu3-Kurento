package com.naevatec.ov3.kurentofilter.controllers;

import com.naevatec.ov3.kurentofilter.config.EnvironmentConfig;
import com.naevatec.ov3.kurentofilter.config.KmsConfig;
import com.naevatec.ov3.kurentofilter.model.FilterRequest;
import com.naevatec.ov3.kurentofilter.utils.AccessPoints;
import com.naevatec.ov3.kurentofilter.utils.JsonUtils;

import io.swagger.annotations.*;
import java.util.*;

import javax.validation.Valid;

import org.kurento.client.GenericMediaElement;
import org.kurento.client.GstreamerDotDetails;
import org.kurento.client.MediaPipeline;
import org.kurento.jsonrpc.Props;
import org.kurento.module.ov3endpoint.OV3Publisher;
import org.kurento.module.ov3endpoint.OV3Subscriber;
import org.slf4j.*;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.*;
import org.springframework.validation.ObjectError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.*;


@RestController
@RequestMapping(AccessPoints.V1_OV3)
@Api(value = "Ov3FilterRestControllerAPI", produces = 
MediaType.APPLICATION_JSON_VALUE)
public class RestApiController {

	private final static Logger logger = LoggerFactory.getLogger(RestApiController.class);

	private Map<String,MediaPipeline> participantsPipes = new HashMap<>();

	@Autowired
	KmsConfig kmsConfig;

	@Autowired
	EnvironmentConfig env;

	protected GenericMediaElement applyFilterInPublisher(MediaPipeline pipeline, String filterType, String filterOptions) {
		GenericMediaElement.Builder builder = new GenericMediaElement.Builder(pipeline, filterType);
		Props props = new JsonUtils().fromJsonObjectToProps(filterOptions);
		props.forEach(prop -> {
			builder.withConstructorParam(prop.getName(), prop.getValue());
		});
		return builder.build();
	}	

	private void createFilter (MediaPipeline pipeline,
							   String sessionId,
							   String participantId,
							   String filterType,
							   String filterStr)
	{
		OV3Publisher publisher;
		OV3Subscriber subscriber;
		GenericMediaElement filter;

		publisher = new OV3Publisher.Builder(pipeline, 
											 env.getOv3Url(),
											 env.getOv3Secret(),
											 env.getOv3ApiKey(),
											 sessionId,
											 participantId+"_filtered").withParticipantId(participantId+"_filtered").build();

		subscriber = new OV3Subscriber.Builder(pipeline,
		                                       env.getOv3Url(),
											   env.getOv3Secret(),
											   env.getOv3ApiKey()).build();


		filter = applyFilterInPublisher (pipeline, filterType, filterStr);

		subscriber.connect(filter);
		filter.connect(publisher);

		if (!publisher.publishParticipant()) {
			logger.warn("Could not publish participant {} in room {}", participantId+"_filtered", sessionId);
		}
		if (!subscriber.subscribeParticipant(sessionId, participantId, false)) {
			logger.warn("Could not subscribe participant {} in room {}", participantId, sessionId);
		}
	}

	@ApiOperation(value = "Add a filter to a participant stream in an OpenVIdu 3 session")
	@PostMapping(AccessPoints.OV3_PARTICIPANT)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Filter correctly setup"),
		@ApiResponse(code = 400, message = "Filter already in place"),
		@ApiResponse(code = 406, message = "Invalid parameters", response=String.class),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVIdu 3 Session or participant not found", response=Void.class)
	})
	public ResponseEntity<String> addFilter(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId,
		@ApiParam(value = "Filter options to add to the partiicpant stream", required = true) @Valid
			@RequestBody FilterRequest filterRequest) throws Exception {
		logger.info("RestApiController.addFilter to {} in {} ", participantId, sessionId);

		String key = sessionId+participantId;
		MediaPipeline pipe = participantsPipes.get (key);
		

		if (pipe != null) {
			logger.error("RestApiController.addFilter, filter already setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter already setup", HttpStatus.INTERNAL_SERVER_ERROR);
		}

		pipe = kmsConfig.kurentoClient().createMediaPipeline();
		participantsPipes.put(key, pipe);

		createFilter (pipe, sessionId, participantId, filterRequest.getFilterType(), filterRequest.getFilterCommand());

		return new ResponseEntity<>(HttpStatus.OK);
	}

	@ApiOperation(value = "Removes a filter to a participant stream in an OpenVIdu 3 session")
	@DeleteMapping(AccessPoints.OV3_PARTICIPANT)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Filter correctly setup"),
		@ApiResponse(code = 400, message = "Filter already in place"),
		@ApiResponse(code = 406, message = "Invalid parameters", response=String.class),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVidu 3 Session or participant or filter not found", response=Void.class)
	})
	public ResponseEntity<String> removeFilter(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId) throws Exception {
		logger.info("RestApiController.addFilter to {} in {} ", participantId, sessionId);

		String key = sessionId+participantId;
		MediaPipeline pipe = participantsPipes.get (key);

		if (pipe == null) {
			logger.error("RestApiController.addFilter, filter not setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter not setup", HttpStatus.NOT_FOUND);
		}

		pipe.release();

		return new ResponseEntity<String> ("Filter correctly released", HttpStatus.OK);
	}

	@ApiOperation(value = "Removes a filter to a participant stream in an OpenVIdu 3 session")
	@GetMapping(AccessPoints.OV3_PARTICIPANT_DOT)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Filter correctly setup"),
		@ApiResponse(code = 400, message = "Filter already in place"),
		@ApiResponse(code = 406, message = "Invalid parameters", response=String.class),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVidu 3 Session or participant or filter not found", response=Void.class)
	})
	public ResponseEntity<String> getFilterDotDump(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId) throws Exception {
		logger.info("RestApiController.addFilter to {} in {} ", participantId, sessionId);

		String key = sessionId+participantId;
		MediaPipeline pipe = participantsPipes.get (key);
		String dot;

		if (pipe == null) {
			logger.error("RestApiController.addFilter, filter not setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter not setup", HttpStatus.NOT_FOUND);
		}

		dot = pipe.getGstreamerDot(GstreamerDotDetails.SHOW_ALL);

		return new ResponseEntity<String> (dot, HttpStatus.OK);
	}


	// ---------------------------------------------------------------------------------------------

/*	@ExceptionHandler(SessionNotFoundException.class)
	ResponseEntity<String> handleSessionNotFoundException() {
		logger.error("ERROR handleSessionNotFoundException: LiveKit Session Not Found");
		return new ResponseEntity<>("LiveKit Session Not Found", HttpStatus.NOT_FOUND);
	}

	@ExceptionHandler(EndpointNotFoundException.class)
	ResponseEntity<String> handleEndpointNotFoundException() {
		logger.error("ERROR handleEndpointNotFoundException: LiveKit Sip Endpoint Not Found");
		return new ResponseEntity<>("LiveKit Sip Endpoint Not Found", HttpStatus.NOT_FOUND);
	}
*/
	@ExceptionHandler(MethodArgumentNotValidException.class)
	ResponseEntity<String> handleBadRequestException(MethodArgumentNotValidException ex) {
		List<String> detailsList = new ArrayList<>();
		for (ObjectError error : ex.getBindingResult().getAllErrors()) {
			detailsList.add(error.getDefaultMessage());
		}
		logger.error("ERROR handleBadRequestException: " + String.join(", ", detailsList));
		return new ResponseEntity<>(String.join(", ", detailsList), HttpStatus.NOT_ACCEPTABLE);
	}

	@ExceptionHandler(Exception.class)
	ResponseEntity<String> handleException(Exception e) {
		logger.error("ERROR handleException", e);
		return new ResponseEntity<>("Internal Server Error: "+ e.toString(), HttpStatus.INTERNAL_SERVER_ERROR);
	}

}
