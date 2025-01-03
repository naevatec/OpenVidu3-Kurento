package com.naevatec.ov3.kurentofilter.controllers;

import com.google.gson.JsonElement;
import com.naevatec.ov3.kurentofilter.config.EnvironmentConfig;
import com.naevatec.ov3.kurentofilter.config.KmsConfig;
import com.naevatec.ov3.kurentofilter.model.FilterRequest;
import com.naevatec.ov3.kurentofilter.model.MethodParamsRequest;
import com.naevatec.ov3.kurentofilter.utils.AccessPoints;
import com.naevatec.ov3.kurentofilter.utils.JsonUtils;

import io.swagger.annotations.*;

import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.*;

import javax.validation.Valid;

import org.kurento.client.GenericMediaElement;
import org.kurento.client.GenericMediaEvent;
import org.kurento.client.GstreamerDotDetails;
import org.kurento.client.ListenerSubscription;
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

	private Map<String,FilterOptions> participantsPipes = new HashMap<>();

	private static class FilterOptions 
	{
		private String sessionId;
		private String participant;
		private MediaPipeline pipeline;
		private GenericMediaElement filter;
		private Map<String, ListenerSubscription> listeners = new HashMap<>();

		
		public String getSessionId() {
			return sessionId;
		}
		public void setSessionId(String sessionId) {
			this.sessionId = sessionId;
		}
		public String getParticipant() {
			return participant;
		}
		public void setParticipant(String participant) {
			this.participant = participant;
		}
		public MediaPipeline getPipeline() {
			return pipeline;
		}
		public void setPipeline(MediaPipeline pipeline) {
			this.pipeline = pipeline;
		}
		public GenericMediaElement getFilter() {
			return filter;
		}
		public void setFilter(GenericMediaElement filter) {
			this.filter = filter;
		}

		public void addListener (ListenerSubscription listener, String type)
		{
			listeners.put(type, listener);
		}

		public void removeListener (String type)
		{
			listeners.remove(type);
		}

		public ListenerSubscription getListener (String type)
		{
			return listeners.get(type);
		}
	}

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

	private FilterOptions createFilter (MediaPipeline pipeline,
							   String sessionId,
							   String participantId,
							   String filterType,
							   String filterStr) throws Exception
	{
		OV3Publisher publisher;
		OV3Subscriber subscriber;
		GenericMediaElement filter;
		FilterOptions result = new FilterOptions();

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

		result.setPipeline(pipeline);
		result.setFilter(filter);

		if (!publisher.publishParticipant()) {
			logger.warn("Could not publish participant {} in room {}", participantId+"_filtered", sessionId);
		}
		if (!subscriber.subscribeParticipant(sessionId, participantId, false)) {
			logger.warn("Could not subscribe participant {} in room {}", participantId, sessionId);
		}

		return result;
	}

	private JsonElement execFilterMethodInPublisher(GenericMediaElement filter, String method, String params) {
		Props props = new JsonUtils().fromJsonObjectToProps(params);
		return (JsonElement) filter.invoke(method, props, JsonElement.class);
	}	

	private void dispatchEvent (FilterOptions options, String eventType, GenericMediaEvent event)
	{
		try {
			URL webhook = new URL (env.getWebhook()+"/"+options.getSessionId()+"/"+options.getParticipant()+"/"+eventType);

			// Create Http connection
			HttpURLConnection connection = (HttpURLConnection) webhook.openConnection(); 
			
			connection.setRequestMethod("POST"); 
			connection.setRequestProperty("Content-Type", "application/json; utf-8"); 
			connection.setRequestProperty("Accept", "application/json"); 
			connection.setDoOutput(true); 
			
			// JSON to send on the request 
			String eventJson = new JsonUtils().fromPropsToJsonObjectString (event.getData()) ;
			
			
			try (OutputStream os = connection.getOutputStream()) { 
				byte[] input = eventJson.getBytes("utf-8"); 
				
				os.write(input, 0, input.length); 
			} 
			// Leer la respuesta 
			int responseCode = connection.getResponseCode(); 
			if ((responseCode < 200) || (responseCode >= 300)) {
				logger.warn("Could not send event webhook {}", responseCode);
			}
		} catch (Exception xcp) {
			logger.warn("Could not send event webhook {}", xcp.toString());
		}
	}

	private void addFilterEventListener (FilterOptions options, String eventType) throws Exception
	{
		ListenerSubscription listener = options.getFilter ().addEventListener(eventType, event -> {
					dispatchEvent (options, eventType, event);
				}, GenericMediaEvent.class);
				options.addListener(listener, eventType);
	}

	private void removeEventLIstener (FilterOptions options, String eventType)
	{
		GenericMediaElement filter = options.getFilter();
		ListenerSubscription listener = options.getListener(eventType);

		filter.removeEventListener(listener);
		options.removeListener(eventType);
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
		MediaPipeline pipe;
		FilterOptions filterResult = participantsPipes.get (key);;
		

		if (filterResult != null) {
			logger.error("RestApiController.addFilter, filter already setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter already setup", HttpStatus.BAD_REQUEST);
		}

		try {
			pipe = kmsConfig.kurentoClient().createMediaPipeline();

			filterResult = createFilter (pipe, sessionId, participantId, filterRequest.getFilterType(), filterRequest.getFilterCommand());
			filterResult.setSessionId(sessionId);
			filterResult.setParticipant(participantId);
			participantsPipes.put(key, filterResult);
			return new ResponseEntity<>(HttpStatus.OK);

		} catch (Exception xcp) {
			return new ResponseEntity<>(xcp.toString(), HttpStatus.INTERNAL_SERVER_ERROR);
		}
	}

	@ApiOperation(value = "Executes a Kurento method in a filter to a participant stream in an OpenVIdu 3 session")
	@PostMapping(AccessPoints.OV3_METHOD)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Filter correctly setup"),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVIdu 3 Session or participant not found", response=Void.class)
	})
	public ResponseEntity<String> executeFilterMethod(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId,
		@ApiParam(value = "Parameters to apply to the method execution", required = true) @Valid
			@RequestBody MethodParamsRequest methodParamsRequest) throws Exception {
		logger.info("RestApiController.executeFilterMethod to {} in {}, method ", participantId, sessionId, methodParamsRequest.getFilterMethod());

		String key = sessionId+participantId;
		FilterOptions filterResult = participantsPipes.get (key);;
		

		if (filterResult == null) {
			logger.error("RestApiController.executeFilterMethod, filter not setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter not setup", HttpStatus.NOT_FOUND);
		}

		try {
			JsonElement result = execFilterMethodInPublisher (filterResult.getFilter(), methodParamsRequest.getFilterMethod(), methodParamsRequest.getMethodParams());
			return new ResponseEntity<>(result.getAsString(), HttpStatus.OK);

		} catch (Exception xcp) {
			return new ResponseEntity<>(xcp.toString(), HttpStatus.INTERNAL_SERVER_ERROR);
		}
	}

	@ApiOperation(value = "Subscribes a Kurento event in a filter to a participant stream in an OpenVIdu 3 session")
	@GetMapping(AccessPoints.OV3_EVENT)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Correct subscription"),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVIdu 3 Session or participant not found", response=Void.class)
	})
	public ResponseEntity<String> subscribeFilterEvent(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId,
		@PathVariable("event") String event) throws Exception {
		logger.info("RestApiController.subscribeFilterEvent to {} in {}, event ", participantId, sessionId, event);

		String key = sessionId+participantId;
		FilterOptions filterResult = participantsPipes.get (key);;
		

		if (filterResult == null) {
			logger.error("RestApiController.subscribeFilterEvent, filter not setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter not setup", HttpStatus.NOT_FOUND);
		}

		try {
			addFilterEventListener (filterResult, event);
			return new ResponseEntity<>(HttpStatus.OK);

		} catch (Exception xcp) {
			return new ResponseEntity<>(xcp.toString(), HttpStatus.INTERNAL_SERVER_ERROR);
		}
	}

	@ApiOperation(value = "Subscribes a Kurento event in a filter to a participant stream in an OpenVIdu 3 session")
	@DeleteMapping(AccessPoints.OV3_EVENT)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Correct unsubscription"),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVIdu 3 Session or participant not found", response=Void.class)
	})
	public ResponseEntity<String> unsubscribeFilterEvent(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId,
		@PathVariable("event") String event) throws Exception {
		logger.info("RestApiController.unsubscribeFilterEvent to {} in {}, event ", participantId, sessionId, event);

		String key = sessionId+participantId;
		FilterOptions filterResult = participantsPipes.get (key);;
		

		if (filterResult == null) {
			logger.error("RestApiController.unsubscribeFilterEvent, filter not setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter not setup", HttpStatus.NOT_FOUND);
		}

		try {
			removeEventLIstener (filterResult, event);
			return new ResponseEntity<>(HttpStatus.OK);

		} catch (Exception xcp) {
			return new ResponseEntity<>(xcp.toString(), HttpStatus.INTERNAL_SERVER_ERROR);
		}
	}

	@ApiOperation(value = "Removes a filter to a participant stream in an OpenVIdu 3 session")
	@DeleteMapping(AccessPoints.OV3_PARTICIPANT)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Filter correctly setup"),
		@ApiResponse(code = 406, message = "Invalid parameters", response=String.class),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVidu 3 Session or participant or filter not found", response=Void.class)
	})
	public ResponseEntity<String> removeFilter(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId) throws Exception {
		logger.info("RestApiController.addFilter to {} in {} ", participantId, sessionId);

		String key = sessionId+participantId;
		FilterOptions options = participantsPipes.get (key);

		if (options == null) {
			logger.error("RestApiController.addFilter, filter not setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter not setup", HttpStatus.NOT_FOUND);
		}

		participantsPipes.remove(key);
		options.getPipeline().release();

		return new ResponseEntity<String> ("Filter correctly released", HttpStatus.OK);
	}

	@ApiOperation(value = "Generates a gstreamer dot dump for a filter pipeline that has been setup")
	@GetMapping(AccessPoints.OV3_PARTICIPANT_DOT)
	@ApiResponses({
		@ApiResponse(code = 200, message = "Filter correctly setup"),
		@ApiResponse(code = 406, message = "Invalid parameters", response=String.class),
		@ApiResponse(code = 500, message = "Internal server error", response=String.class),
		@ApiResponse(code = 404, message = "OpenVidu 3 Session or participant or filter not found", response=Void.class)
	})
	public ResponseEntity<String> getFilterDotDump(
		@PathVariable("ov3RoomId") String sessionId,
		@PathVariable("participantId") String participantId) throws Exception {
		logger.info("RestApiController.addFilter to {} in {} ", participantId, sessionId);

		String key = sessionId+participantId;
		FilterOptions options = participantsPipes.get (key);
		String dot;

		if (options == null) {
			logger.error("RestApiController.addFilter, filter not setup for {} in {}", participantId, sessionId);
			return new ResponseEntity<String>("Filter not setup", HttpStatus.NOT_FOUND);
		}

		dot = options.getPipeline().getGstreamerDot(GstreamerDotDetails.SHOW_ALL);

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
