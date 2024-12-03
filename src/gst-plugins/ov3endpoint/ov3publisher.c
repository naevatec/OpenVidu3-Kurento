/*
 * (C) Copyright 2015 Kurento (http://kurento.org/)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
#ifdef HAVE_CONFIG_H
#include "config.h"
#endif

#include <gst/gst.h>
#include <gst/app/gstappsink.h>
#include <kurento/commons/kmselement.h>
#include <kurento/commons/kmsagnosticcaps.h>
#include <kurento/commons/kmsutils.h>
#include <kurento/commons/kms-core-enumtypes.h>
#include <kurento/commons/kmsfiltertype.h>
#include "ov3publisher.h"
#include "libov3endpoint.h"

#define PLUGIN_NAME "kmsov3publisher"

GST_DEBUG_CATEGORY_STATIC (ov3_publisher_debug_category);
#define GST_CAT_DEFAULT ov3_publisher_debug_category


/* class initialization */

G_DEFINE_TYPE_WITH_CODE (Ov3Publisher, ov3_publisher,
    KMS_TYPE_ELEMENT,
    GST_DEBUG_CATEGORY_INIT (GST_CAT_DEFAULT, PLUGIN_NAME,
        0, "debug category for OpenVidu3 publisher element"));



#define OV3_PUBLISHER_GET_PRIVATE(obj) (  \
  G_TYPE_INSTANCE_GET_PRIVATE (              \
    (obj),                                   \
    KMS_TYPE_OV3_PUBLISHER,                   \
    Ov3PublisherPrivate                    \
  )                                          \
)


#define AUDIO_SINK_BIN_NAME "audio_sink"
#define VIDEO_SINK_BIN_NAME "video_sink"



struct _Ov3PublisherPrivate {
  GstBin *audio_sink;
  GstBin *video_sink;

  gchar *url;
  gchar *secret;
  gchar *key;
  gchar *room;
  gchar *participant_name;
  gchar *participant_id;
  gchar *ingressId;
  gchar *publisherId;
  gboolean screenshare;
  gboolean publishAudio;
  gboolean publishVideo;
  gboolean connected;

  gulong audio_pad_added_conn;
  gulong video_pad_added_conn;
};

/* Properties */
enum
{
  PROP_0,
  PROP_OV3_URL,
  PROP_OV3_SECRET,
  PROP_OV3_KEY,
  PROP_OV3_ROOM,
  PROP_OV3_PARTICIPANT_NAME,
  PROP_OV3_PARTICIPANT_ID,
  PROP_OV3_IS_SCREENSHARE,
  PROP_OV3_PUBLISH_AUDIO,
  PROP_OV3_PUBLISH_VIDEO,
  PROP_OV3_CONNECTED,
};


/* Signals and args */
enum
{
  /* signals */
  SIGNAL_CONNECT,
  SIGNAL_DISCONNECT,

  LAST_SIGNAL
};

static guint obj_signals[LAST_SIGNAL] = { 0 };



static void
audio_sink_pad_added (GstElement * element,
                    GstPad * new_pad,
                    gpointer user_data)
{
  Ov3Publisher *self = (Ov3Publisher*) user_data;

  kms_element_connect_sink_target (KMS_ELEMENT (self), new_pad, KMS_ELEMENT_PAD_TYPE_AUDIO);

  g_signal_handler_disconnect(element, self->priv->audio_pad_added_conn);
  self->priv->audio_pad_added_conn = 0;
}

static void
video_sink_pad_added (GstElement * element,
                    GstPad * new_pad,
                    gpointer user_data)
{
  Ov3Publisher *self = (Ov3Publisher*) user_data;

  kms_element_connect_sink_target (KMS_ELEMENT (self), new_pad, KMS_ELEMENT_PAD_TYPE_VIDEO);

  g_signal_handler_disconnect(element, self->priv->video_pad_added_conn);
  self->priv->video_pad_added_conn = 0;
}




