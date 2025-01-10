package com.naevatec.ov3.kurentofilter.config;

import javax.annotation.PostConstruct;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.*;
import org.springframework.core.env.Environment;

@Configuration
@PropertySource("classpath:/application.properties")
public class EnvironmentConfig {

	private final static Logger log = LoggerFactory.getLogger(EnvironmentConfig.class);

	/* Config constants */
	public final static String DEFAULT_KURENTO_URL = "ws://localhost:8888/kurento";
	public final static String DEFAULT_KURENTO_PUBLIC_IP = "127.0.0.1";
	public final static String DEFAULT_OV3_URL = "https://xxxx.xxxx";
	public final static String DEFAULT_OV3_SECRET = "changeme";
	public final static String DEFAULT_OV3_API_KEY = "changeme";
	public final static String DEFAULT_API_PASS = "changeme";
	public final static String DEFAULT_WEBHOOK = "";

	public final static String PN_KURENTO_URL = "kurento.url";
	public final static String PN_OV3_URL = "ov3.url";
	public final static String PN_OV3_SECRET = "ov3.secret";
	public final static String PN_OV3_API_KEY = "ov3.apikey";
	public final static String PN_API_PASS = "ov3.api_pass";
	public final static String PN_WEBHOOK = "filter.webhook";


	/* Config variables */
	private String kurentoUrl;
	private String ov3Url;
	private String ov3Secret;
	private String ov3ApiKey;
	private String apiPass;
	private String webhook;

	@Autowired
	private Environment environment;

	@PostConstruct
	public void init() throws Exception{
		readProperties();
		logConfiguration();
	}

	private void logConfiguration() {
		log.info("REST API configuration:");
		log.info("{}: {}", PN_KURENTO_URL, kurentoUrl);
		log.info("{}: {}", PN_OV3_URL, ov3Url);
		log.info("{}: {}", PN_OV3_SECRET, (ov3Secret != null) ? "provided" : "not provided");
		log.info("{}: {}", PN_OV3_API_KEY, (ov3ApiKey != null) ? "provided" : "not provided");
		log.info("{}: {}", PN_WEBHOOK, webhook);
	}

	private void readProperties() throws Exception {
		this.kurentoUrl = environment.getProperty(PN_KURENTO_URL, DEFAULT_KURENTO_URL);
		this.ov3Url = environment.getProperty(PN_OV3_URL, DEFAULT_OV3_URL);
		this.ov3Secret = environment.getProperty(PN_OV3_SECRET, DEFAULT_OV3_SECRET);
		this.ov3ApiKey = environment.getProperty(PN_OV3_API_KEY, DEFAULT_OV3_API_KEY);
		this.apiPass = environment.getProperty(PN_API_PASS, DEFAULT_API_PASS);
		this.webhook = environment.getProperty(PN_WEBHOOK, DEFAULT_WEBHOOK);
	}

	public String getKurentoUrl() {
		return kurentoUrl;
	}

	public void setKurentoUrl(String kurentoUrl) {
		this.kurentoUrl = kurentoUrl;
	}


	public String getOv3Url() {
		return ov3Url;
	}

	public String getOv3Secret() {
		return ov3Secret;
	}

	public String getOv3ApiKey() {
		return ov3ApiKey;
	}

	public void setOv3Url(String lkUrl) {
		this.ov3Url = lkUrl;
	}

	public void setOv3Secret(String lkSecret) {
		this.ov3Secret = lkSecret;
	}

	public void setOv3ApiKey(String lkKey) {
		this.ov3ApiKey = lkKey;
	}

	public Boolean usingLiveKit () {
		return !DEFAULT_OV3_URL.equals(ov3Url);
	}

	public String getApiPass() {
		return apiPass;
	}

	public void setApiPass(String apiPass) {
		this.apiPass = apiPass;
	}

	public String getWebhook() {
		return webhook;
	}

	public void setWebhook(String webhook) {
		this.webhook = webhook;
	}

}
