package com.naevatec.ov3.kurentofilter.utils;

import java.io.FileNotFoundException;
import java.io.FileReader;
import java.io.IOException;
import java.io.Reader;
import java.util.Map.Entry;

import org.kurento.jsonrpc.Props;

import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParseException;
import com.google.gson.JsonParser;

public class JsonUtils {

	public Props fromJsonObjectToProps(String paramsStr) 
    {
        Gson gson = new Gson();
        JsonObject params = gson.fromJson(paramsStr, JsonObject.class);

        return fromJsonObjectToProps(params);
    }
    
    public Props fromJsonObjectToProps(JsonObject params) {
		Props props = new Props();
		for (Entry<String, JsonElement> entry : params.entrySet()) {
			if (entry.getValue().isJsonPrimitive()) {
				props.add(entry.getKey(), entry.getValue().getAsString());
			} else if (entry.getValue().isJsonObject()) {
				props.add(entry.getKey(), fromJsonObjectToProps(entry.getValue().getAsJsonObject()));
			}
		}
		return props;
	}

	public JsonObject fromFileToJsonObject(String filePath)
			throws IOException, FileNotFoundException, JsonParseException, IllegalStateException {
		return this.fromFileToJsonElement(filePath).getAsJsonObject();
	}

	public JsonArray fromFileToJsonArray(String filePath)
			throws IOException, FileNotFoundException, JsonParseException, IllegalStateException {
		return this.fromFileToJsonElement(filePath).getAsJsonArray();
	}

	public JsonElement fromFileToJsonElement(String filePath)
			throws IOException, FileNotFoundException, JsonParseException, IllegalStateException {
		return fromReaderToJsonElement(new FileReader(filePath));
	}

	public JsonObject fromReaderToJsonObject(Reader reader) throws IOException {
		return this.fromReaderToJsonElement(reader).getAsJsonObject();
	}

	public JsonElement fromReaderToJsonElement(Reader reader) throws IOException {
		JsonElement json = null;
		try {
			json = JsonParser.parseReader(reader);
		} catch (JsonParseException | IllegalStateException exception) {
			throw exception;
		} finally {
			try {
				reader.close();
			} catch (IOException e) {
				throw e;
			}
		}
		return json;
	}

}