static void
ov3_publisher_connect (Ov3Publisher *self)
{
  gchar *result;
  GstBin *audio_sink = NULL;
  GstBin *video_sink = NULL;




  if (self->priv->publishAudio) {
    GstElement *bin;

    bin = gst_bin_new (AUDIO_SINK_BIN_NAME);
    self->priv->audio_sink = GST_BIN(bin);
    audio_sink = gst_object_ref(self->priv->audio_sink);
    gst_bin_add(GST_BIN(self), GST_ELEMENT(audio_sink));
    self->priv->audio_pad_added_conn = g_signal_connect (G_OBJECT (self->priv->audio_sink),
                                            "pad-added", G_CALLBACK (audio_sink_pad_added), self);
    gst_element_sync_state_with_parent (GST_ELEMENT(self->priv->audio_sink));
  }
  if (self->priv->publishVideo) {
    GstElement *bin;

    bin = gst_bin_new (VIDEO_SINK_BIN_NAME);
    self->priv->video_sink = GST_BIN(bin);
    video_sink = gst_object_ref(self->priv->video_sink);
    gst_bin_add(GST_BIN(self), GST_ELEMENT(video_sink));
    self->priv->video_pad_added_conn = g_signal_connect (G_OBJECT (self->priv->video_sink),
                                            "pad-added", G_CALLBACK (video_sink_pad_added), self);
    gst_element_sync_state_with_parent (GST_ELEMENT(self->priv->video_sink));
  }
  result = connectToRoom (self->priv->url, self->priv->key, self->priv->secret, self->priv->room, self->priv->participant_name, self->priv->participant_id);
  // If result starts with ERROR, then no connection could be made
  if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
    GST_ERROR_OBJECT(self, "Could not connect %s to room %s on service %s for publishing", self->priv->participant_name, self->priv->room, self->priv->url);
    return;
  }

  self->priv->ingressId = result;
  result = publishParticipant (self->priv->screenshare, self->priv->ingressId, audio_sink, video_sink);
  // If result starts with ERROR, no subscription could be made
  if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
    GST_ERROR_OBJECT(self, "Could not publish %s to room %s on service %s", self->priv->participant_name, self->priv->room, self->priv->url);
    return;
  }

  self->priv->publisherId = result;

  self->priv->connected = TRUE;
  GST_INFO_OBJECT(self, "Connected and publishing %s to room %s on service %s for publishing", self->priv->participant_name, self->priv->room, self->priv->url);
}

static void
ov3_publisher_disconnect (Ov3Publisher *self)
{
  gchar *result;

  if (self->priv->publisherId != NULL) {
    result = unpublishParticipant(self->priv->screenshare, self->priv->publisherId);
    // If results begins with ERROR then no unsunscription could be made
    if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
      GST_ERROR_OBJECT(self, "Could not unpublish %s from room %s on service %s", self->priv->participant_name, self->priv->room, self->priv->url);
      return;
    }

    self->priv->publisherId = NULL;
    self->priv->connected = FALSE;

    result = disconnectFromRoom(self->priv->ingressId);
    // If results begins with ERROR then no unsunscription could be made
    if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
      GST_INFO_OBJECT(self, "Not disconnecting %s from room %s on service %s", self->priv->participant_name, self->priv->room, self->priv->url);
      return;
    }
    GST_INFO_OBJECT(self, "Disconnected publish %s from room %s on service %s", self->priv->participant_name, self->priv->room, self->priv->url);

  }

}


static void
ov3_publisher_finalize (GObject *object)
{
  Ov3Publisher *self = KMS_OV3_PUBLISHER(object);

  if (self->priv->audio_sink != NULL) {
    gst_object_unref (self->priv->audio_sink);
  }
  if (self->priv->video_sink != NULL) {
    gst_object_unref (self->priv->video_sink);
  }
  if (self->priv->url != NULL) {
    g_free(self->priv->url);
  }
  if (self->priv->secret != NULL) {
    g_free(self->priv->secret);
  }
  if (self->priv->key != NULL) {
    g_free(self->priv->key);
  }
  if (self->priv->room != NULL) {
    g_free(self->priv->room);
  }
  if (self->priv->participant_name != NULL) {
    g_free(self->priv->participant_name);
  }
  if (self->priv->participant_id != NULL) {
    g_free(self->priv->participant_id);
  }
  if (self->priv->ingressId != NULL) {
    g_free(self->priv->ingressId);
  }
  if (self->priv->publisherId != NULL) {
    g_free(self->priv->publisherId);
  }
}


