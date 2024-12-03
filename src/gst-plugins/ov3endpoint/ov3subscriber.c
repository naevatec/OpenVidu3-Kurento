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
#include <gst/app/gstappsrc.h>
#include <kurento/commons/kmsagnosticcaps.h>
#include <kurento/commons/kmsutils.h>
#include <kurento/commons/kms-core-enumtypes.h>
#include <kurento/commons/kmsfiltertype.h>
#include "ov3subscriber.h"
#include "libov3endpoint.h"

#define PLUGIN_NAME "kmsov3subscriber"


GST_DEBUG_CATEGORY_STATIC (ov3_subscriber_debug_category);
#define GST_CAT_DEFAULT ov3_subscriber_debug_category


/* class initialization */

G_DEFINE_TYPE_WITH_CODE (Ov3Subscriber, ov3_subscriber,
    KMS_TYPE_ELEMENT,
    GST_DEBUG_CATEGORY_INIT (GST_CAT_DEFAULT, PLUGIN_NAME,
        0, "debug category for OV3 subscriber element"));



#define OV3_SUBSCRIBER_GET_PRIVATE(obj) (  \
  G_TYPE_INSTANCE_GET_PRIVATE (              \
    (obj),                                   \
    KMS_TYPE_OV3_SUBSCRIBER,                   \
    Ov3SubscriberPrivate                    \
  )                                          \
)


struct _Ov3SubscriberPrivate {
  GstBin *audio_src;
  GstBin *video_src;
  GstElement *audioAgnosticBin;
  GstElement *videoAgnosticBin;

  gulong audio_pad_added_conn;
  gulong video_pad_added_conn;

  gchar *url;
  gchar *secret;
  gchar *key;
  gchar *room;
  gchar *participant;
  gchar *egressId;
  gchar *subscriberId;
  gboolean screenshare;
  gulong keyFrameProbeId;
  gboolean connected;
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
  PROP_OV3_IS_SCREENSHARE,
  PROP_OV3_CONNECTED,
};


/* Signals and args */
enum
{
  /* signals */
  SIGNAL_CONNECT,
  SIGNAL_DISCONNECT,
  SIGNAL_REQUESTKF,

  LAST_SIGNAL
};

static guint obj_signals[LAST_SIGNAL] = { 0 };

static void
ov3_subscriber_connect (Ov3Subscriber *self)
{
  gchar *result;

  result = connectToRoom (self->priv->url, self->priv->key, self->priv->secret, self->priv->room, NULL, NULL);
  // If result starts with ERROR, then no connection could be made
  if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
    GST_ERROR_OBJECT(self, "Could not connect to room %s on service %s for subscribing", self->priv->room, self->priv->url);
    return;
  }

  self->priv->egressId = result;
  result = subscribeParticipant (self->priv->participant, self->priv->screenshare, self->priv->egressId, self->priv->audio_src, self->priv->video_src);
  // If result starts with ERROR, no subscription could be made
  if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
    GST_ERROR_OBJECT(self, "Could not subscribe %s to room %s on service %s", self->priv->participant, self->priv->room, self->priv->url);
    return;
  }

  self->priv->subscriberId = result;

  self->priv->connected = TRUE;
  GST_INFO_OBJECT(self, "Connected and subscribing %s to room %s on service %s for publishing", self->priv->participant, self->priv->room, self->priv->url);
}

static void
ov3_subscriber_request_keyframe (Ov3Subscriber *self)
{
  requestKeyFrame (self->priv->subscriberId);
}

static void
ov3_subscriber_disconnect (Ov3Subscriber *self)
{
  gchar *result;

  if (self->priv->keyFrameProbeId > 0) {
    GstElement *element = gst_bin_get_by_name (GST_BIN(self), "video_source");

    if (element != NULL) {
      GstPad *pad = gst_element_get_static_pad (element, "src");

      gst_pad_remove_probe (pad, self->priv->keyFrameProbeId);

      gst_object_unref (pad);
    }
  }

  if (self->priv->subscriberId != NULL) {
    result = unsubscribeParticipant(self->priv->subscriberId);
    // If results begins with ERROR then no unsunscription could be made
    if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
      GST_ERROR_OBJECT(self, "Could not unsubscribe %s from room %s on service %s", self->priv->participant, self->priv->room, self->priv->url);
      return;
    }

    self->priv->subscriberId = NULL;
    self->priv->connected = FALSE;

    result = disconnectFromRoom(self->priv->egressId);
    if ((result == NULL) ||(strlen(result) == 0) || (strncmp(result, "ERROR", 5) == 0)) {
      GST_INFO_OBJECT(self, "Not disconnecting from room %s on service %s", self->priv->room, self->priv->url);
      return;
    }
    GST_INFO_OBJECT(self, "Disconnected subscribe from room %s on service %s", self->priv->room, self->priv->url);
}

}


