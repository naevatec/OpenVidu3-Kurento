package com.naevatec.ov3.kurentofilter.config;

import org.kurento.client.KurentoClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.context.annotation.*;

@Configuration
public class KmsConfig {
	@Autowired
	private EnvironmentConfig environmentConfig;

	// Kms object to ask kms-server for sessionId and token
	@Bean
	public KurentoClient kurentoClient() {
		return KurentoClient.create(this.environmentConfig.getKurentoUrl());
	}
}