static void 
ov3_publisher_set_property (GObject * object, guint property_id,
    const GValue * value, GParamSpec * pspec)
{
  Ov3Publisher *self = KMS_OV3_PUBLISHER (object);

  switch (property_id) {
    case PROP_OV3_URL:{
      g_free (self->priv->url);
      self->priv->url = g_value_dup_string (value);
      break;
    }
    case PROP_OV3_KEY:{
      g_free (self->priv->key);
      self->priv->key = g_value_dup_string (value);
      break;
    }
    case PROP_OV3_SECRET:{
      g_free (self->priv->secret);
      self->priv->secret = g_value_dup_string (value);
      break;
    }
    case PROP_OV3_ROOM:{
      g_free (self->priv->room);
      self->priv->room = g_value_dup_string (value);
      break;
    }
    case PROP_OV3_PARTICIPANT_NAME:{
      g_free (self->priv->participant_name);
      self->priv->participant_name = g_value_dup_string (value);
      break;
    }
    case PROP_OV3_PARTICIPANT_ID:{
      g_free (self->priv->participant_id);
      self->priv->participant_id = g_value_dup_string (value);
      break;
    }
    case PROP_OV3_IS_SCREENSHARE:{
      self->priv->screenshare = g_value_get_boolean (value);
      break;
    }
    case PROP_OV3_PUBLISH_AUDIO:{
      self->priv->publishAudio = g_value_get_boolean (value);
      break;
    }
    case PROP_OV3_PUBLISH_VIDEO:{
      self->priv->publishVideo = g_value_get_boolean (value);
      break;
    }
    default:
      G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);
      break;
  }
}

static void
ov3_publisher_get_property (GObject * object, guint property_id,
    GValue * value, GParamSpec * pspec)
{
  Ov3Publisher *self = KMS_OV3_PUBLISHER (object);

  switch (property_id) {
    case PROP_OV3_URL: {
      g_value_set_string (value, self->priv->url);
      break;
    }
    case PROP_OV3_KEY: {
      g_value_set_string (value, self->priv->key);
      break;
    }
    case PROP_OV3_SECRET: {
      g_value_set_string (value, self->priv->secret);
      break;
    }
    case PROP_OV3_ROOM: {
      g_value_set_string (value, self->priv->room);
      break;
    }
    case PROP_OV3_PARTICIPANT_NAME: {
      g_value_set_string (value, self->priv->participant_name);
      break;
    }
    case PROP_OV3_PARTICIPANT_ID: {
      g_value_set_string (value, self->priv->participant_id);
      break;
    }
    case PROP_OV3_IS_SCREENSHARE: {
      g_value_set_boolean (value, self->priv->screenshare);
      break;
    }
    case PROP_OV3_PUBLISH_AUDIO:{
      g_value_set_boolean (value, self->priv->publishAudio);
      break;
    }
    case PROP_OV3_PUBLISH_VIDEO:{
      g_value_set_boolean (value, self->priv->publishVideo);
      break;
    }
    case PROP_OV3_CONNECTED: {
      g_value_set_boolean (value, self->priv->connected);
      break;
    }
    default:
      G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);
      break;

  }
}

