package com.naevatec.ov3.kurentofilter.utils;

public class AccessPoints {

	private static final String V1 = "/v1";
	public static final String V1_OV3 = V1 + "/ov3";


	// session
	public static final String OV3_SESSION = "/{ov3RoomId}";

	// participant
	public static final String OV3_PARTICIPANT = OV3_SESSION + "/{participantId}";

	// filter
	public static final String OV3_FILTER = OV3_PARTICIPANT + "/filter";

	// filter
	public static final String OV3_PARTICIPANT_DOT = OV3_PARTICIPANT + "/dot";

	// method
	public static final String OV3_METHOD = OV3_PARTICIPANT + "/exec";

	// event
	public static final String OV3_EVENT = OV3_PARTICIPANT + "/event/{event}";

}