static GstPadProbeReturn
forceKeyUnitProbe (GstPad * pad,
                  GstPadProbeInfo * info,
                  gpointer user_data)
{
  Ov3Subscriber *self = (Ov3Subscriber*) user_data;
  GstEvent *event = gst_pad_probe_info_get_event  (info);
  const GstStructure *st;

  if (event == NULL) {
    return GST_PAD_PROBE_OK;
  }
  if (!self->priv->connected) {
    return GST_PAD_PROBE_DROP;
  }
  st = gst_event_get_structure (event);
  if (gst_structure_has_name (st, "GstForceKeyUnit")) {
    if (self->priv->subscriberId != NULL) {

      requestKeyFrame (self->priv->subscriberId);
    }
  }
  return GST_PAD_PROBE_DROP;
}


static void
ov3_subscriber_finalize (GObject *object)
{
  Ov3Subscriber *self = KMS_OV3_SUBSCRIBER(object);

  gst_object_unref (self->priv->audio_src);
  gst_object_unref (self->priv->video_src);

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
  if (self->priv->participant != NULL) {
    g_free(self->priv->participant);
  }
  if (self->priv->egressId != NULL) {
    g_free(self->priv->egressId);
  }
  if (self->priv->subscriberId != NULL) {
    g_free(self->priv->subscriberId);
  }
}


static void 
ov3_subscriber_set_property (GObject * object, guint property_id,
    const GValue * value, GParamSpec * pspec)
{
  Ov3Subscriber *self = KMS_OV3_SUBSCRIBER (object);

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
      g_free (self->priv->participant);
      self->priv->participant = g_value_dup_string (value);
      break;
    }
    case PROP_OV3_IS_SCREENSHARE:{
      self->priv->screenshare = g_value_get_boolean (value);
      break;
    }
    default:
      G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);
      break;
  }
}

