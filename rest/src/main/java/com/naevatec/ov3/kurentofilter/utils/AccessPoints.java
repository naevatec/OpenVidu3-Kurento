package com.naevatec.ov3.kurentofilter.utils;

public class AccessPoints {

	private static final String V1 = "/v1";
	public static final String V1_OV3 = V1 + "/ov3";


	// session
	public static final String OV3_SESSION = V1_OV3 + "/{ov3RoomId}";

	// participant
	public static final String OV3_PARTICIPANT = OV3_SESSION + "/{participantId}";

	// filter
	public static final String OV3_FILTER = OV3_PARTICIPANT + "/filter";

}