static void
ov3_publisher_class_init (Ov3PublisherClass * klass)
{
  GObjectClass *gobject_class;

  gobject_class = G_OBJECT_CLASS (klass);
  gobject_class->set_property = ov3_publisher_set_property;
  gobject_class->get_property = ov3_publisher_get_property;
  gobject_class->finalize = ov3_publisher_finalize;

  klass->ov3_connect = ov3_publisher_connect ;
  klass->ov3_disconnect = ov3_publisher_disconnect;

  gst_element_class_set_static_metadata (GST_ELEMENT_CLASS (klass),
      "Ov3Publisher", "Generic/KmsElement", "Kurento OpenVidu3 WebRtc publisher",
      "Saúl Pablo Labajo Izquierdo <slabajo@naevatec.com>");

  g_object_class_install_property (gobject_class, PROP_OV3_URL,
      g_param_spec_string ("ov3-url",
          "OpenVidu3 URL", "URL to access OpenVidu3 service",
          "",
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_SECRET,
      g_param_spec_string ("ov3-secret",
          "OpenVidu3 secret", "Secret to access OpenVidu3 service",
          "",
		  G_PARAM_WRITABLE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_KEY,
      g_param_spec_string ("ov3-key",
          "OpenVidu3 Key", "Key to access OpenVidu3 service",
          "",
		  G_PARAM_WRITABLE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_ROOM,
      g_param_spec_string ("ov3-room",
          "OpenVidu3 room", "Room to where this endpoint will connect",
          "",
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_PARTICIPANT_NAME,
      g_param_spec_string ("ov3-participant-name",
          "OpenVidu3 participant", "Participant Name in the OpenVidu3 room whose tracks will be subscribed by this endpoint",
          "",
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_PARTICIPANT_ID,
      g_param_spec_string ("ov3-participant-id",
          "OpenVidu3 participant", "Participant Id in the OpenVidu3 room whose tracks will be subscribed by this endpoint",
          "",
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_IS_SCREENSHARE,
      g_param_spec_boolean ("ov3-screenshare",
          "OpenVidu3 ScreenShare", "This endpoint must subscribe to screen share tracks",
          FALSE,
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_PUBLISH_AUDIO,
      g_param_spec_boolean ("ov3-publishAudio",
          "OpenVidu3 PublishAudio", "TRUE if this endpoint must publish an audio track",
          FALSE,
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_PUBLISH_VIDEO,
      g_param_spec_boolean ("ov3-publishVideo",
          "OpenVidu3 ScreenShare", "TRUE if this endpoint must publish a Video track",
          FALSE,
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_CONNECTED,
      g_param_spec_boolean ("ov3-connected",
          "OpenVidu3 Subscriber connected", "True if the endpoint is currently connected and subscribing tracks",
          FALSE,
		  G_PARAM_READABLE | G_PARAM_STATIC_STRINGS));      


  obj_signals[SIGNAL_CONNECT] =
      g_signal_new ("ov3-connect",
      G_TYPE_FROM_CLASS (klass),
      G_SIGNAL_ACTION | G_SIGNAL_RUN_LAST,
      G_STRUCT_OFFSET (Ov3PublisherClass, ov3_connect), NULL, NULL,
      NULL, G_TYPE_NONE, 0, G_TYPE_NONE);
  obj_signals[SIGNAL_DISCONNECT] =
      g_signal_new ("ov3-disconnect",
      G_TYPE_FROM_CLASS (klass),
      G_SIGNAL_ACTION | G_SIGNAL_RUN_LAST,
      G_STRUCT_OFFSET (Ov3PublisherClass, ov3_disconnect), NULL, NULL,
      NULL, G_TYPE_NONE, 0, G_TYPE_NONE);

  g_type_class_add_private (klass, sizeof (Ov3PublisherPrivate));


}


static void
ov3_publisher_init (Ov3Publisher * self)
{
  self->priv = OV3_PUBLISHER_GET_PRIVATE (self);

  self->priv->url = g_strdup ("");
  self->priv->secret = g_strdup ("");
  self->priv->key = g_strdup ("");
  self->priv->room = g_strdup ("");
  self->priv->participant_name = g_strdup ("");
  self->priv->participant_id = g_strdup ("");
  self->priv->screenshare = FALSE;
  self->priv->ingressId = NULL;
  self->priv->connected = FALSE;
  self->priv->audio_pad_added_conn = 0;
  self->priv->video_pad_added_conn = 0;
  self->priv->audio_sink = NULL;
  self->priv->video_sink = NULL;
  self->priv->audio_pad_added_conn = 0;
  self->priv->video_pad_added_conn = 0;
}

gboolean
kms_ov3_publisher_plugin_init (GstPlugin * plugin)
{

  return gst_element_register (plugin, PLUGIN_NAME, GST_RANK_NONE,
      KMS_TYPE_OV3_PUBLISHER);
}