static void
ov3_subscriber_get_property (GObject * object, guint property_id,
    GValue * value, GParamSpec * pspec)
{
  Ov3Subscriber *self = KMS_OV3_SUBSCRIBER (object);

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
      g_value_set_string (value, self->priv->participant);
      break;
    }
    case PROP_OV3_IS_SCREENSHARE: {
      g_value_set_boolean (value, self->priv->screenshare);
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
ov3_subscriber_class_init (Ov3SubscriberClass * klass)
{
  GObjectClass *gobject_class;

  gobject_class = G_OBJECT_CLASS (klass);
  gobject_class->set_property = ov3_subscriber_set_property;
  gobject_class->get_property = ov3_subscriber_get_property;
  gobject_class->finalize = ov3_subscriber_finalize;

  klass->ov3_connect = ov3_subscriber_connect ;
  klass->ov3_disconnect = ov3_subscriber_disconnect ;
  klass->ov3_request_keyframe = ov3_subscriber_request_keyframe ;

  gst_element_class_set_static_metadata (GST_ELEMENT_CLASS (klass),
      "OV3Subscriber", "Generic/KmsElement", "Kurento OpenVIdu 3 WebRtc subscriber",
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
      g_param_spec_string ("ov3-participant",
          "OpenVidu3 participant", "Participant Id in the OpenVidu3 room whose tracks will be subscribed by this endpoint",
          "",
		  G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS));      
  g_object_class_install_property (gobject_class, PROP_OV3_IS_SCREENSHARE,
      g_param_spec_boolean ("ov3-screenshare",
          "OpenVidu3 ScreenShare", "This endpoint must subscribe to screen share tracks",
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
      G_STRUCT_OFFSET (Ov3SubscriberClass, ov3_connect), NULL, NULL,
      NULL, G_TYPE_NONE, 0, G_TYPE_NONE);
  obj_signals[SIGNAL_DISCONNECT] =
      g_signal_new ("ov3-disconnect",
      G_TYPE_FROM_CLASS (klass),
      G_SIGNAL_ACTION | G_SIGNAL_RUN_LAST,
      G_STRUCT_OFFSET (Ov3SubscriberClass, ov3_disconnect), NULL, NULL,
      NULL, G_TYPE_NONE, 0, G_TYPE_NONE);
  obj_signals[SIGNAL_REQUESTKF] =
      g_signal_new ("ov3-request-keyframe",
      G_TYPE_FROM_CLASS (klass),
      G_SIGNAL_ACTION | G_SIGNAL_RUN_LAST,
      G_STRUCT_OFFSET (Ov3SubscriberClass, ov3_request_keyframe), NULL, NULL,
      NULL, G_TYPE_NONE, 0, G_TYPE_NONE);

  g_type_class_add_private (klass, sizeof (Ov3SubscriberPrivate));


}

static void
audio_src_pad_added (GstElement * element,
                    GstPad * new_pad,
                    gpointer user_data)
{
  Ov3Subscriber *self = (Ov3Subscriber*) user_data;

  gst_element_link (GST_ELEMENT(self->priv->audio_src), self->priv->audioAgnosticBin);
  g_signal_handler_disconnect(element, self->priv->audio_pad_added_conn);
  self->priv->audio_pad_added_conn = 0;

  gst_element_sync_state_with_parent (GST_ELEMENT(self->priv->audio_src));
}

static void
video_src_pad_added (GstElement * element,
                    GstPad * new_pad,
                    gpointer user_data)
{
  Ov3Subscriber *self = (Ov3Subscriber*) user_data;
  gulong probeId;

  gst_element_link (GST_ELEMENT(self->priv->video_src), self->priv->videoAgnosticBin);
  g_signal_handler_disconnect(element, self->priv->video_pad_added_conn);
  self->priv->video_pad_added_conn = 0;

  probeId = gst_pad_add_probe (new_pad, GST_PAD_PROBE_TYPE_EVENT_UPSTREAM, forceKeyUnitProbe, self, NULL);
  self->priv->keyFrameProbeId = probeId;

  gst_element_sync_state_with_parent (GST_ELEMENT(self->priv->video_src));
}


static void
ov3_subscriber_init (Ov3Subscriber * self)
{
  self->priv = OV3_SUBSCRIBER_GET_PRIVATE (self);

  self->priv->url = g_strdup ("");
  self->priv->secret = g_strdup ("");
  self->priv->key = g_strdup ("");
  self->priv->room = g_strdup ("");
  self->priv->participant = g_strdup ("");
  self->priv->screenshare = FALSE;
  self->priv->egressId = NULL;
  self->priv->subscriberId = NULL;
  self->priv->connected = FALSE;
  self->priv->audio_pad_added_conn = 0;
  self->priv->video_pad_added_conn = 0;

  self->priv->audioAgnosticBin = kms_element_get_audio_agnosticbin (KMS_ELEMENT (self));
  self->priv->videoAgnosticBin = kms_element_get_video_agnosticbin (KMS_ELEMENT (self));

  self->priv->audio_src = GST_BIN(gst_bin_new("ov3_audio_source"));
  self->priv->video_src = GST_BIN(gst_bin_new ("ov3_video_source"));
  gst_bin_add_many (GST_BIN(self), gst_object_ref (GST_ELEMENT(self->priv->audio_src)), gst_object_ref (GST_ELEMENT(self->priv->video_src)), NULL);
  self->priv->audio_pad_added_conn = g_signal_connect (G_OBJECT (self->priv->audio_src),
                                            "pad-added", G_CALLBACK (audio_src_pad_added), self);
  self->priv->video_pad_added_conn = g_signal_connect (G_OBJECT (self->priv->video_src),
                                            "pad-added", G_CALLBACK (video_src_pad_added), self);
}

gboolean
kms_ov3_subscriber_plugin_init (GstPlugin * plugin)
{

  return gst_element_register (plugin, PLUGIN_NAME, GST_RANK_NONE,
      KMS_TYPE_OV3_SUBSCRIBER);
}

